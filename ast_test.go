package main

import (
	"strings"
	"testing"
)

// TestExprStmtWrapsExpression 钉住"表达式语句"这层包装：位置和 S-表达式都沿用
// 内部表达式，所以拆分前后 run / dumpStmts 的输出逐字不变。
func TestExprStmtWrapsExpression(t *testing.T) {
	stmts := parseProgram(t, "1+2*3\nlet x = 1\nx\n")
	if len(stmts) != 3 {
		t.Fatalf("解析出 %d 条语句，期望 3 条", len(stmts))
	}

	first, ok := stmts[0].(ExprStmt)
	if !ok {
		t.Fatalf("第一条语句是 %T，期望 ExprStmt", stmts[0])
	}
	if _, ok := first.X.(Binary); !ok {
		t.Fatalf("表达式语句内部是 %T，期望 Binary", first.X)
	}
	if got, want := SExpr(stmts[0]), SExpr(first.X); got != want {
		t.Errorf("语句的 S-表达式 = %q，期望与内部表达式一致 %q", got, want)
	}
	if stmts[0].Pos() != first.X.Pos() {
		t.Errorf("语句位置 %v 与内部表达式 %v 不一致", stmts[0].Pos(), first.X.Pos())
	}

	if _, ok := stmts[1].(Let); !ok {
		t.Fatalf("第二条语句是 %T，期望 Let", stmts[1])
	}
}

// TestStmtResultContract 钉住 Exec 的契约：let 与表达式语句都带值（HasValue），
// REPL / run 靠它展示结果；语句本身不写回树。
func TestStmtResultContract(t *testing.T) {
	stmts := parseProgram(t, "let x = 2 + 3\nx * 2\n")
	env := NewEnv()

	first, err := stmts[0].Exec(env)
	if err != nil {
		t.Fatalf("let 执行失败：%v", err)
	}
	if !first.HasValue || first.Value != IntValue(5) {
		t.Errorf("let 的 Result = %+v，期望带值 5", first)
	}

	second, err := stmts[1].Exec(env)
	if err != nil {
		t.Fatalf("表达式语句执行失败：%v", err)
	}
	if !second.HasValue || second.Value != IntValue(10) {
		t.Errorf("表达式语句的 Result = %+v，期望带值 10", second)
	}
}

// nopStmt 是测试用语句：没有返回值。它验证 Stmt 接口本身允许"无值语句"，
// 将来 while / if 可以照这个形状落地（Result{} + 只做类型检查）。
type nopStmt struct {
	at Position
}

func (s nopStmt) Pos() Position              { return s.at }
func (s nopStmt) writeTo(b *strings.Builder) { b.WriteString("(nop)") }
func (s nopStmt) typeCheck(*TypeEnv) error   { return nil }
func (s nopStmt) Exec(*Env) (Result, error)  { return Result{}, nil }

// TestValuelessStmtIsAllowed 钉住"语句可以没有值"这条未来契约。
func TestValuelessStmtIsAllowed(t *testing.T) {
	s := nopStmt{at: Position{Line: 1, Column: 1}}
	if err := CheckProgram([]Stmt{s}); err != nil {
		t.Errorf("无值语句应当通过类型检查，却报错：%v", err)
	}
	res, err := s.Exec(NewEnv())
	if err != nil || res.HasValue {
		t.Errorf("无值语句的 Exec = (%+v, %v)，期望 Result{} 且无错", res, err)
	}
	if got := SExpr(s); got != "(nop)" {
		t.Errorf("SExpr(nopStmt) = %q，期望 (nop)", got)
	}
}
