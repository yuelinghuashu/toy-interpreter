package main

import (
	"strings"
	"testing"
)

// TestTreeStableAcrossPasses 是"节点构造后不再被修改"这条不变量的兜底：
// 基线在读法跑之前取，之后每一遍都比对，因此"读到一半顺手改写树"也会被抓到。
//
// 值接收者加值语义的节点，已经挡住了沿字段的原地改写（接口里存的是副本，
// 拿到的是副本，改了也不落到树上）；真正还能改到树的只有一条路——调用方
// 持有 []Stmt，任何一遍读法都可以替换切片元素（比如"检查时顺手常量折叠"）。
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

// TestExampleFilesAreValid 守住 examples/：每个示例都要能解析 + 过类型检查 + 求值。
// 示例按语法分主题，新增示例文件会被自动纳入覆盖。
func TestExampleFilesAreValid(t *testing.T) {
	programs, err := loadPrograms("examples")
	if err != nil {
		t.Fatalf("读取示例目录失败：%v", err)
	}
	if len(programs) == 0 {
		t.Fatal("examples/ 下没有示例文件")
	}
	for _, p := range programs {
		t.Run(p.name, func(t *testing.T) {
			stmts := parseProgram(t, p.src)
			if len(stmts) == 0 {
				t.Fatalf("%s 解析出 0 条语句", p.name)
			}
			if err := CheckProgram(stmts); err != nil {
				t.Fatalf("%s 类型检查失败：%v", p.name, err)
			}
			env := NewEnv()
			for i, s := range stmts {
				if _, err := s.Exec(env); err != nil {
					t.Errorf("%s 第 %d 条语句求值失败：%v", p.name, i, err)
				}
			}
		})
	}
}

// TestLoadPrograms 钉住默认入口：目录取全部 .toy 且按文件名排序，单个文件只取一个。
func TestLoadPrograms(t *testing.T) {
	programs, err := loadPrograms("examples")
	if err != nil {
		t.Fatalf("loadPrograms(examples) 失败：%v", err)
	}
	if len(programs) < 2 {
		t.Fatalf("examples/ 下只读到 %d 个文件，期望至少 2 个", len(programs))
	}
	for i := 1; i < len(programs); i++ {
		if programs[i-1].name >= programs[i].name {
			t.Errorf("示例文件未按文件名排序：%q 在 %q 之前", programs[i-1].name, programs[i].name)
		}
	}

	one, err := loadPrograms(programs[0].name)
	if err != nil {
		t.Fatalf("loadPrograms(%q) 失败：%v", programs[0].name, err)
	}
	if len(one) != 1 || one[0].src != programs[0].src {
		t.Errorf("单文件读取应原样返回该文件，实际读到 %d 个", len(one))
	}

	if _, err := loadPrograms("不存在的路径"); err == nil {
		t.Error("读取不存在的路径应返回错误")
	}
}

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
		{`1 && 2` + "\n", []string{"类型检查失败", "需要 bool"}},
		{`!1` + "\n", []string{"类型检查失败", "需要 bool"}},
		{"true + true\n", []string{"类型检查失败", "需要 int 或 string"}},
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
