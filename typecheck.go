package main

// 第 2.5 层：静态类型检查。表达式（Expr）有类型，语句（Stmt）只回答"合不合法"。
//
//   - Expr.typeCheck(env) (Type, error)：出错返回 TypeCount（不是合法类型，
//     String() 得到 "?"）——Type 的零值恰好是 TypeInt，直接拿返回值当类型用会把
//     错误藏起来，所以调用方必须先看 error。
//   - Stmt.typeCheck(env) error：while / if 的条件检查、let 的绑定都在这里，
//     语句本身没有类型，所以不返回 Type。

type Type uint8

const (
	TypeInt Type = iota
	TypeString
	TypeBool

	// TypeCount 不是类型，是总数；同时用作错误路径的返回值。
	TypeCount
)

var typeNames = [...]string{"int", "string", "bool"}

var _ [int(TypeCount) - len(typeNames)]struct{}
var _ [len(typeNames) - int(TypeCount)]struct{}

func (t Type) String() string {
	if t < TypeCount {
		return typeNames[t]
	}
	return "?"
}

// TypeEnv 是类型环境，记录变量的类型。
type TypeEnv struct {
	vars map[string]Type
}

// NewTypeEnv 创建一个新的类型环境。
func NewTypeEnv() *TypeEnv { return &TypeEnv{vars: make(map[string]Type)} }

// Lookup 查找变量名的类型，返回类型和是否存在该变量。
func (e *TypeEnv) Lookup(name string) (Type, bool) {
	t, ok := e.vars[name]
	return t, ok
}

// Bind 绑定变量名的类型。
func (e *TypeEnv) Bind(name string, t Type) { e.vars[name] = t }

// CheckProgram 逐条语句做类型检查：let 绑定写进同一个 TypeEnv，后面的语句能看到。
func CheckProgram(stmts []Stmt) error {
	env := NewTypeEnv()
	for _, s := range stmts {
		if err := s.typeCheck(env); err != nil {
			return err
		}
	}
	return nil
}

// ---------- 表达式 ----------

func (n NumberLiteral) typeCheck(env *TypeEnv) (Type, error) { return TypeInt, nil }
func (s StringLiteral) typeCheck(env *TypeEnv) (Type, error) { return TypeString, nil }
func (b BoolLiteral) typeCheck(env *TypeEnv) (Type, error)   { return TypeBool, nil }

func (i Identifier) typeCheck(env *TypeEnv) (Type, error) {
	if t, ok := env.Lookup(i.Name); ok {
		return t, nil
	}
	return TypeCount, errAtPos(i.At, KindUndefinedVar, "未定义变量 %q", i.Name)
}

// typeCheck 沿左脊迭代展开（node.go 的 leftSpine），再自内向外逐层合并类型：
// 长左结合链只影响 steps 切片长度，不影响 Go 栈。leftPos 跟着合并结果走——
// 内层那一层的左操作数是最左节点，外层的是刚合出来的子树（位置取内层运算符）。
func (b Binary) typeCheck(env *TypeEnv) (Type, error) {
	steps, leftmost := leftSpine(b)
	lt, err := leftmost.typeCheck(env)
	if err != nil {
		return TypeCount, err
	}
	leftPos := leftmost.Pos()
	for i := len(steps) - 1; i >= 0; i-- {
		st := steps[i]
		rt, err := st.right.typeCheck(env)
		if err != nil {
			return TypeCount, err
		}
		lt, err = applyType(st, lt, leftPos, rt)
		if err != nil {
			return TypeCount, err
		}
		leftPos = st.at
	}
	return lt, nil
}

// applyType 用左右操作数的类型算一层二元运算的类型；leftPos 是左操作数（可能是
// 合出来的子树）的位置，右侧用 st.right.Pos()。
func applyType(st binaryStep, lt Type, leftPos Position, rt Type) (Type, error) {
	switch st.op {
	case OpPlus:
		if lt != rt {
			return TypeCount, errAtPos(st.at, KindType, "+ 两侧类型不同：%s 和 %s", lt, rt)
		}
		if lt != TypeInt && lt != TypeString {
			return TypeCount, errAtPos(st.at, KindType, "运算符 + 需要 int 或 string，得到 %s", lt)
		}
		return lt, nil

	case OpMinus, OpStar, OpSlash, OpPercent:
		if lt != TypeInt {
			return TypeCount, errAtPos(leftPos, KindType, "运算符 %s 需要整数，得到 %s", st.op, lt)
		}
		if rt != TypeInt {
			return TypeCount, errAtPos(st.right.Pos(), KindType, "运算符 %s 需要整数，得到 %s", st.op, rt)
		}
		return TypeInt, nil

	case OpEq, OpNeq:
		if lt != rt {
			return TypeCount, errAtPos(st.at, KindType, "%s 两侧类型不同：%s 和 %s", st.op, lt, rt)
		}
		return TypeBool, nil

	case OpLt, OpLe, OpGt, OpGe:
		if lt != TypeInt {
			return TypeCount, errAtPos(leftPos, KindType, "运算符 %s 需要整数，得到 %s", st.op, lt)
		}
		if rt != TypeInt {
			return TypeCount, errAtPos(st.right.Pos(), KindType, "运算符 %s 需要整数，得到 %s", st.op, rt)
		}
		return TypeBool, nil

	case OpAnd, OpOr:
		if lt != TypeBool {
			return TypeCount, errAtPos(leftPos, KindType, "运算符 %s 需要 bool，得到 %s", st.op, lt)
		}
		if rt != TypeBool {
			return TypeCount, errAtPos(st.right.Pos(), KindType, "运算符 %s 需要 bool，得到 %s", st.op, rt)
		}
		return TypeBool, nil
	}
	return TypeCount, errAtPos(st.at, KindInternal, "未知运算符 %d", st.op)
}

// typeCheck 一元运算：负号要 int，! 要 bool。
func (u Unary) typeCheck(env *TypeEnv) (Type, error) {
	t, err := u.X.typeCheck(env)
	if err != nil {
		return TypeCount, err
	}
	switch u.Op {
	case OpMinus:
		if t != TypeInt {
			return TypeCount, errAtPos(u.At, KindType, "一元负号需要整数，得到 %s", t)
		}
		return TypeInt, nil
	case OpNot:
		if t != TypeBool {
			return TypeCount, errAtPos(u.At, KindType, "一元 ! 需要 bool，得到 %s", t)
		}
		return TypeBool, nil
	}
	return TypeCount, errAtPos(u.At, KindInternal, "未知一元运算符 %d", u.Op)
}

// ---------- 语句 ----------

// typeCheck 表达式语句：检查内部表达式，值本身不进入类型环境。
func (s ExprStmt) typeCheck(env *TypeEnv) error {
	_, err := s.X.typeCheck(env)
	return err
}

// typeCheck let：把右侧的类型绑进环境，重复 let 允许覆盖（与 Env.Set 一致）。
func (l Let) typeCheck(env *TypeEnv) error {
	t, err := l.Value.typeCheck(env)
	if err != nil {
		return err
	}
	env.Bind(l.Name, t)
	return nil
}
