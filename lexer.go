package main

import (
	"strings"
	"unicode/utf8"
)

// 第 1 层：词法。Lexer 逐字节扫描源码，产出 token（类型与结构见 token.go）。
// 标识符和运算符只认 ASCII；字符串、注释里的非 ASCII 字符按 rune 扫描，
// 1 个 rune 算 1 列。

// oneCharPuncts 把单字符标点（运算符、括号、赋值号）映射到 token 类型。
// 用 [utf8.RuneSelf]TokenType 数组：单字符分支已在 default 里确认
// c < utf8.RuneSelf，直接下标寻址；未登记的字符落在零值 ILLEGAL——
// 它正好就是"不是标点"的哨兵，不再需要 map 的 ok。
var oneCharPuncts = [utf8.RuneSelf]TokenType{
	'+': PLUS,
	'-': MINUS,
	'*': STAR,
	'/': SLASH,
	'%': PERCENT,
	'(': LPAREN,
	')': RPAREN,
	'=': ASSIGN,
	'<': LT,
	'>': GT,
	'!': BANG,
}

// 编译期断言 ILLEGAL 必须是 0：上面这张表靠零值表示"不是标点"。
// 以后有人在 ILLEGAL 前面插常量，这里会直接编译失败，而不是让
// 某个未登记字符静默变成新插入的 token。
var _ = [1]struct{}{ILLEGAL: {}} // ILLEGAL 越出 [0,0] 即编译失败 → ILLEGAL == 0

// twoCharPuncts 把双字符标点映射到 token 类型。
// 必须在 oneCharPuncts 之前查：否则 "==" 会被拆成两个 ASSIGN，"!=" 会被拆成 BANG + ASSIGN。
var twoCharPuncts = map[string]TokenType{
	"==": EQ, "!=": NEQ, "<=": LE, ">=": GE, "&&": ANDAND, "||": OROR,
}

// keywords 把关键字映射到 token 类型。
var keywords = map[string]TokenType{
	"let":   LET,
	"true":  BOOLEAN,
	"false": BOOLEAN,
}

// Lexer 逐字节扫描源码。
type Lexer struct {
	source   string // 源码
	position int    // 字节下标，不是 rune 下标
	line     int    // 从 1 开始
	column   int    // 从 1 开始；ASCII 按字节、非 ASCII 按 rune 各算 1 列
}

// NewLexer 返回一个从源码开头开始扫描的 Lexer。
func NewLexer(source string) *Lexer {
	// 剥掉开头的 BOM 再建扫描器——BOM 常见于 Windows 记事本另存的文件，
	// 不剥掉会被当成非法字符。
	source = strings.TrimPrefix(source, "\ufeff")
	return &Lexer{source: source, line: 1, column: 1}
}

// isDigit 判断字符是否为数字。
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isLetter 判断字符是否为字母。
func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// isIdentifierStart 判断字符是否为标识符的起始字符。
func isIdentifierStart(c byte) bool { return isLetter(c) || c == '_' }

// isIdentifierContinue 判断字符是否为标识符的续接字符。
func isIdentifierContinue(c byte) bool { return isLetter(c) || isDigit(c) || c == '_' }

// 数字字面量的扫描辅助：下划线也一并收进 NUMBER
func isDigitOrUnderscore(c byte) bool { return isDigit(c) || c == '_' }

// atEnd 判断是否到达源码末尾。
func (l *Lexer) atEnd() bool { return l.position >= len(l.source) }

// advanceASCII 前进一个字节。
// 调用方必须保证：!atEnd() 且当前字节是 ASCII；否则直接 panic——
// 越界前进会让后续切片越界，劈开 rune 则会让字面值和列号一起静默出错。
// 当前位置可能不是 ASCII 时用 advanceRune。
func (l *Lexer) advanceASCII() {
	if l.atEnd() || l.source[l.position] >= utf8.RuneSelf {
		panic("lexer: advanceASCII 要求当前位置是 ASCII 字节且未到末尾")
	}
	l.position++
	l.column++
}

// advanceRune 解码并前进一个 rune；非法 UTF-8 按 1 字节计，仍只算 1 列。
// 调用方必须保证：!atEnd()；否则 panic——DecodeRuneInString("") 返回 size 0，
// 会让 position 原地不动而 column 一直涨。
// 调用点若已经拿到 size，直接 position += size 更划算。
func (l *Lexer) advanceRune() {
	if l.atEnd() {
		panic("lexer: advanceRune 在源码末尾调用")
	}
	_, size := utf8.DecodeRuneInString(l.source[l.position:])
	l.position += size
	l.column++
}

// skipWhitespaceAndComments 行注释用 //；不消费换行符——换行统一由 NextToken 处理成 NEWLINE。
func (l *Lexer) skipWhitespaceAndComments() {
	for !l.atEnd() {
		switch l.source[l.position] {
		case ' ', '\t':
			l.advanceASCII()
		case '/':
			if l.position+1 < len(l.source) && l.source[l.position+1] == '/' {
				l.advanceASCII()
				l.advanceASCII()

				// 注释正文按 rune 计列：逐字节算会让含中文的注释之后的列号虚高。
				for !l.atEnd() {
					ch := l.source[l.position]
					if ch == '\n' || ch == '\r' {
						break
					}
					if ch < utf8.RuneSelf {
						l.advanceASCII()
						continue
					}
					l.advanceRune()
				}
				continue
			}
			return
		default:
			return
		}
	}
}

// NextToken 先跳过行内空白，再切下一个 token；扫到结尾返回 EOF。
// 换行（\n、\r\n、孤立 \r）在这里切成 NEWLINE，交给 parser 当语句分隔符。
func (l *Lexer) NextToken() Token {
	l.skipWhitespaceAndComments()

	if l.atEnd() {
		return Token{Type: EOF, Line: l.line, Column: l.column}
	}

	c := l.source[l.position]
	line, column := l.line, l.column

	// \n、\r\n、孤立 \r 都只产生一个 NEWLINE，行号 +1，列重置为 1。
	if c == '\n' || c == '\r' {
		l.position++
		if c == '\r' && !l.atEnd() && l.source[l.position] == '\n' {
			l.position++ // 吃掉 \r\n 里的 \n，避免算成两行
		}
		l.line++
		l.column = 1
		return Token{Type: NEWLINE, Literal: "\n", Line: line, Column: column}
	}

	switch {
	case isDigit(c):
		start := l.position

		if c == '0' && l.position+1 < len(l.source) {
			switch l.source[l.position+1] {
			case 'x', 'X', 'b', 'B', 'o', 'O':
				l.advanceASCII() // '0'
				l.advanceASCII() // 前缀字母
				for !l.atEnd() && isIdentifierContinue(l.source[l.position]) {
					l.advanceASCII()
				}
				return Token{Type: NUMBER, Literal: l.source[start:l.position], Line: line, Column: column}
			}
		}

		// 纯十进制：只吞数字和下划线。3.14 切成 NUMBER、ILLEGAL(".")、NUMBER；
		// 前导零（007、010）由 parser 拒绝；下划线是否合法交给 strconv.ParseInt(base=0) 统一裁定（1_000、0xff_ff 合法，1__0、0x_ 不合法）。
		for !l.atEnd() && isDigitOrUnderscore(l.source[l.position]) {
			l.advanceASCII()
		}
		return Token{Type: NUMBER, Literal: l.source[start:l.position], Line: line, Column: column}

	case isIdentifierStart(c):
		start := l.position
		for !l.atEnd() && isIdentifierContinue(l.source[l.position]) {
			l.advanceASCII()
		}
		word := l.source[start:l.position]
		tt := IDENTIFIER
		if kw, ok := keywords[word]; ok {
			tt = kw
		}
		return Token{Type: tt, Literal: word, Line: line, Column: column}

	case c == '"':
		start := l.position
		l.advanceASCII()
		for !l.atEnd() {
			ch := l.source[l.position]
			if ch == '"' {
				l.advanceASCII()
				return Token{Type: STRING, Literal: l.source[start:l.position], Line: line, Column: column}
			}
			// 字符串不能跨行、末尾不能悬空：这几种都算"未闭合"，单独成一个 token
			// 类型，让 parser 能报"字符串未闭合"，而不是笼统的"非法字符"。
			if ch == '\n' || ch == '\r' {
				return Token{Type: UNTERMINATED_STRING, Literal: l.source[start:l.position], Line: line, Column: column}
			}
			if ch == '\\' {
				l.advanceASCII()
				if l.atEnd() {
					return Token{Type: UNTERMINATED_STRING, Literal: l.source[start:l.position], Line: line, Column: column}
				}
				// 转义符后面可能跟一个多字节 rune（如 "\中"），要按 rune 消费。
				r, size := utf8.DecodeRuneInString(l.source[l.position:])
				// 但 \ 紧跟真实换行时不能吞：吞掉后外层就走不到"行号 +1"，
				// 后面整个文件的行号会少 1。按"未闭合"上报，换行留给 NextToken。
				if r == '\n' || r == '\r' {
					return Token{Type: UNTERMINATED_STRING, Literal: l.source[start:l.position], Line: line, Column: column}
				}
				l.position += size // size 已在手，不调 advanceRune，免得再解一次
				l.column++
				continue
			}
			if ch < utf8.RuneSelf {
				l.advanceASCII()
				continue
			}
			l.advanceRune()
		}
		return Token{Type: UNTERMINATED_STRING, Literal: l.source[start:l.position], Line: line, Column: column}

	default:
		// 先查双字符标点。切片边界由 position+1 < len 保证；
		// 匹配成功意味着两字节都在表里（全是 ASCII），advanceASCII 不会 panic。
		if l.position+1 < len(l.source) {
			if tt, ok := twoCharPuncts[l.source[l.position:l.position+2]]; ok {
				lit := l.source[l.position : l.position+2]
				l.advanceASCII()
				l.advanceASCII()
				return Token{Type: tt, Literal: lit, Line: line, Column: column}
			}
		}

		var lit string
		tt := ILLEGAL
		if c < utf8.RuneSelf {
			tt = oneCharPuncts[c]
			lit = string(c)
			l.advanceASCII()
		} else {
			start := l.position
			l.advanceRune()
			lit = l.source[start:l.position]
		}
		return Token{Type: tt, Literal: lit, Line: line, Column: column}
	}
}
