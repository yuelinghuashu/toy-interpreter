package main

import (
	"errors"
	"testing"
)

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

// TestStringQuoteRoundTrip 钉住 SExpr / Value.String 的引用结果能被 parser 再解析。
// strconv.Quote 会产出 \a \b \f \v \xHH \uHHHH \UHHHHHHHH，unquoteString 必须
// 全部认得，否则含控制字符的字符串在 SExpr 之后会报「未知转义序列」。
func TestStringQuoteRoundTrip(t *testing.T) {
	var samples []string
	for b := 0; b < 256; b++ {
		samples = append(samples, string([]byte{byte(b)}))
	}
	for _, r := range []rune{'\u00a0', '\u2028', '\uFFFD', '你', '\U0010FFFF'} {
		samples = append(samples, string(r))
	}

	for _, s := range samples {
		src := SExpr(StringLiteral{Value: s})
		stmts := parseProgram(t, src+"\n")
		if len(stmts) != 1 {
			t.Fatalf("%q 的 SExpr %q 解析出 %d 条语句", s, src, len(stmts))
		}
		got, ok := stmtExpr(t, stmts[0]).(StringLiteral)
		if !ok {
			t.Fatalf("%q 的 SExpr %q 解析出的表达式是 %T", s, src, stmtExpr(t, stmts[0]))
		}
		if got.Value != s {
			t.Errorf("round-trip 变了：%q -> SExpr %q -> %q", s, src, got.Value)
		}
	}
}

// TestStringEscapesExtended 覆盖新增的 \a \b \f \v \x \u \U，以及它们的错误路径。
func TestStringEscapesExtended(t *testing.T) {
	cases := []struct{ src, want string }{
		{`"\a\b\f\v"` + "\n", "\a\b\f\v"},
		{`"\x41\x42"` + "\n", "AB"},
		{`"\xff"` + "\n", "\xff"}, // 裸字节，不要求是合法 UTF-8
		{`"\u4f60"` + "\n", "你"},
		{`"\U0001F600"` + "\n", "😀"},
	}
	for _, tt := range cases {
		t.Run(tt.src, func(t *testing.T) {
			stmts := parseProgram(t, tt.src)
			sl, ok := stmtExpr(t, stmts[0]).(StringLiteral)
			if !ok {
				t.Fatalf("%q 解析出的表达式是 %T，期望 StringLiteral", tt.src, stmtExpr(t, stmts[0]))
			}
			if sl.Value != tt.want {
				t.Errorf("%q = %q，期望 %q", tt.src, sl.Value, tt.want)
			}
		})
	}

	for _, src := range []string{`"\x4"`, `"\xGG"`, `"\u12"`, `"\uD800"`, `"\U00110000"`, `"\q"`} {
		t.Run(src, func(t *testing.T) {
			_, err := NewParser(NewLexer(src + "\n")).ParseProgram()
			if err == nil {
				t.Fatalf("%q 期望报错，却解析成功", src)
			}
			if !errors.Is(err, ErrSyntax) {
				t.Errorf("%q 的错误类别不是 ErrSyntax：%v", src, err)
			}
		})
	}
}
