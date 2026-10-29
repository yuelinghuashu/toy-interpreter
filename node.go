package main

// 第 2 层：语法。所有 AST 节点：struct + Pos + writeTo。
// 求值与执行在 eval.go，类型检查在 typecheck.go；接口在 ast.go。
//
// 节点按"叶子 → 组合 → 语句"排：Number / String / Bool / Identifier 是叶子
// 表达式，Binary / Unary 是组合表达式，ExprStmt / Let 是语句。加新节点时按这个
// 顺序插，writeTo 紧跟自己的 struct。

import (
	"strconv"
	"strings"
)

// ---------- 叶子表达式 ----------

// NumberLiteral 是整数字面量。
type NumberLiteral struct {
	Value int64
	At    Position
}

func (n NumberLiteral) Pos() Position              { return n.At }
func (n NumberLiteral) writeTo(b *strings.Builder) { b.WriteString(strconv.FormatInt(n.Value, 10)) }

// StringLiteral 是字符串字面量。Value 存反转义后的内容（不含两侧引号），
// 转义在 parser 里做；writeTo 用 strconv.Quote 加回引号。
// unquoteString 认得 strconv.Quote 可能产出的全部转义，所以 SExpr 的输出
// 能再喂回解析器（见 TestStringQuoteRoundTrip）。
type StringLiteral struct {
	Value string
	At    Position
}

func (s StringLiteral) Pos() Position              { return s.At }
func (s StringLiteral) writeTo(b *strings.Builder) { b.WriteString(strconv.Quote(s.Value)) }

// BoolLiteral 是布尔字面量。
type BoolLiteral struct {
	Value bool
	At    Position
}

func (b BoolLiteral) Pos() Position { return b.At }
func (b BoolLiteral) writeTo(w *strings.Builder) {
	if b.Value {
		w.WriteString("true")
	} else {
		w.WriteString("false")
	}
}

// Identifier 是变量引用。Name 由 lexer 保证首字符是字母或下划线。
type Identifier struct {
	Name string
	At   Position
}

func (i Identifier) Pos() Position              { return i.At }
func (i Identifier) writeTo(b *strings.Builder) { b.WriteString(i.Name) }

// ---------- 组合表达式 ----------

// Binary 是二元运算：Left Op Right。Op 只能取 binOps（operator.go）里的那几种：
// 算术 OpPlus / OpMinus / OpStar / OpSlash / OpPercent，比较 OpEq / OpNeq /
// OpLt / OpLe / OpGt / OpGe，逻辑 OpAnd / OpOr，构造不出"运算符是 NUMBER"这种
// 组合；一元专用的 OpNot 不会落在这里。
//
// Left 链（左脊）可以很长（"1+1+…" 十多万项），所以 Eval / typeCheck / writeTo
// 都沿左脊迭代、再自内向外逐层应用，不靠递归吃它；递归只剩右子树，深度受
// parser 的优先级层数与 maxDepth 限制。
type Binary struct {
	Op          Op
	Left, Right Expr
	At          Position
}

// binaryStep 是沿左脊展开 Binary 得到的一层：把 Op 作用在「左侧已算出的结果」
// 和 Right 上。leftSpine 把左结合链摊平成 steps，长度等于链的项数（堆上的切片，
// 不是 Go 栈），Eval / typeCheck / writeTo 共用这套迭代方式。
type binaryStep struct {
	op    Op
	at    Position
	right Expr
}

// leftSpine 把 root 的左脊（一路 Left 上的 Binary）展开成从外层到内层的 steps，
// 返回最左的非 Binary 操作数。
func leftSpine(root Binary) (steps []binaryStep, leftmost Expr) {
	cur := Expr(root)
	for {
		bin, ok := cur.(Binary)
		if !ok {
			return steps, cur
		}
		steps = append(steps, binaryStep{op: bin.Op, at: bin.At, right: bin.Right})
		cur = bin.Left
	}
}

func (n Binary) Pos() Position { return n.At }

// writeTo 沿左脊迭代：下行时先写 "(op "，到最左操作数后，再自内向外补上
// 右子树和 ")"。长左结合链的项数只影响循环次数，不影响 Go 栈。
func (n Binary) writeTo(b *strings.Builder) {
	steps, leftmost := leftSpine(n)
	for _, st := range steps {
		b.WriteByte('(')
		b.WriteString(st.op.String())
		b.WriteByte(' ')
	}
	leftmost.writeTo(b)
	for i := len(steps) - 1; i >= 0; i-- {
		b.WriteByte(' ')
		steps[i].right.writeTo(b)
		b.WriteByte(')')
	}
}

// Unary 是一元运算：Op X。目前有一元负号（复用 OpMinus）和一元 !（OpNot）。
type Unary struct {
	Op Op
	X  Expr
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

// ---------- 语句 ----------

// ExprStmt 是表达式语句：单独出现的表达式。它的位置和渲染都沿用内部表达式，
// 所以 "1+2" 的 S-表达式仍是 "(PLUS 1 2)"，与拆分前逐字相同。
type ExprStmt struct {
	X Expr
}

func (s ExprStmt) Pos() Position              { return s.X.Pos() }
func (s ExprStmt) writeTo(b *strings.Builder) { s.X.writeTo(b) }

// Let 是变量声明：let <Name> = <Value>。Value 是任意表达式（Expr，不是 Node，
// 所以 let 不能出现在表达式位置）。
type Let struct {
	Name  string
	Value Expr
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

// 编译期钉桩：节点归属一旦写错（例如把 let 放进表达式的 Left），这里就编不过。
var (
	_ Expr = NumberLiteral{}
	_ Expr = StringLiteral{}
	_ Expr = BoolLiteral{}
	_ Expr = Identifier{}
	_ Expr = Binary{}
	_ Expr = Unary{}
	_ Stmt = ExprStmt{}
	_ Stmt = Let{}
)
