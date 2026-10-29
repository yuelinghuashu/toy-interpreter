package main

import "testing"

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
			// 标识符走 isIdentifierStart；@ 不在 oneCharPuncts / twoCharPuncts 里，是 ILLEGAL。
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
			// oneCharPuncts 表里每个条目都在这里出现一次：名字表写错顺序、或漏配一个符号，这条会先红。
			name:  "全部单字符符号",
			input: "()+-*/%=<>!\n",
			want:  `1:LPAREN:"(" 1:RPAREN:")" 1:PLUS:"+" 1:MINUS:"-" 1:STAR:"*" 1:SLASH:"/" 1:PERCENT:"%" 1:ASSIGN:"=" 1:LT:"<" 1:GT:">" 1:BANG:"!" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			name:  "双字符运算符",
			input: "== != <= >= && ||\n",
			want:  `1:EQ:"==" 1:NEQ:"!=" 1:LE:"<=" 1:GE:">=" 1:ANDAND:"&&" 1:OROR:"||" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			name:  "单字符比较与感叹号",
			input: "< > !\n",
			want:  `1:LT:"<" 1:GT:">" 1:BANG:"!" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// = 与 ==、! 与 != 靠"先查双字符"区分；这条把顺序钉住。
			name:  "= 与 ==、! 与 != 不混淆",
			input: "= == ! !=\n",
			want:  `1:ASSIGN:"=" 1:EQ:"==" 1:BANG:"!" 1:NEQ:"!=" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			name:  "布尔字面量",
			input: "true false\n",
			want:  `1:BOOLEAN:"true" 1:BOOLEAN:"false" 1:NEWLINE:"\n" 2:EOF:""`,
		},
		{
			// let 关键字；keywords 的另两个条目（true / false）见上面的布尔字面量用例。
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
