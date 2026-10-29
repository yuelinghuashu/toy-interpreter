package main

import (
	"strings"
	"testing"
)

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
		{"1 && 2\n", "需要 bool"},
		{"true && 2\n", "需要 bool"},
		{"1 || true\n", "需要 bool"},
		{"1 < true\n", "需要整数"},
		{"true < false\n", "需要整数"},
		{`1 == "a"` + "\n", "两侧类型不同"},
		{"!1\n", "一元 ! 需要 bool"},
		{"true + true\n", "需要 int 或 string"},
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

// TestTypeCheckPass 钉住合法程序的类型检查：字符串 +、比较 / 逻辑 / 一元 !、
// let 绑定、共享 TypeEnv。
func TestTypeCheckPass(t *testing.T) {
	for _, src := range []string{
		"1+2*3\n",
		`"a" + "b"` + "\n",
		`let s = "hi"` + "\ns\n",
		`let a = "x"` + "\nlet b = a + \"y\"" + "\nb\n",
		"let x = 5\nx + 2\n",
		"1 < 2\n",
		"true && false\n",
		"!true\n",
		"1 == 1\n",
		`"a" == "b"` + "\n",
		"let b = 1 < 2\nb && true\n",
	} {
		t.Run(src, func(t *testing.T) {
			if err := CheckProgram(parseProgram(t, src)); err != nil {
				t.Fatalf("%q 期望通过类型检查，却报错：%v", src, err)
			}
		})
	}
}

// TestTypeCheckInvalidResult 钉住错误路径的返回值：TypeCount（非法类型），不是 0——
// Type 的零值恰好是 TypeInt，拿返回值当类型用会把错误藏起来。
func TestTypeCheckInvalidResult(t *testing.T) {
	bad := Binary{
		Op:    OpMinus,
		Left:  StringLiteral{Value: "a"},
		Right: NumberLiteral{Value: 1},
	}
	got, err := bad.typeCheck(NewTypeEnv())
	if err == nil {
		t.Fatal("期望类型错误，却通过了检查")
	}
	if got != TypeCount {
		t.Errorf("错误路径返回 %d（%s），期望 TypeCount", got, got)
	}
}
