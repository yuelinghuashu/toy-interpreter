package main

import (
	"strings"
	"testing"
)

// runREPL 跑一段输入并返回输出，方便断言。
func runREPL(t *testing.T, in string) string {
	t.Helper()
	var out strings.Builder
	if err := repl(strings.NewReader(in), &out); err != nil {
		t.Fatalf("repl 返回错误：%v", err)
	}
	return out.String()
}

func TestREPLBasic(t *testing.T) {
	got := runREPL(t, "1+2*3\nlet x = 5\nx * 2\n:quit\n")
	for _, want := range []string{"= 7", "= 5", "= 10"} {
		if !strings.Contains(got, want) {
			t.Errorf("输出缺少 %q\n--- 实际 ---\n%s", want, got)
		}
	}
}

func TestREPLVarPersistsAcrossLines(t *testing.T) {
	got := runREPL(t, "let a = 10\nlet b = a + 5\nb\n:quit\n")
	if !strings.Contains(got, "= 15") {
		t.Errorf("b 应为 15，实际：\n%s", got)
	}
}

func TestREPLErrorDoesNotKillSession(t *testing.T) {
	got := runREPL(t, "1/0\n1+1\n:quit\n")
	if !strings.Contains(got, "除数为零") {
		t.Errorf("应包含除零错误，实际：\n%s", got)
	}
	if !strings.Contains(got, "= 2") {
		t.Errorf("错误后应能继续求值 1+1=2，实际：\n%s", got)
	}
}

// TestREPLErrorRollback 钉住"失败不留僵尸绑定"：let 右边除零时，
// typeCheck 会 Bind y，但 Eval 不 Set y；若不回滚，下一句 y 会被
// 误判为已定义。
func TestREPLErrorRollback(t *testing.T) {
	got := runREPL(t, "let y = 1/0\ny\n:env\n:quit\n")
	if !strings.Contains(got, "除数为零") {
		t.Errorf("应包含除零错误，实际：\n%s", got)
	}
	if !strings.Contains(got, "未定义变量") {
		t.Errorf("y 应处于未定义状态（错误后已回滚），实际：\n%s", got)
	}
	// 只断言“未定义变量”不够：没回滚时 Eval 也会报同样的错。:env 能看出
	// typeEnv 里是否残留 y，把“事务式回滚”真正钉住。
	if strings.Contains(got, "  y = ") {
		t.Errorf("回滚后 :env 不应列出 y，实际：\n%s", got)
	}
}

func TestREPLEnvCommand(t *testing.T) {
	got := runREPL(t, "let x = 1\nlet s = \"hi\"\n:env\n:quit\n")
	if !strings.Contains(got, "x = 1 : int") {
		t.Errorf("应显示 x 的绑定，实际：\n%s", got)
	}
	if !strings.Contains(got, `s = "hi" : string`) {
		t.Errorf("应显示 s 的绑定，实际：\n%s", got)
	}
}

func TestREPLEOFExits(t *testing.T) {
	// 不输入 :quit，靠输入流 EOF 退出。
	got := runREPL(t, "1+1\n")
	if !strings.Contains(got, "= 2") {
		t.Errorf("实际输出：\n%s", got)
	}
}

func TestREPLUnknownCommand(t *testing.T) {
	got := runREPL(t, ":wat\n:quit\n")
	if !strings.Contains(got, "未知命令") {
		t.Errorf("应提示未知命令，实际：\n%s", got)
	}
}

// TestDumpREPLEnvSplit 钉住分叉检测：两套环境键不一致时显式标出缺失一侧，
// 而不是把 map 零值伪装成正常的 int/字符串。
func TestDumpREPLEnvSplit(t *testing.T) {
	var out strings.Builder
	env := NewEnv()
	env.Set("only_val", IntValue(1))
	typeEnv := NewTypeEnv()
	typeEnv.Bind("only_type", TypeInt)
	dumpREPLEnv(&out, env, typeEnv)
	got := out.String()
	for _, want := range []string{
		"only_val = 1 : <unknown>",
		"only_type = <unbound> : int",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("输出缺少 %q，实际：\n%s", want, got)
		}
	}
}

// TestREPLLongLine 钉住单行长度：默认 64KB 的 Scanner 上限会把长行截成错误，
// 提到 1MB 后一条 20 万字符的加法链应当正常求值。
func TestREPLLongLine(t *testing.T) {
	got := runREPL(t, strings.Repeat("1+", 100000)+"1\n:quit\n")
	if !strings.Contains(got, "= 100001") {
		t.Errorf("长行应求值为 100001，实际：\n%.200s", got)
	}
}

// TestREPLTooLongLineReturnsError 钉住上限之外的行为：报错而不是静默截断。
func TestREPLTooLongLineReturnsError(t *testing.T) {
	var out strings.Builder
	err := repl(strings.NewReader(strings.Repeat("1", 2<<20)+"\n"), &out)
	if err == nil {
		t.Fatal("超过 1MB 的单行应返回错误，而不应静默继续")
	}
}
