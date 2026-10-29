package main

// 第 2 层：语法。本文件只放框架：Node / Expr / Stmt 接口、Position、错误类别
// 与构造（Error / errAtPos）、S-表达式渲染入口。
// 节点定义在 node.go，求值与执行在 eval.go，类型检查在 typecheck.go，环境在 env.go。
//
// 文件布局：
//   ast.go       接口（Node / Expr / Stmt / Result）/ Position / 错误类别 / SExpr / errAtPos
//   node.go      所有节点 struct + Pos + writeTo（按叶子→组合→语句排）
//   eval.go      所有 Expr 的 Eval 与 Stmt 的 Exec（顺序与 node.go 一致）
//   typecheck.go 所有节点的方法 + TypeEnv
//   env.go       变量环境

import (
	"fmt"
	"strings"
)

// ---------- 接口 ----------

// Node 是所有 AST 节点的公共部分：位置（Pos）和 S-表达式渲染（writeTo）。
// 表达式和语句各自嵌入它——表达式有值、能求值；语句没有值、只执行。
// writeTo 未导出，Node 只能由本包实现（本包是 main，外部本来就 import 不到）。
type Node interface {
	Pos() Position              // 节点在源码里的位置
	writeTo(b *strings.Builder) // 把节点写成 S-表达式
}

// Expr 是有值的表达式：求值得到 Value，静态类型检查得到 Type。
// 算术 / 比较 / 逻辑 / 字面量 / 标识符都实现它；let 和表达式语句不实现。
type Expr interface {
	Node
	Eval(env *Env) (Value, error)         // 表达式求值
	typeCheck(env *TypeEnv) (Type, error) // 静态类型检查
}

// Result 是执行一条语句的产物。
// 表达式语句和 let 带值（HasValue=true），REPL / run 靠它展示结果；
// 将来 while、if 返回零值（HasValue=false）。return / break 这类控制流也在这里
// 扩展（加控制流字段），Exec 的签名不用再变。
type Result struct {
	Value    Value
	HasValue bool
}

// Stmt 是语句：对外不产生值，执行产物放在 Result 里；类型检查只回答"合不合法"。
type Stmt interface {
	Node
	Exec(env *Env) (Result, error) // 执行语句
	typeCheck(env *TypeEnv) error  // 静态类型检查（语句没有类型）
}

// Position 是节点在源码里的位置，行、列都从 1 开始。
type Position struct {
	Line, Column int
}

func (p Position) String() string {
	return fmt.Sprintf("第 %d 行，第 %d 列", p.Line, p.Column)
}

// ---------- 错误 ----------

// ErrorKind 是语言层错误的类别。分流（REPL 提示、测试断言、未来的错误恢复）
// 一律按 Kind 判断，不要匹配错误文本。
type ErrorKind int

const (
	KindSyntax       ErrorKind = iota + 1 // 词法 / 语法错误
	KindType                              // 类型错误
	KindUndefinedVar                      // 未定义变量
	KindZeroDivisor                       // 除数为零（除法与取模）
	KindInternal                          // 不该发生：未知运算符等
)

// Error 是带位置和类别的语言层错误。
// errors.As 可以取出 *Error 读 Pos / Kind；errors.Is 可以按类别匹配哨兵
// （ErrSyntax、ErrUndefinedVar 等只填 Kind，不带位置和消息）。
type Error struct {
	Pos  Position
	Kind ErrorKind
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Pos, e.Msg) }

// Is 让 errors.Is 按类别匹配：哨兵只填 Kind，实际错误带完整位置和消息。
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t != nil && t.Kind == e.Kind
}

// 错误类别哨兵，供 errors.Is 使用。
var (
	ErrSyntax       = &Error{Kind: KindSyntax}
	ErrType         = &Error{Kind: KindType}
	ErrUndefinedVar = &Error{Kind: KindUndefinedVar}
	ErrZeroDivisor  = &Error{Kind: KindZeroDivisor}
	ErrInternal     = &Error{Kind: KindInternal}
)

// errAtPos 统一 "第 N 行，第 M 列: <msg>" 的错误格式并带上类别：
// 解析、类型检查和求值三层共用。
func errAtPos(pos Position, kind ErrorKind, format string, args ...any) error {
	return &Error{Pos: pos, Kind: kind, Msg: fmt.Sprintf(format, args...)}
}

// ---------- 渲染入口 ----------

// SExpr 把整棵树渲染成 S-表达式。整棵树共用一个 strings.Builder：若每个节点
// 各返回一段字符串再拼接，深度 d 的树会反复拷贝子树文本（O(输出长度 × 深度)）；
// 往同一个 builder 里追加则是 O(输出长度)。
func SExpr(n Node) string {
	var b strings.Builder
	n.writeTo(&b)
	return b.String()
}
