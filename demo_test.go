package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// ---------- 第 1 层：词法 ----------

// lexAll 把源码扫成 token 序列（含 EOF）；两个快照助手共用它。
func lexAll(src string) []Token {
	l := NewLexer(src)
	var out []Token
	for {
		tok := l.NextToken()
		out = append(out, tok)
		if tok.Type == EOF {
			return out
		}
	}
}

// tokensOf 把整段源码扫成 "行:类型:字面值" 快照，失败时一眼能看出错在哪。
func tokensOf(src string) string {
	tokens := lexAll(src)
	parts := make([]string, len(tokens))
	for i, tok := range tokens {
		parts[i] = fmt.Sprintf("%d:%s:%q", tok.Line, tok.Type, tok.Literal)
	}
	return strings.Join(parts, " ")
}

func TestNextToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "空行 + 单条语句 + 尾换行",
			input: "\n1+2+3\n",
			want:  `1:NEWLINE:"\n" 2:NUMBER:"1" 2:PLUS:"+" 2:NUMBER:"2" 2:PLUS:"+" 2:NUMBER:"3" 2:NEWLINE:"\n" 3:EOF:""`,
		},
		{
			name:  "运算符与数字的边界",
			input: "1+2*3\n",
			want:  `1:NUMBER:"1" 1:PLUS:"+" 1:NUMBER:"2" 1:STAR:"*" 1:NUMBER:"3" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// 空格、制表符被跳过；CRLF 只算一个 NEWLINE。
			name:  "行内空白与 CRLF",
			input: "  1 +\t2  *  3 \r\n",
			want:  `1:NUMBER:"1" 1:PLUS:"+" 1:NUMBER:"2" 1:STAR:"*" 1:NUMBER:"3" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// 孤立 \r 也算换行：和 CRLF、LF 得到相同的 token 流。
			name:  "孤立回车也是换行",
			input: "1\r2\r",
			want:  `1:NUMBER:"1" 1:NEWLINE:"\n" 2:NUMBER:"2" 2:NEWLINE:"\n" 3:EOF:""`,
		},
		{
			// 标识符走 isIdentifierStart；@ 不在 punctuators 里，是 ILLEGAL。
			name:  "标识符与非法字符",
			input: "a@1\n",
			want:  `1:IDENTIFIER:"a" 1:ILLEGAL:"@" 1:NUMBER:"1" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// 数字在连续数字处结束，紧邻字母单独成为 IDENTIFIER。
			name:  "数字与相邻标识符",
			input: "1a\n",
			want:  `1:NUMBER:"1" 1:IDENTIFIER:"a" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// punctuators 表里每个条目都在这里出现一次：名字表写错顺序、或漏配一个符号，这条会先红。
			name:  "全部单字符符号",
			input: "()+-*/%=\n",
			want:  `1:LPAREN:"(" 1:RPAREN:")" 1:PLUS:"+" 1:MINUS:"-" 1:STAR:"*" 1:SLASH:"/" 1:PERCENT:"%" 1:ASSIGN:"=" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// keywords 表里每个条目都在这里出现一次。
			name:  "let 关键字",
			input: "let\n",
			want:  `1:LET:"let" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// BOM 在 NewLexer 里被剥掉：不会变成 ILLEGAL，也不占列号。
			name:  "开头的 BOM 不产生 token",
			input: "\ufeff1+2\n",
			want:  `1:NUMBER:"1" 1:PLUS:"+" 1:NUMBER:"2" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// 进制前缀和字母数字一起收进 NUMBER；具体合法与否交给 parser。
			name:  "进制字面量",
			input: "0x1F 0b101 0o17\n",
			want:  `1:NUMBER:"0x1F" 1:NUMBER:"0b101" 1:NUMBER:"0o17" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// 行注释被跳过：// 到行尾的内容不产生 token，换行仍切成 NEWLINE。
			name:  "行注释被跳过",
			input: "1 // 注释\n2\n",
			want:  `1:NUMBER:"1" 1:NEWLINE:"\n" 2:NUMBER:"2" 2:NEWLINE:"\n" 3:EOF:""`,
		},
		{
			// 字符串里的 // 不是注释：整个字符串是一个 STRING token。
			name:  "字符串里的 // 不是注释",
			input: `"a//b"` + "\n",
			want:  `1:STRING:"\"a//b\"" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// STRING token 保留两侧引号与反斜杠原文，转义留给 parser 做。
			name:  "字符串 token 保留原文",
			input: `"a\nb"` + " " + `"中文"` + "\n",
			want:  `1:STRING:"\"a\\nb\"" 1:STRING:"\"中文\"" 1:NEWLINE:"\n" 2:EOF:""`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokensOf(tt.input); got != tt.want {
				t.Errorf("输入 %q\n  got: %s\n want: %s", tt.input, got, tt.want)
			}
		})
	}
}

// dumpColumn 把 token 序列压成 "类型 行:列" 快照；列号单独测，不混进上面的行快照。
func dumpColumn(tokens []Token) string {
	parts := make([]string, len(tokens))
	for i, tk := range tokens {
		parts[i] = fmt.Sprintf("%s %d:%d", tk.Type, tk.Line, tk.Column)
	}
	return strings.Join(parts, " ")
}

// TestTokenColumns 守住列号约定：token 起点、行内空白推进列、\t 按 1 列、CRLF 只加一行。
func TestTokenColumns(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"制表符按 1 列", "\t1\n", "NUMBER 1:2 NEWLINE 1:3 EOF 2:1"},
		{"行内空白推进列号", "  1+\n", "NUMBER 1:3 PLUS 1:4 NEWLINE 1:5 EOF 2:1"},
		{"CRLF 只加一行、列重置", "1\r\n2\n", "NUMBER 1:1 NEWLINE 1:2 NUMBER 2:1 NEWLINE 2:2 EOF 3:1"},
		{"空格与 TAB 混排的列号", "  1 +\t2  *  3 \r\n",
			"NUMBER 1:3 PLUS 1:5 NUMBER 1:7 STAR 1:10 NUMBER 1:13 NEWLINE 1:15 EOF 2:1"},
		{"行注释按 rune 计列", "1 // 中\n2\n", "NUMBER 1:1 NEWLINE 1:7 NUMBER 2:1 NEWLINE 2:2 EOF 3:1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dumpColumn(lexAll(tt.input)); got != tt.want {
				t.Errorf("输入 %q\n  got: %s\n want: %s", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- 第 2 层：语法 ----------

// parseProgram 解析整段源码；失败直接 Fatal。
func parseProgram(t *testing.T, src string) []Node {
	t.Helper()
	p := NewParser(NewLexer(src))
	stmts, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("解析 %q 失败：%v", src, err)
	}
	return stmts
}

// dumpStmts 把语句列表渲染成 "a | b | c"，用 S-表达式展示 AST 形状。
func dumpStmts(stmts []Node) string {
	parts := make([]string, len(stmts))
	for i, s := range stmts {
		parts[i] = SExpr(s)
	}
	return strings.Join(parts, " | ")
}

func TestParseProgram(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"左结合", "1+2+3\n", "(PLUS (PLUS 1 2) 3)"},
		{"乘法优先级更高", "1+2*3\n", "(PLUS 1 (STAR 2 3))"},
		{"混合优先级且左结合", "1+2*3-4/2\n", "(MINUS (PLUS 1 (STAR 2 3)) (SLASH 4 2))"},
		{"长链左结合", "1-2-3-4\n", "(MINUS (MINUS (MINUS 1 2) 3) 4)"},
		{"括号改变结合", "(1+2)*3\n", "(STAR (PLUS 1 2) 3)"},
		{"一元负号", "-1*2\n", "(STAR (MINUS 1) 2)"},
		{"乘号右边的一元负号", "2*-3\n", "(STAR 2 (MINUS 3))"},
		{"负号作用于括号", "-(1+2)\n", "(MINUS (PLUS 1 2))"},
		{"连写负号", "--3\n", "(MINUS (MINUS 3))"},
		{"减号后的一元负号", "1--2\n", "(MINUS 1 (MINUS 2))"},
		{"取模与乘除同级、高于加减", "1+2%3\n", "(PLUS 1 (PERCENT 2 3))"},
		{"取模与乘法同级左结合", "3*17%5\n", "(PERCENT (STAR 3 17) 5)"},
		{"连续取模左结合", "17%5%3\n", "(PERCENT (PERCENT 17 5) 3)"},
		{"取模右侧的一元负号", "17%-5\n", "(PERCENT 17 (MINUS 5))"},
		{"空行被跳过、多条语句", "\n1+2\n4*5\n", "(PLUS 1 2) | (STAR 4 5)"},
		{"只有空行", "  \n\n", ""},
		// ---- let ----
		{"let 声明", "let x = 1\n", "(let x 1)"},
		{"let 右侧是表达式", "let x = 1 + 2 * 3\n", "(let x (PLUS 1 (STAR 2 3)))"},
		{"变量引用", "let x = 1\nx + 2\n", "(let x 1) | (PLUS x 2)"},
		{"let 与后续语句", "let x = 1\nlet y = x * 2\ny\n",
			"(let x 1) | (let y (STAR x 2)) | y"},
		// ---- 进制字面量 ----
		{"十六进制", "0xff\n", "255"},
		{"二进制", "0b1010\n", "10"},
		{"八进制", "0o17\n", "15"},
		// ---- 字符串 ----
		{"空字符串", `""` + "\n", `""`},
		{"普通字符串", `"abc"` + "\n", `"abc"`},
		{"转义字符串", `"a\nb"` + "\n", `"a\nb"`},
		{"字符串拼接", `"a" + "b"` + "\n", `(PLUS "a" "b")`},
		{"let 字符串", `let s = "hello"` + "\n", `(let s "hello")`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dumpStmts(parseProgram(t, tt.src)); got != tt.want {
				t.Errorf("输入 %q 的 AST = %q，期望 %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		src, want string
	}{
		{"1+2 3\n", "语句后缺少换行，得到 NUMBER"},
		{"(1+2\n", "期望 RPAREN，得到 NEWLINE"},
		{")\n", "期望表达式，得到 RPAREN"},
		{"1+\n", "期望表达式，得到 NEWLINE"},
		{"-\n", "期望表达式，得到 NEWLINE"},
		{"()\n", "期望表达式，得到 RPAREN"},
		{"(+)\n", "暂不支持一元正号"},
		{"99999999999999999999\n", "数字字面量不合法"},
		{"0x\n", "数字字面量不合法"},
		{"0b\n", "数字字面量不合法"},
		{"0_1\n", "数字字面量不合法"},
		{"1__0\n", "数字字面量不合法"},
		{"1_\n", "数字字面量不合法"},
		{"1a\n", "语句后缺少换行"},
		// ---- let ----
		{"let 5 = 1\n", "期望标识符，得到 NUMBER"},
		{"let x 1\n", "期望赋值号，得到 NUMBER"},
		{"let x =\n", "期望表达式，得到 NEWLINE"},
		{"let let = 1\n", "期望标识符，得到 LET"},
		// ---- 字符串 ----
		{`"a\q"` + "\n", "未知转义序列"},
		{`"\中"` + "\n", "未知转义序列：'中'"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			p := NewParser(NewLexer(tt.src))
			_, err := p.ParseProgram()
			if err == nil {
				t.Fatalf("输入 %q 期望报错，却解析成功", tt.src)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("输入 %q 的错误 = %q，期望包含 %q", tt.src, err, tt.want)
			}
		})
	}
}

// TestNestingDepthLimit 钉住深度闸：恰好 maxDepth 层通过，多一层带位置报错，
// 而不是栈溢出。括号和一元负号是解析器仅有的两条无界递归路径，各测一遍。
func TestNestingDepthLimit(t *testing.T) {
	parens := func(n int) string { return strings.Repeat("(", n) + "1" + strings.Repeat(")", n) + "\n" }
	if _, err := NewParser(NewLexer(parens(maxDepth))).ParseProgram(); err != nil {
		t.Fatalf("%d 层括号应当通过，却报错：%v", maxDepth, err)
	}
	_, err := NewParser(NewLexer(parens(maxDepth + 1))).ParseProgram()
	if err == nil || !strings.Contains(err.Error(), "嵌套过深") {
		t.Fatalf("%d 层括号的错误 = %v，期望包含\"嵌套过深\"", maxDepth+1, err)
	}
	if !strings.Contains(err.Error(), "第 1 行") {
		t.Errorf("深度错误没带位置：%v", err)
	}

	minus := func(n int) string { return strings.Repeat("-", n) + "1\n" }
	if _, err := NewParser(NewLexer(minus(maxDepth))).ParseProgram(); err != nil {
		t.Fatalf("%d 层一元负号应当通过，却报错：%v", maxDepth, err)
	}
	if _, err := NewParser(NewLexer(minus(maxDepth + 1))).ParseProgram(); err == nil ||
		!strings.Contains(err.Error(), "嵌套过深") {
		t.Fatalf("%d 层一元负号的错误 = %v，期望包含\"嵌套过深\"", maxDepth+1, err)
	}
}

// ---------- 运算符表 ----------

func TestBinOpTable(t *testing.T) {
	seen := make(map[Op]TokenType, len(binOps))
	for i, info := range binOps {
		if !info.isBinary() {
			continue
		}
		tok := TokenType(i)
		if info.prec < 1 {
			t.Errorf("%s 的绑定力 = %d：必须 >= 1", tok, info.prec)
		}
		if info.op >= opCount {
			t.Errorf("%s 的 Op = %d，超出 opCount(%d)", tok, info.op, opCount)
		}
		if prev, ok := seen[info.op]; ok {
			t.Errorf("Op %s 同时挂在 %s 与 %s 上", info.op, prev, tok)
		}
		seen[info.op] = tok
	}
	for op := range opCount {
		if _, ok := seen[op]; !ok {
			t.Errorf("Op %s 不在 binOps 里：解析器永远产不出它", op)
		}
	}
}

func TestBinOpsAreFullyWired(t *testing.T) {
	sym := make(map[TokenType]byte, len(punctuators))
	for b, tt := range punctuators {
		sym[tt] = b
	}
	for i, info := range binOps {
		if !info.isBinary() {
			continue
		}
		tok := TokenType(i)
		t.Run(info.op.String(), func(t *testing.T) {
			b, ok := sym[tok]
			if !ok {
				t.Fatalf("词法：punctuators 表里没有 %s 的符号", tok)
			}
			src := fmt.Sprintf("1%c2\n", b)
			stmts := parseProgram(t, src)
			if len(stmts) != 1 {
				t.Fatalf("%q 解析出 %d 条语句，期望 1 条", src, len(stmts))
			}
			bin, ok := stmts[0].(Binary)
			if !ok {
				t.Fatalf("%q 的 AST 顶层是 %T，期望 Binary", src, stmts[0])
			}
			if bin.Op != info.op {
				t.Fatalf("%q 解析出的 Op = %s，期望 %s", src, bin.Op, info.op)
			}
			if got, want := SExpr(stmts[0]), "("+info.op.String()+" 1 2)"; got != want {
				t.Fatalf("%q 的 S-表达式 = %q，期望 %q", src, got, want)
			}
			if _, err := stmts[0].Eval(NewEnv()); err != nil {
				t.Fatalf("求值 %q 失败：%v（eval.go 的 case 没接通）", src, err)
			}
		})
	}
}

// ---------- 静态类型检查 ----------

// TestTypeCheck 钉住静态检查：类型错误必须在求值前被拦住。
func TestTypeCheck(t *testing.T) {
	tests := []struct {
		src, want string
	}{
		{"a\n", "未定义变量"},
		{"1 + a\n", "未定义变量"},
		{"let x = 1\nx + \"a\"\n", "两侧类型不同"},
		{`"a" - 1` + "\n", "需要整数"},
		{`"a" * 2` + "\n", "需要整数"},
		{`"a" % 2` + "\n", "需要整数"},
		{`-"a"` + "\n", "一元负号需要整数"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			stmts := parseProgram(t, tt.src)
			err := CheckProgram(stmts)
			if err == nil {
				t.Fatalf("%q 期望类型错误，却通过了检查", tt.src)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%q 的错误 = %q，期望包含 %q", tt.src, err, tt.want)
			}
		})
	}
}

// TestTypeCheckPass 钉住合法程序的类型检查：字符串 +、let 绑定、共享 TypeEnv。
func TestTypeCheckPass(t *testing.T) {
	for _, src := range []string{
		"1+2*3\n",
		`"a" + "b"` + "\n",
		`let s = "hi"` + "\ns\n",
		`let a = "x"` + "\nlet b = a + \"y\"" + "\nb\n",
		"let x = 5\nx + 2\n",
	} {
		t.Run(src, func(t *testing.T) {
			if err := CheckProgram(parseProgram(t, src)); err != nil {
				t.Fatalf("%q 期望通过类型检查，却报错：%v", src, err)
			}
		})
	}
}

// ---------- 第 3 层：求值 ----------

// evalLast 解析并逐条求值，返回最后一条语句的值。env 建在循环外，
// 多条语句共享绑定（let 与后续引用）。
func evalLast(t *testing.T, src string) (Value, error) {
	t.Helper()
	stmts := parseProgram(t, src)
	env := NewEnv()
	var last Value
	for _, s := range stmts {
		v, err := s.Eval(env)
		if err != nil {
			return Value{}, err
		}
		last = v
	}
	return last, nil
}

func TestEval(t *testing.T) {
	tests := []struct {
		src  string
		want Value
	}{
		{"1+2*3\n", IntValue(7)},
		{"(1+2)*3\n", IntValue(9)},
		{"10-2-3\n", IntValue(5)},
		{"1+2*3-4/2\n", IntValue(5)},
		{"-1*2\n", IntValue(-2)},
		{"2*-3\n", IntValue(-6)},

		{"-(1+2)\n", IntValue(-3)},
		{"1--2\n", IntValue(3)},
		{"--3\n", IntValue(3)},
		{"8/2\n", IntValue(4)},
		{"((1+2)*3-4)/2\n", IntValue(2)},
		{"2*0\n", IntValue(0)},
		{"0*5\n", IntValue(0)},
		{"1+2\n4*5\n", IntValue(20)},

		{"7%3\n", IntValue(1)},
		{"10%10\n", IntValue(0)},
		{"10+17%5\n", IntValue(12)},
		{"3*17%5\n", IntValue(1)},
		{"-17%5\n", IntValue(-2)},
		{"17%-5\n", IntValue(2)},

		// ---- 进制字面量 ----
		{"0xff\n", IntValue(255)},
		{"0XFF\n", IntValue(255)},
		{"0b1010\n", IntValue(10)},
		{"0o17\n", IntValue(15)},
		{"0xff + 1\n", IntValue(256)},
		{"0x10 * 0b10\n", IntValue(32)},

		// ---- let ----
		{"let x = 5\nx + 2\n", IntValue(7)},
		{"let x = 2\nlet y = x * 3\ny\n", IntValue(6)},
		{"let x = 5\n", IntValue(5)},               // let 返回值 = 赋的值
		{"let x = 1\nlet x = 2\nx\n", IntValue(2)}, // 允许覆盖
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			got, err := evalLast(t, tt.src)
			if err != nil {
				t.Fatalf("求值 %q 失败：%v", tt.src, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("%q = %s，期望 %s", tt.src, got, tt.want)
			}
		})
	}
}

// TestEvalTypeGuards 钉住求值层的对称兜底：绕过类型检查直接求值时，
// 二元和一元运算遇到类型不符的操作数都必须报错，而不是静默算出个 0。
func TestEvalTypeGuards(t *testing.T) {
	tests := []struct{ src, want string }{
		{`"a" - 1` + "\n", "需要整数"},  // Binary.Eval 的守卫
		{`-"a"` + "\n", "一元负号需要整数"}, // Unary.Eval 的守卫
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			_, err := evalLast(t, tt.src)
			if err == nil {
				t.Fatalf("%q 期望报错，却求值成功", tt.src)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%q 的错误 = %q，期望包含 %q", tt.src, err, tt.want)
			}
		})
	}
}

// TestValueEqual 钉住 Equal 的语义：kind 与内容都相同才相等，int 与 string 不互通。
func TestValueEqual(t *testing.T) {
	if !IntValue(7).Equal(IntValue(7)) {
		t.Error("相同 int 应相等")
	}
	if IntValue(7).Equal(IntValue(8)) {
		t.Error("不同 int 不应相等")
	}
	if IntValue(7).Equal(StringValue("7")) {
		t.Error("int 与 string 不应相等")
	}
	if !StringValue("hi").Equal(StringValue("hi")) {
		t.Error("相同 string 应相等")
	}
	if StringValue("hi").Equal(StringValue("hey")) {
		t.Error("不同 string 不应相等")
	}
}

// TestEvalString 覆盖字符串本身、转义与拼接。
func TestEvalString(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{`""` + "\n", ""},
		{`"abc"` + "\n", "abc"},
		{`"a\nb"` + "\n", "a\nb"},
		{`"a\tb"` + "\n", "a\tb"},
		{`"a\"b"` + "\n", `a"b`},
		{`"C:\\temp"` + "\n", `C:\temp`},
		{`"你好，世界"` + "\n", "你好，世界"},
		{`let s = "hello"` + "\ns\n", "hello"},
		{`"a" + "b"` + "\n", "ab"},
		{`let a = "x"` + "\nlet b = \"y\"" + "\na + b\n", "xy"},
		{`let g = "hello"` + "\nlet h = g + \"!\"\nh\n", "hello!"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			got, err := evalLast(t, tt.src)
			if err != nil {
				t.Fatalf("求值 %q 失败：%v", tt.src, err)
			}
			if !got.IsString() {
				t.Fatalf("%q = %s，期望 string", tt.src, got)
			}
			if got.AsString() != tt.want {
				t.Errorf("%q = %q，期望 %q", tt.src, got.AsString(), tt.want)
			}
		})
	}
}

func TestEvalDivisionByZero(t *testing.T) {
	for _, src := range []string{"1/0\n", "0/0\n"} {
		t.Run(src, func(t *testing.T) {
			_, err := evalLast(t, src)
			if err == nil {
				t.Fatalf("%q 期望除零报错，却成功了", src)
			}
			if !strings.Contains(err.Error(), "除数为零") {
				t.Errorf("%q 的错误 = %q，期望包含 除数为零", src, err)
			}
		})
	}
}

func TestModuloByZero(t *testing.T) {
	for _, src := range []string{"1%0\n", "0%0\n"} {
		t.Run(src, func(t *testing.T) {
			_, err := evalLast(t, src)
			if err == nil {
				t.Fatalf("%q 期望取模除零报错，却成功了", src)
			}
			if !strings.Contains(err.Error(), "余数为零") {
				t.Errorf("%q 的错误 = %q，期望包含 余数为零", src, err)
			}
		})
	}

	_, err := evalLast(t, "1%0\n")
	if err == nil {
		t.Fatal("期望取模除零报错，却成功了")
	}
	if !strings.Contains(err.Error(), "第 1 行，第 2 列") {
		t.Errorf("错误 = %q，期望带上位置 \"第 1 行，第 2 列\"", err)
	}
}

// TestLongChainParseShallowEvalDeep 钉住解析器的迭代性质，并顺带求值 10 万项
// 加法链：解析是循环、栈深不随项数增长；求值仍是递归（Go 栈会自动增长）。
func TestLongChainParseShallowEvalDeep(t *testing.T) {
	got, err := evalLast(t, strings.Repeat("1+", 100000)+"1\n")
	if err != nil {
		t.Fatalf("10 万项加法链求值失败：%v", err)
	}
	if got != IntValue(100001) {
		t.Errorf("10 万项加法链 = %s，期望 100001", got)
	}
}

// ---------- 读法不写回节点 ----------

// readSignature 把三段读法各跑一遍，压成一行快照：S-表达式、类型检查、求值结果。
// 每次都用新的 TypeEnv / Env，这样"环境复用"不会被误当成"节点被写回"。
func readSignature(stmts []Node) (string, error) {
	if err := CheckProgram(stmts); err != nil {
		return "", err
	}
	env := NewEnv()
	vals := make([]string, 0, len(stmts))
	for _, s := range stmts {
		v, err := s.Eval(env)
		if err != nil {
			return "", err
		}
		vals = append(vals, v.String())
	}
	return dumpStmts(stmts) + " ⇒ " + strings.Join(vals, ","), nil
}

// TestTreeStableAcrossPasses 是"节点构造后不再被修改"这条不变量的兜底
// （第 1 篇汇总节、第 3 篇不变量清单）：基线在读法跑之前取，之后每一遍都比对，
// 因此"读到一半顺手改写树"也会被抓到。
//
// 值接收者加值语义的节点，已经挡住了沿字段的原地改写（接口里存的是副本，
// 拿到的是副本，改了也不落到树上）；真正还能改到树的只有一条路——调用方
// 持有 []Node，任何一遍读法都可以替换切片元素（比如"检查时顺手常量折叠"）。
// 这条用例守的就是这条路。
func TestTreeStableAcrossPasses(t *testing.T) {
	stmts := parseProgram(t, "let s = \"a\" + \"b\"\nlet n = 1 + 2 * 3\nn\n")
	before := dumpStmts(stmts) // 任何读法跑之前：树的形状

	var first string
	for pass := 1; pass <= 3; pass++ {
		got, err := readSignature(stmts)
		if err != nil {
			t.Fatalf("第 %d 遍读树失败：%v", pass, err)
		}
		if pass == 1 {
			first = got
		} else if got != first {
			t.Fatalf("第 %d 遍快照与第一遍不同：\n第一遍：%s\n第 %d 遍：%s", pass, first, pass, got)
		}
		if now := dumpStmts(stmts); now != before {
			t.Fatalf("第 %d 遍读法改动了树：\n读前：%s\n读后：%s", pass, before, now)
		}
	}
}

// ---------- 示例源码 ----------

// TestExampleFileIsValid 守住 input.toy：必须能解析 + 过类型检查 + 求值。
func TestExampleFileIsValid(t *testing.T) {
	src, err := os.ReadFile("input.toy")
	if err != nil {
		t.Fatalf("读取示例文件失败：%v", err)
	}
	stmts := parseProgram(t, string(src))
	if len(stmts) == 0 {
		t.Fatal("示例文件解析出 0 条语句")
	}
	if err := CheckProgram(stmts); err != nil {
		t.Fatalf("示例文件类型检查失败：%v", err)
	}
	env := NewEnv()
	for i, s := range stmts {
		if _, err := s.Eval(env); err != nil {
			t.Errorf("示例文件第 %d 条语句求值失败：%v", i, err)
		}
	}
}

// ---------- 演示入口 ----------

func TestRunPrintsAllLayers(t *testing.T) {
	var buf strings.Builder
	if err := run("1+2*3\n", &buf); err != nil {
		t.Fatalf("run 返回错误：%v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"== 1. 词法 ==",
		"== 2. 语法（AST）==",
		"  语句[0] = (PLUS 1 (STAR 2 3))",
		"== 2.5 类型检查 ==",
		"  (通过)",
		"== 3. 求值 ==",
		"  语句[0] 返回值 = 7",
		"== 4. 最终结果 ==",
		"  语句[0] (PLUS 1 (STAR 2 3)) = 7",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q\n--- 实际输出 ---\n%s", want, out)
		}
	}
	// NEWLINE 的字面量若是原样输出，每个 NEWLINE token 后都会多出一行空行。
	if strings.Contains(out, "\n\n") {
		t.Errorf("token 输出出现连续空行：NEWLINE 字面量没有被转义\n--- 实际输出 ---\n%s", out)
	}
}

// TestEscapeTokenLiteral 钉住 escapeTokenLiteral：控制字符显示成可见转义，其余原样。
func TestEscapeTokenLiteral(t *testing.T) {
	tests := []struct{ in, want string }{
		{"\n", `\n`},
		{"\t", `\t`},
		{"\r", `\r`},
		{"abc", "abc"},
		{"a\nb\tc", `a\nb\tc`},
	}
	for _, tt := range tests {
		if got := escapeTokenLiteral(tt.in); got != tt.want {
			t.Errorf("escapeTokenLiteral(%q) = %q，期望 %q", tt.in, got, tt.want)
		}
	}
}

func TestRunEmptyProgram(t *testing.T) {
	var buf strings.Builder
	if err := run("", &buf); err != nil {
		t.Fatalf("空程序应当成功，却返回错误：%v", err)
	}
	if !strings.Contains(buf.String(), "(空程序，没有语句)") {
		t.Errorf("空程序输出 = %q，期望包含空程序提示", buf.String())
	}
}

func TestRunReturnsErrors(t *testing.T) {
	tests := []struct {
		src   string
		wants []string
	}{
		{"1+\n", []string{"解析失败", "期望表达式"}},
		{"1/0\n", []string{"求值失败", "除数为零"}},
		{"1%0\n", []string{"求值失败", "余数为零"}},
		{`"a" - 1` + "\n", []string{"类型检查失败", "需要整数"}},
		{`1 + "a"` + "\n", []string{"类型检查失败", "两侧类型不同"}},
		{"y + 1\n", []string{"类型检查失败", "未定义变量"}},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			var buf strings.Builder
			err := run(tt.src, &buf)
			if err == nil {
				t.Fatalf("run(%q) 期望返回错误，却成功了", tt.src)
			}
			for _, want := range tt.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("run(%q) 的错误 = %q，期望包含 %q", tt.src, err, want)
				}
			}
		})
	}
}

func TestEvalErrorHasPosition(t *testing.T) {
	_, err := evalLast(t, "1/0\n")
	if err == nil {
		t.Fatal("期望除零报错，却成功了")
	}
	if !strings.Contains(err.Error(), "第 1 行，第 2 列") {
		t.Errorf("错误 = %q，期望带上位置 \"第 1 行，第 2 列\"", err)
	}
	if !strings.Contains(err.Error(), "除数为零") {
		t.Errorf("错误 = %q，期望包含 除数为零", err)
	}
}

// TestRunStringProgram 端到端验证 run 输出里的 string 值能被 Value.String()
// 正确加引号，和 SExpr 的 StringLiteral 输出对齐。
func TestRunStringProgram(t *testing.T) {
	var buf strings.Builder
	if err := run(`let s = "hi"`+"\ns + \"!\"\n", &buf); err != nil {
		t.Fatalf("run 返回错误：%v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`语句[0] = (let s "hi")`,
		`语句[1] = (PLUS s "!")`,
		`语句[0] 返回值 = "hi"`,
		`语句[1] 返回值 = "hi!"`,
		`语句[1] (PLUS s "!") = "hi!"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q\n--- 实际输出 ---\n%s", want, out)
		}
	}
}
