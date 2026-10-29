package main

// 第 2 层：语法。所有 AST 节点：struct + Pos + writeTo。
// 求值（Eval）在 eval.go，类型检查（typeCheck）在 typecheck.go；接口和渲染
// 入口在 ast.go。
//
// 节点按"叶子在前、组合在后"排：Number / String / Identifier 是叶子，Binary /
// Unary / Let 是组合节点。加新节点时按这个顺序插，writeTo 紧跟自己的 struct。

import (
	"strconv"
	"strings"
)

// ---------- 叶子 ----------

// NumberLiteral 是整数字面量。
type NumberLiteral struct {
	Value int64
	At    Position
}

func (n NumberLiteral) Pos() Position              { return n.At }
func (n NumberLiteral) writeTo(b *strings.Builder) { b.WriteString(strconv.FormatInt(n.Value, 10)) }

// StringLiteral 是字符串字面量。Value 存反转义后的内容（不含两侧引号），
// 转义在 parser 里做；writeTo 用 strconv.Quote 加回引号。
type StringLiteral struct {
	Value string
	At    Position
}

func (s StringLiteral) Pos() Position              { return s.At }
func (s StringLiteral) writeTo(b *strings.Builder) { b.WriteString(strconv.Quote(s.Value)) }

// Identifier 是变量引用。Name 由 lexer 保证首字符是字母或下划线。
type Identifier struct {
	Name string
	At   Position
}

func (i Identifier) Pos() Position              { return i.At }
func (i Identifier) writeTo(b *strings.Builder) { b.WriteString(i.Name) }

// ---------- 组合 ----------

// Binary 是二元运算：Left Op Right。Op 只能取 operator.go 里的 OpPlus /
// OpMinus / OpStar / OpSlash / OpPercent 之一，构造不出"运算符是 NUMBER"这种组合。
type Binary struct {
	Op          Op
	Left, Right Node
	At          Position
}

func (n Binary) Pos() Position { return n.At }
func (n Binary) writeTo(b *strings.Builder) {
	b.WriteByte('(')
	b.WriteString(n.Op.String())
	b.WriteByte(' ')
	n.Left.writeTo(b)
	b.WriteByte(' ')
	n.Right.writeTo(b)
	b.WriteByte(')')
}

// Unary 是一元运算：Op X。目前只有一元负号，Op 复用 OpMinus。
type Unary struct {
	Op Op
	X  Node
	At Position
}

func (u Unary) Pos() Position { return u.At }
func (u Unary) writeTo(b *strings.Builder) {
	b.WriteByte('(')
	b.WriteString(u.Op.String())
	b.WriteByte(' ')
	u.X.writeTo(b)
	b.WriteByte(')')
}

// Let 是变量声明：let <Name> = <Value>。Value 是任意表达式。
type Let struct {
	Name  string
	Value Node
	At    Position
}

func (l Let) Pos() Position { return l.At }
func (l Let) writeTo(b *strings.Builder) {
	b.WriteString("(let ")
	b.WriteString(l.Name)
	b.WriteByte(' ')
	l.Value.writeTo(b)
	b.WriteByte(')')
}
