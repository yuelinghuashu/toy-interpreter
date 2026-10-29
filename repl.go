package main

// REPL：按行读取、解析、类型检查、执行；命令以 : 开头（:help / :env / :quit，
// :h、:q、:exit 是别名）。每行失败时回滚：在临时环境里跑完才交换回主环境。

import (
	"bufio"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

const replPrompt = "toy> "

// repl 运行 REPL
func repl(in io.Reader, out io.Writer) error {
	fmt.Fprintln(out, "toy REPL — 每行一条语句，:help 查看命令，:quit 退出")
	scanner := bufio.NewScanner(in)
	// 默认 64KB 上限会把较长的一行截成 "token too long"；提到 1MB，
	// 超过上限仍会返回 scanner.Err()，由调用方处理。
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	env := NewEnv()
	typeEnv := NewTypeEnv()
	for {
		fmt.Fprintf(out, replPrompt)
		if !scanner.Scan() {
			fmt.Fprintln(out)
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, ":") {
			if handleREPLCommand(line, out, env, typeEnv) {
				return nil
			}
			continue
		}

		evalREPLLine(line, out, env, typeEnv)
	}
}

// handleREPLCommand 处理 REPL 命令，返回 true 表示退出。
func handleREPLCommand(line string, out io.Writer, env *Env, typeEnv *TypeEnv) bool {
	cmd := strings.Fields(line)[0]
	switch cmd {
	case ":quit", ":q", ":exit":
		return true
	case ":help", ":h":
		fmt.Fprintln(out, "命令：")
		fmt.Fprintln(out, "  :help  显示本帮助")
		fmt.Fprintln(out, "  :quit  退出")
		fmt.Fprintln(out, "  :env   列出当前变量绑定及类型")
		fmt.Fprintln(out, "其余输入按表达式求值；每行一条语句。")
	case ":env":
		dumpREPLEnv(out, env, typeEnv)
	default:
		fmt.Fprintf(out, "未知命令 %q，输入 :help 查看帮助\n", cmd)
	}
	return false
}

// dumpREPLEnv 列出当前变量绑定及类型。取两套环境的并集：正常流程下键一致，
// 一旦因回滚 bug 分叉，缺失的那一侧会显式打出来，不会被零值伪装成 int。
func dumpREPLEnv(out io.Writer, env *Env, typeEnv *TypeEnv) {
	names := make([]string, 0, len(env.vars)+len(typeEnv.vars))
	seen := make(map[string]bool, len(env.vars)+len(typeEnv.vars))
	for n := range env.vars {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for n := range typeEnv.vars {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	slices.Sort(names)
	if len(names) == 0 {
		fmt.Fprintln(out, "无绑定")
		return
	}
	for _, n := range names {
		v, hasVal := env.vars[n]
		t, hasType := typeEnv.vars[n]
		switch {
		case hasVal && hasType:
			fmt.Fprintf(out, "  %s = %s : %s\n", n, v, t)
		case hasVal:
			fmt.Fprintf(out, "  %s = %s : <unknown>\n", n, v)
		default:
			fmt.Fprintf(out, "  %s = <unbound> : %s\n", n, t)
		}
	}
}

// evalREPLLine 解析并执行一行；成功时更新 env/typeEnv，有值的语句才打印结果。
func evalREPLLine(line string, out io.Writer, env *Env, typeEnv *TypeEnv) {
	p := NewParser(NewLexer(line))
	stmts, err := p.ParseProgram()
	if err != nil {
		fmt.Fprintf(out, "解析失败：%v\n", err)
		return
	}
	if len(stmts) == 0 {
		return
	}

	tmpEnv := cloneEnv(env)
	tmpTypeEnv := cloneTypeEnv(typeEnv)

	for _, s := range stmts {
		if err := s.typeCheck(tmpTypeEnv); err != nil {
			fmt.Fprintf(out, "类型检查失败：%v\n", err)
			return
		}
	}

	var last Result
	for _, s := range stmts {
		res, err := s.Exec(tmpEnv)
		if err != nil {
			fmt.Fprintf(out, "求值失败：%v\n", err)
			return
		}
		if res.HasValue {
			last = res
		}
	}

	env.vars = tmpEnv.vars
	typeEnv.vars = tmpTypeEnv.vars
	if last.HasValue {
		fmt.Fprintf(out, "= %s\n", last.Value)
	}
}

// cloneEnv / cloneTypeEnv 浅拷贝一份环境。map 的大小就是用户变量数，
// REPL 里几乎可以忽略开销；换来的是"一行失败 = 零副作用"。
func cloneEnv(e *Env) *Env {
	ne := NewEnv()
	maps.Copy(ne.vars, e.vars)
	return ne
}

func cloneTypeEnv(e *TypeEnv) *TypeEnv {
	ne := NewTypeEnv()
	maps.Copy(ne.vars, e.vars)
	return ne
}
