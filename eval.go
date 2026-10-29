package main

// 第 3 层：求值。所有节点的 Eval 方法；结构在 node.go。
//
// 按 node.go 的节点顺序排（叶子→组合），加新节点时两处一起加、顺序一致。
// 求值错误统一用 errAtPos 构造（ast.go），格式与解析错误一致（"第 N 行，第 M 列: ..."）。

// ---------- 叶子 ----------

// Eval 是 NumberLiteral 的求值：字面量就是它自己。
func (n NumberLiteral) Eval(env *Env) (Value, error) { return IntValue(n.Value), nil }
func (s StringLiteral) Eval(env *Env) (Value, error) { return StringValue(s.Value), nil }

// Eval 是 Identifier 的求值：查环境；未定义带标识符位置报错。
func (i Identifier) Eval(env *Env) (Value, error) {
	if v, ok := env.Get(i.Name); ok {
		return v, nil
	}
	return Value{}, errAtPos(i.At, "未定义变量 %q", i.Name)
}

// ---------- 组合 ----------

// Eval 是 Binary 的求值：先递归求两边，再按运算符算。
// 求值错误带上运算符的位置（errAtPos），和解析错误的格式一致。
// 整数溢出按 Go 语义静默回绕（9223372036854775807 + 1 会变成负数），demo 不检查。
func (b Binary) Eval(env *Env) (Value, error) {
	left, err := b.Left.Eval(env)
	if err != nil {
		return Value{}, err
	}
	right, err := b.Right.Eval(env)
	if err != nil {
		return Value{}, err
	}

	switch b.Op {
	case OpPlus:
		if left.IsInt() && right.IsInt() {
			return IntValue(left.AsInt() + right.AsInt()), nil
		}
		if left.IsString() && right.IsString() {
			return StringValue(left.AsString() + right.AsString()), nil
		}
		return Value{}, errAtPos(b.At, "+ 两侧类型不同：%s 和 %s",
			left.TypeName(), right.TypeName())

	case OpMinus, OpStar, OpSlash, OpPercent:
		// typecheck 已保证两边是 int；这里再挡一次，兜住绕过类型检查直接求值的调用方。
		if !left.IsInt() || !right.IsInt() {
			return Value{}, errAtPos(b.At, "运算符 %s 需要整数，得到 %s 和 %s",
				b.Op, left.TypeName(), right.TypeName())
		}

		l, r := left.AsInt(), right.AsInt()
		var res int64

		switch b.Op {
		case OpMinus:
			res = l - r
		case OpStar:
			res = l * r
		case OpSlash:
			if r == 0 {
				return Value{}, errAtPos(b.At, "除数为零")
			}
			res = l / r
		case OpPercent:
			if r == 0 {
				return Value{}, errAtPos(b.At, "余数为零")
			}
			res = l % r
		}
		return IntValue(res), nil

	default:
		return Value{}, errAtPos(b.At, "未知运算符 %d", b.Op)
	}
}

// Eval 是一元运算的求值：目前只有一元负号。
func (u Unary) Eval(env *Env) (Value, error) {
	x, err := u.X.Eval(env)
	if err != nil {
		return Value{}, err
	}
	switch u.Op {
	case OpMinus:
		// 与 Binary 一致：typecheck 已保证 x 是 int，这里再挡一次直接求值的调用方。
		if !x.IsInt() {
			return Value{}, errAtPos(u.At, "一元负号需要整数，得到 %s", x.TypeName())
		}
		return IntValue(-x.AsInt()), nil
	default:
		return Value{}, errAtPos(u.At, "未知一元运算符 %d", u.Op)
	}
}

// Eval 是 let 的求值：先算右侧，再写入环境，返回值就是赋的值——
// 这样 "语句[0] 返回值 = ..." 有信息量，比固定返回 0 更贴近 REPL 直觉。
func (l Let) Eval(env *Env) (Value, error) {
	v, err := l.Value.Eval(env)
	if err != nil {
		return Value{}, err
	}
	env.Set(l.Name, v)
	return v, nil
}
