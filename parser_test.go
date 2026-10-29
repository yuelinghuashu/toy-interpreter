package main

import (
	"errors"
	"strings"
	"testing"
)

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

		{"比较优先级低于加法", "1 + 2 < 3 * 4\n", "(LT (PLUS 1 2) (STAR 3 4))"},
		{"比较是左结合", "1 < 2 < 3\n", "(LT (LT 1 2) 3)"},
		{"相等比较", "1 == 2\n", "(EQ 1 2)"},
		{"不等比较", "1 != 2\n", "(NEQ 1 2)"},
		{"逻辑与高于逻辑或", "true || false && true\n", "(OR true (AND false true))"},
		{"逻辑左结合", "true && false && true\n", "(AND (AND true false) true)"},
		{"一元 ! 绑定最紧", "!true && false\n", "(AND (NOT true) false)"},
		{"一元 ! 套括号", "!(true && false)\n", "(NOT (AND true false))"},
		{"连写 !!", "!!true\n", "(NOT (NOT true))"},
		{"布尔字面量", "true\n", "true"},
		{"let 布尔", "let b = 1 < 2\nb\n", "(let b (LT 1 2)) | b"},
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
		{"!\n", "期望表达式，得到 NEWLINE"},
		{"let true = 1\n", `期望标识符，得到 BOOLEAN（"true"）`},
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

// TestUnterminatedString 钉住"未闭合字符串"的独立 token 类型与错误文案：
// 不该再笼统地报"非法字符"。
func TestUnterminatedString(t *testing.T) {
	cases := []string{`"abc` + "\n", `"abc`, `1 + "abc` + "\n", "\"abc\\\n"}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			sawUnterminated := false
			for _, tok := range lexAll(src) {
				if tok.Type == UNTERMINATED_STRING {
					sawUnterminated = true
				}
				if tok.Type == ILLEGAL {
					t.Errorf("%q 里出现了 ILLEGAL，未闭合字符串应单独成 token", src)
				}
			}
			if !sawUnterminated {
				t.Fatalf("%q 没有产出 UNTERMINATED_STRING：%s", src, tokensOf(src))
			}

			_, err := NewParser(NewLexer(src)).ParseProgram()
			if err == nil {
				t.Fatalf("%q 应解析失败，却成功了", src)
			}
			if !strings.Contains(err.Error(), "字符串未闭合") {
				t.Errorf("错误 = %q，期望包含\"字符串未闭合\"", err)
			}
			if strings.Contains(err.Error(), "非法字符") {
				t.Errorf("错误 = %q，不应再出现\"非法字符\"", err)
			}
			if !errors.Is(err, ErrSyntax) {
				t.Errorf("错误类别不是 ErrSyntax：%v", err)
			}
		})
	}
}

// TestNestingDepthLimit 钉住深度闸：恰好 maxDepth 层通过，多一层带位置报错，
// 而不是栈溢出。括号、一元负号和一元 ! 是三条无界递归路径，各测一遍；二元
// 右递归今天受优先级层数限制，但也过同一道闸（为将来的右结合运算符兜底）。
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

	not := func(n int) string { return strings.Repeat("!", n) + "true\n" }
	if _, err := NewParser(NewLexer(not(maxDepth))).ParseProgram(); err != nil {
		t.Fatalf("%d 层一元 ! 应当通过，却报错：%v", maxDepth, err)
	}
	if _, err := NewParser(NewLexer(not(maxDepth + 1))).ParseProgram(); err == nil ||
		!strings.Contains(err.Error(), "嵌套过深") {
		t.Fatalf("%d 层一元 ! 的错误 = %v，期望包含\"嵌套过深\"", maxDepth+1, err)
	}
}
