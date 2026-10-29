package main

type Type uint8

const (
	TypeInt Type = iota
	TypeString

	TypeCount
)

var typeNames = [...]string{"int", "string"}

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

func CheckProgram(stmts []Node) error {
	env := NewTypeEnv()
	for _, s := range stmts {
		if _, err := s.typeCheck(env); err != nil {
			return err
		}
	}
	return nil
}

func (n NumberLiteral) typeCheck(env *TypeEnv) (Type, error) { return TypeInt, nil }
func (s StringLiteral) typeCheck(env *TypeEnv) (Type, error) { return TypeString, nil }

func (i Identifier) typeCheck(env *TypeEnv) (Type, error) {
	if t, ok := env.Lookup(i.Name); ok {
		return t, nil
	}
	return 0, errAtPos(i.At, "未定义变量 %q", i.Name)
}

func (b Binary) typeCheck(env *TypeEnv) (Type, error) {
	lt, err := b.Left.typeCheck(env)
	if err != nil {
		return 0, err
	}
	rt, err := b.Right.typeCheck(env)
	if err != nil {
		return 0, err
	}
	switch b.Op {
	case OpPlus:
		if lt != rt {
			return 0, errAtPos(b.At, "+ 两侧类型不同：%s 和 %s", lt, rt)
		}
		return lt, nil
	case OpMinus, OpStar, OpSlash, OpPercent:
		if lt != TypeInt {
			return 0, errAtPos(b.Left.Pos(), "运算符 %s 需要整数，得到 %s", b.Op, lt)
		}
		if rt != TypeInt {
			return 0, errAtPos(b.Right.Pos(), "运算符 %s 需要整数，得到 %s", b.Op, rt)
		}
		return TypeInt, nil
	}
	return 0, errAtPos(b.At, "未知运算符 %d", b.Op)
}

func (u Unary) typeCheck(env *TypeEnv) (Type, error) {
	t, err := u.X.typeCheck(env)
	if err != nil {
		return 0, err
	}
	if t != TypeInt {
		return 0, errAtPos(u.At, "一元负号需要整数，得到 %s", t)
	}
	return TypeInt, nil
}

func (l Let) typeCheck(env *TypeEnv) (Type, error) {
	t, err := l.Value.typeCheck(env)
	if err != nil {
		return 0, err
	}
	env.Bind(l.Name, t)
	return t, nil
}
