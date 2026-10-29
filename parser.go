package main

// 第 2 层：语法。优先级爬升（precedence climbing）的递归下降解析器：
// 一个循环 + 一张绑定力表（operator.go 的 binOps），不再为每级优先级写一个函数。
//
// 左结合靠"右递归传 prec+1"：同优先级的运算符抢不走右边的操作数，只能回到
// 本层循环里继续往左叠（"1-2-3" → ((1-2)-3)）；更高优先级的运算符 prec 更大，
// 才够格落进右递归成为内层节点。平坦长链（"1+1+…"）的栈深因此不随项数增长：
// 内层看到同级运算符就直接返回了。
//
// 两条无界递归路径由 maxDepth 挡住：括号（parsePrimary → parseExpr）和一元
// 负号（parsePrimary → parsePrimary）。一元负号只吃下一个 primary，所以绑定力
// 高于所有二元运算符："-2*3" 解析成 (-2)*3。

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Parser 读 token 流，产出 AST。
type Parser struct {
	lex   *Lexer
	cur   Token
	depth int // 当前嵌套层数，由 enter/leave 维护
}

// NewParser 读入第一个 token，返回解析器。
func NewParser(lex *Lexer) *Parser {
	p := &Parser{lex: lex}
	p.advance()
	return p
}

func (p *Parser) advance() { p.cur = p.lex.NextToken() }

// ---------- 深度闸 ----------

// maxDepth 是解析嵌套层数的上限。括号与一元负号都走递归，输入 "((((…" /
// "----…" 足够深会压爆 Go 栈——栈溢出是不可 recover 的 fatal error，必须在
// 解析阶段挡住。1000 层 × 每层约 3 帧，远低于 Go 的栈上限。
const maxDepth = 1000

// enter 在进入一层新的嵌套前调用，位置取引起嵌套的 token（"(" 或 "-"）。
// 先判上限再自增：失败时计数不变，否则失败一次就会永久抬高一层。
func (p *Parser) enter(tok Token) error {
	if p.depth >= maxDepth {
		return errAt(tok, "表达式嵌套过深（上限 %d 层）", maxDepth)
	}
	p.depth++
	return nil
}

// leave 与 enter 成对，只能用 defer 调用，出错返回时计数也会还原。
func (p *Parser) leave() { p.depth-- }

// errAt 统一 "第 N 行，第 M 列: <msg>" 的错误格式。
// 当前 token 是 ILLEGAL 时在 msg 后面补上非法字符本身：调用方的上下文
// （"期望表达式"、"语句后缺少换行"）和"哪个字符非法"都保留。
func errAt(tok Token, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if tok.Type == ILLEGAL {
		msg = fmt.Sprintf("%s（非法字符 %q）", msg, tok.Literal)
	}
	return errAtPos(Position{Line: tok.Line, Column: tok.Column}, "%s", msg)
}

// ParseProgram 把整份源码解析成语句列表；语句之间用换行分隔。
func (p *Parser) ParseProgram() ([]Node, error) {
	var stmts []Node
	for p.cur.Type != EOF {
		if p.cur.Type == NEWLINE {
			p.advance()
			continue
		}
		n, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, n)
		if p.cur.Type != NEWLINE && p.cur.Type != EOF {
			return nil, errAt(p.cur, "语句后缺少换行，得到 %s", p.cur.Type)
		}
	}
	return stmts, nil
}

// parseStatement 分派一条语句：let 走 parseLet，其余走表达式。
// 后续加 x = 1（Assign）时也只需要在这里加一个分支。
func (p *Parser) parseStatement() (Node, error) {
	if p.cur.Type == LET {
		return p.parseLet()
	}
	return p.parseExpr(minPrecLevel)
}

// parseLet 解析 let <Name> = <Value>。Value 走 parseExpr(minPrecLevel)，
// 遇到 NEWLINE 自然停，所以 let x = 1 + 2 合法；嵌套 let 不在表达式里支持
// （parsePrimary 的 switch 里没有 LET，会报"期望表达式"）。
func (p *Parser) parseLet() (Node, error) {
	at := Position{Line: p.cur.Line, Column: p.cur.Column}
	p.advance()

	if p.cur.Type != IDENTIFIER {
		return nil, errAt(p.cur, "期望标识符，得到 %s", p.cur.Type)
	}
	name := p.cur.Literal
	p.advance()

	if p.cur.Type != ASSIGN {
		return nil, errAt(p.cur, "期望赋值号，得到 %s", p.cur.Type)
	}
	p.advance()

	value, err := p.parseExpr(minPrecLevel)
	if err != nil {
		return nil, err
	}
	return Let{Name: name, Value: value, At: at}, nil
}

// parseExpr 解析所有绑定力 >= minPrec 的二元运算，左结合。
// 遇到"不是二元运算符的 token"或"比 minPrec 更松的运算符"就返回，把决定权
// 交还外层——这就是"爬升"：内层只吃比门槛紧的运算符，同级的由本层循环往左叠。
func (p *Parser) parseExpr(minPrec int) (Node, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		// p.cur 只来自 lexer，Type 一定落在 [0, tokenTypeCount)，直接索引不会越界。
		info := binOps[p.cur.Type]
		if !info.isBinary() {
			// 当前 token 不是二元运算符（换行、右括号、EOF…）：表达式在本层完整了。
			return left, nil
		}
		if info.prec < minPrec {
			// 是运算符，但比本层门槛松（"+" 遇到正在算的 "*"）：留给外层去叠。
			return left, nil
		}
		opPos := Position{Line: p.cur.Line, Column: p.cur.Column}
		p.advance()
		// 右递归传 prec+1：同优先级的运算符进不了右递归，只能回到本层。
		right, err := p.parseExpr(info.prec + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: info.op, Left: left, Right: right, At: opPos}
	}
}

// parsePrimary 解析字面量（数字 / 字符串）、标识符、括号和一元负号。括号里的
// 表达式必须重新从 minPrecLevel 爬，否则 "(1+2)*3" 里外层的 * 会伸进括号内部。
//
// 一元负号只吃下一个 primary（递归回 parsePrimary），所以它的绑定力高于所有
// 二元运算符："-2*3" 是 (-2)*3，"1--2" 是 1-(-2)，"--3" 是 -(-3)。
func (p *Parser) parsePrimary() (Node, error) {
	switch p.cur.Type {
	case MINUS:
		minusTok := p.cur
		if err := p.enter(minusTok); err != nil {
			return nil, err
		}
		defer p.leave()
		p.advance()
		x, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return Unary{Op: OpMinus, X: x, At: Position{Line: minusTok.Line, Column: minusTok.Column}}, nil

	case PLUS:
		return nil, errAt(p.cur, "暂不支持一元正号")

	case NUMBER:
		tok := p.cur
		p.advance()

		if len(tok.Literal) > 1 && tok.Literal[0] == '0' {
			switch tok.Literal[1] {
			case 'x', 'X', 'b', 'B', 'o', 'O':
				// 进制前缀，交给 ParseInt
			case '_':
				return nil, errAt(tok, "数字字面量不合法：%s（0 后不能直接跟下划线）", tok.Literal)
			default:
				return nil, errAt(tok, "前导零不被允许：%s", tok.Literal)
			}
		}

		v, err := strconv.ParseInt(tok.Literal, 0, 64)
		if err != nil {
			return nil, errAt(tok, "数字字面量不合法：%s", tok.Literal)
		}
		return NumberLiteral{Value: v, At: Position{Line: tok.Line, Column: tok.Column}}, nil

	case STRING:
		tok := p.cur
		p.advance()
		s, err := unquoteString(tok)
		if err != nil {
			return nil, err
		}
		return StringLiteral{Value: s, At: Position{Line: tok.Line, Column: tok.Column}}, nil

	case LPAREN:
		open := p.cur
		if err := p.enter(open); err != nil {
			return nil, err
		}
		defer p.leave()
		p.advance()
		n, err := p.parseExpr(minPrecLevel)
		if err != nil {
			return nil, err
		}
		if p.cur.Type != RPAREN {
			return nil, errAt(p.cur, "期望 RPAREN，得到 %s", p.cur.Type)
		}
		p.advance()
		return n, nil

	case IDENTIFIER:
		tok := p.cur
		p.advance()
		return Identifier{Name: tok.Literal, At: Position{Line: tok.Line, Column: tok.Column}}, nil
	}
	return nil, errAt(p.cur, "期望表达式，得到 %s", p.cur.Type)
}

// unquoteString 去掉两侧引号并反转义。lexer 的 STRING.Literal 保留原文
// （含两侧引号、反斜杠原样），转义在这里做：\n \t \r \\ \" 五种；其余 \x 报错。
func unquoteString(tok Token) (string, error) {
	lit := tok.Literal
	if len(lit) < 2 || lit[0] != '"' || lit[len(lit)-1] != '"' {
		return "", errAt(tok, "字符串字面量不合法：%s", lit)
	}

	inner := lit[1 : len(lit)-1]
	var b strings.Builder
	b.Grow(len(inner))

	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(inner) {
			// 理论上不可达：lexer 保证 STRING 里 \ 后必跟一个非换行字符，
			// 所以 inner 不可能以孤立反斜杠结尾。留着兜住未来 token 化的改动。
			return "", errAt(tok, "转义序列不完整")
		}
		switch inner[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		default:
			// 按 rune 解码再 %q：inner[i] 只是字节，多字节字符直接 %c 会输出乱码。
			r, _ := utf8.DecodeRuneInString(inner[i:])
			return "", errAt(tok, "未知转义序列：%q", r)
		}
	}
	return b.String(), nil
}
