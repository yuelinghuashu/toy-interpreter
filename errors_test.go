package main

import (
	"errors"
	"strings"
	"testing"
)

// TestErrorKinds 钉住错误分类锚点：errors.Is 按类别匹配，errors.As 取位置，
// 而且 fmt.Errorf("%w") 包装之后仍然有效（run() 会包「求值失败：」）。
func TestErrorKinds(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{"语法", func() error {
			_, err := NewParser(NewLexer("1+\n")).ParseProgram()
			return err
		}, ErrSyntax},
		{"未闭合字符串", func() error {
			_, err := NewParser(NewLexer(`"abc` + "\n")).ParseProgram()
			return err
		}, ErrSyntax},
		{"未定义变量（类型检查）", func() error {
			return CheckProgram(parseProgram(t, "a\n"))
		}, ErrUndefinedVar},
		{"类型不符（类型检查）", func() error {
			return CheckProgram(parseProgram(t, `"a" - 1`+"\n"))
		}, ErrType},
		{"类型不符（求值兜底）", func() error {
			_, err := evalLast(t, `"a" - 1`+"\n")
			return err
		}, ErrType},
		{"除数为零", func() error {
			_, err := evalLast(t, "1/0\n")
			return err
		}, ErrZeroDivisor},
		{"取模除零", func() error {
			_, err := evalLast(t, "1%0\n")
			return err
		}, ErrZeroDivisor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatalf("期望错误，却成功了")
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("errors.Is(%v, %v) = false", err, tt.want)
			}
			var langErr *Error
			if !errors.As(err, &langErr) {
				t.Fatalf("errors.As 取不出 *Error：%v", err)
			}
			if langErr.Pos.Line < 1 || langErr.Pos.Column < 1 {
				t.Errorf("错误没带合法位置：%+v", langErr.Pos)
			}
			if langErr.Msg == "" {
				t.Error("错误消息为空")
			}
		})
	}

	var buf strings.Builder
	if err := run("1/0\n", &buf); !errors.Is(err, ErrZeroDivisor) {
		t.Errorf("run() 包装后 errors.Is 失败：%v", err)
	}
}
