package main

import (
	"fmt"
	"strings"
	"testing"
)

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

// dumpColumn 把 token 序列压成 "类型 行:列" 快照；列号单独测，不混进上面的行快照。
func dumpColumn(tokens []Token) string {
	parts := make([]string, len(tokens))
	for i, tk := range tokens {
		parts[i] = fmt.Sprintf("%s %d:%d", tk.Type, tk.Line, tk.Column)
	}
	return strings.Join(parts, " ")
}

// parseProgram 解析整段源码；失败直接 Fatal。
func parseProgram(t *testing.T, src string) []Stmt {
	t.Helper()
	p := NewParser(NewLexer(src))
	stmts, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("解析 %q 失败：%v", src, err)
	}
	return stmts
}

// dumpStmts 把语句列表渲染成 "a | b | c"，用 S-表达式展示 AST 形状。
func dumpStmts(stmts []Stmt) string {
	parts := make([]string, len(stmts))
	for i, s := range stmts {
		parts[i] = SExpr(s)
	}
	return strings.Join(parts, " | ")
}

// stmtExpr 取出表达式语句内部的 Expr；测试里对解析结果做类型断言时用。
func stmtExpr(t *testing.T, s Stmt) Expr {
	t.Helper()
	es, ok := s.(ExprStmt)
	if !ok {
		t.Fatalf("语句是 %T，期望 ExprStmt", s)
	}
	return es.X
}

// evalLast 解析并逐条执行，返回最后一条"带值"语句的值。env 建在循环外，
// 多条语句共享绑定（let 与后续引用）。
func evalLast(t *testing.T, src string) (Value, error) {
	t.Helper()
	stmts := parseProgram(t, src)
	env := NewEnv()
	var last Result
	for _, s := range stmts {
		res, err := s.Exec(env)
		if err != nil {
			return Value{}, err
		}
		if res.HasValue {
			last = res
		}
	}
	return last.Value, nil
}

// readSignature 把三段读法各跑一遍，压成一行快照：S-表达式、类型检查、执行结果。
// 每次都用新的 TypeEnv / Env，这样"环境复用"不会被误当成"节点被写回"。
func readSignature(stmts []Stmt) (string, error) {
	if err := CheckProgram(stmts); err != nil {
		return "", err
	}
	env := NewEnv()
	vals := make([]string, 0, len(stmts))
	for _, s := range stmts {
		res, err := s.Exec(env)
		if err != nil {
			return "", err
		}
		if !res.HasValue {
			continue
		}
		vals = append(vals, res.Value.String())
	}
	return dumpStmts(stmts) + " ⇒ " + strings.Join(vals, ","), nil
}
