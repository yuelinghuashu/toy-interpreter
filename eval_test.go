package main

import (
	"strings"
	"testing"
)

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

		{"1 < 2\n", BoolValue(true)},
		{"2 <= 2\n", BoolValue(true)},
		{"1 > 2\n", BoolValue(false)},
		{"2 >= 3\n", BoolValue(false)},
		{"1 == 1\n", BoolValue(true)},
		{"1 != 2\n", BoolValue(true)},
		{`"a" == "a"` + "\n", BoolValue(true)},
		{"true && true\n", BoolValue(true)},
		{"true && false\n", BoolValue(false)},
		{"false || true\n", BoolValue(true)},
		{"!true\n", BoolValue(false)},
		{"!!true\n", BoolValue(true)},
		{"1 + 2 < 4\n", BoolValue(true)},
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
		{`"a" - 1` + "\n", "需要整数"},           // Binary.Eval 的守卫
		{`-"a"` + "\n", "一元负号需要整数"},          // Unary.Eval 的守卫
		{"1 && 2\n", "需要 bool"},              // Binary.Eval 短路前的类型守卫
		{"!1\n", "一元 ! 需要 bool"},             // Unary.Eval 的 OpNot 守卫
		{"true + true\n", "需要 int 或 string"}, // Binary.Eval 的 OpPlus 守卫
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

// TestLongChainParseShallowEvalDeep 钉住"长左结合链不吃 Go 栈"：解析是循环，
// 求值 / 类型检查 / SExpr 也沿左脊迭代（node.go 的 leftSpine），所以 10 万项
// 加法链在每一步都只占常数栈深。
func TestLongChainParseShallowEvalDeep(t *testing.T) {
	got, err := evalLast(t, strings.Repeat("1+", 100000)+"1\n")
	if err != nil {
		t.Fatalf("10 万项加法链求值失败：%v", err)
	}
	if got != IntValue(100001) {
		t.Errorf("10 万项加法链 = %s，期望 100001", got)
	}
}

// TestDeepLeftChainNoStackOverflow 钉住"长左结合链不压爆 Go 栈"的极端规模：
// 旧实现求值 100 万项时约 81 万层就触发 fatal stack overflow（不可 recover），
// 现在 Eval / typeCheck / writeTo 都沿左脊迭代，应当正常跑完。
func TestDeepLeftChainNoStackOverflow(t *testing.T) {
	const n = 1_000_000
	stmts := parseProgram(t, strings.Repeat("1+", n)+"1\n")
	if len(stmts) != 1 {
		t.Fatalf("解析出 %d 条语句，期望 1 条", len(stmts))
	}
	if err := CheckProgram(stmts); err != nil {
		t.Fatalf("100 万项链的类型检查失败：%v", err)
	}
	if sexpr := SExpr(stmts[0]); !strings.HasPrefix(sexpr, "(PLUS (PLUS ") {
		t.Fatalf("S-表达式开头 = %.20q，期望 (PLUS (PLUS ", sexpr)
	}
	res, err := stmts[0].Exec(NewEnv())
	if err != nil {
		t.Fatalf("100 万项链求值失败：%v", err)
	}
	if res.Value != IntValue(n+1) {
		t.Errorf("100 万项链 = %s，期望 %d", res.Value, n+1)
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

// TestEvalShortCircuit 钉住 && / || 的短路语义：左边能定结果时右边不求值。
// 右边故意放除零，"短路"与"先求两边再判断"在这里结果天差地别。
func TestEvalShortCircuit(t *testing.T) {
	tests := []struct {
		src  string
		want Value
	}{
		{"false && 1/0\n", BoolValue(false)},
		{"true || 1/0\n", BoolValue(true)},
		{"true && true\n", BoolValue(true)},
		{"false || false\n", BoolValue(false)},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			got, err := evalLast(t, tt.src)
			if err != nil {
				t.Fatalf("求值 %q 失败：%v（短路没生效）", tt.src, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("%q = %s，期望 %s", tt.src, got, tt.want)
			}
		})
	}
}
