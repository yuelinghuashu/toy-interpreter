package main

// 第 2 层：语法。本文件只放框架：Node 接口、Position、S-表达式渲染入口和
// 各层共用的带位置错误构造。
// 节点定义在 node.go，求值在 eval.go，类型检查在 typecheck.go，环境在 env.go。
//
// 文件布局：
//   ast.go       接口 / Position / SExpr / errAtPos
//   node.go      所有节点 struct + Pos + writeTo（按叶子→组合排）
//   eval.go      所有 Eval 方法（顺序与 node.go 一致）
//   typecheck.go 所有 typeCheck 方法 + TypeEnv
//   env.go       变量环境

import (
	"fmt"
	"strings"
)

// ---------- 接口 ----------

// Node 是语句节点的接口：表达式节点和 let 语句都实现它。
// 接口有四个能力：位置（Pos）、渲染（writeTo，node.go）、求值（Eval，eval.go）
// 和静态类型检查（typeCheck，typecheck.go）。
// writeTo 未导出，Node 只能由本包实现（本包是 main，外部本来就 import 不到）。
type Node interface {
	Pos() Position                        // 节点在源码里的位置
	writeTo(b *strings.Builder)           // 把节点写成 S-表达式
	Eval(env *Env) (Value, error)         // 表达式求值
	typeCheck(env *TypeEnv) (Type, error) // 静态类型检查
}

// Position 是节点在源码里的位置，行、列都从 1 开始。
type Position struct {
	Line, Column int
}

func (p Position) String() string {
	return fmt.Sprintf("第 %d 行，第 %d 列", p.Line, p.Column)
}

// errAtPos 统一 "第 N 行，第 M 列: <msg>" 的错误格式，解析、类型检查和求值三层共用。
func errAtPos(pos Position, format string, args ...any) error {
	return fmt.Errorf("%s: %s", pos, fmt.Sprintf(format, args...))
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
