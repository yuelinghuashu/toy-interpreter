package main

// 第 3 层：求值与执行。Expr 实现 Eval（有值），Stmt 实现 Exec（用 Result 传值）。
// 结构在 node.go。
//
// 按 node.go 的节点顺序排（叶子 → 组合 → 语句），加新节点时两处一起加、顺序一致。
// 错误统一用 errAtPos 构造（ast.go），带位置和类别：上层可以用 errors.Is 按
// ErrType / ErrUndefinedVar / ErrZeroDivisor 分流，errors.As 取出 *Error 读位置。

// ---------- 叶子表达式 ----------

// Eval 是 NumberLiteral 的求值：字面量就是它自己。
func (n NumberLiteral) Eval(env *Env) (Value, error) { return IntValue(n.Value), nil }
func (s StringLiteral) Eval(env *Env) (Value, error) { return StringValue(s.Value), nil }
func (b BoolLiteral) Eval(env *Env) (Value, error)   { return BoolValue(b.Value), nil }

// Eval 是 Identifier 的求值：查环境；未定义带标识符位置报错。
func (i Identifier) Eval(env *Env) (Value, error) {
	if v, ok := env.Get(i.Name); ok {
		return v, nil
	}
	return Value{}, errAtPos(i.At, KindUndefinedVar, "未定义变量 %q", i.Name)
}

// ---------- 组合表达式 ----------

// Eval 是 Binary 的求值。沿左脊迭代展开（node.go 的 leftSpine），再自内向外
// 逐层应用：左结合链的项数只影响 steps 切片长度，不影响 Go 栈；递归只剩右子树，
// 其深度受 parseExpr 的优先级层数（<= 5）和 maxDepth（括号、一元，<= 1000 层）限制。
// && / || 的短路在逐层应用时天然成立：左侧已能定结果时，Right 根本不会被求值。
// 整数溢出按 Go 语义静默回绕（9223372036854775807 + 1 会变成负数），demo 不检查。
func (b Binary) Eval(env *Env) (Value, error) {
	steps, leftmost := leftSpine(b)
	left, err := leftmost.Eval(env)
	if err != nil {
		return Value{}, err
	}
	for i := len(steps) - 1; i >= 0; i-- {
		left, err = applyBinary(steps[i], left, env)
		if err != nil {
			return Value{}, err
		}
	}
	return left, nil
}

// applyBinary 用左值算一层二元运算。右子树在这里才求值，所以短路自然生效：
// 左值已能定结果（&& 左 false / || 左 true）时直接返回，右子树不会被求值。
func applyBinary(st binaryStep, left Value, env *Env) (Value, error) {
	if st.op == OpAnd || st.op == OpOr {
		if !left.IsBool() {
			return Value{}, errAtPos(st.at, KindType, "运算符 %s 需要 bool，得到 %s", st.op, left.TypeName())
		}
		lb := left.AsBool()
		if st.op == OpAnd && !lb {
			return BoolValue(false), nil
		}
		if st.op == OpOr && lb {
			return BoolValue(true), nil
		}
	}

	right, err := st.right.Eval(env)
	if err != nil {
		return Value{}, err
	}

	switch st.op {
	case OpPlus:
		if left.IsInt() && right.IsInt() {
			return IntValue(left.AsInt() + right.AsInt()), nil
		}
		if left.IsString() && right.IsString() {
			return StringValue(left.AsString() + right.AsString()), nil
		}
		return Value{}, errAtPos(st.at, KindType, "运算符 + 需要 int 或 string，得到 %s 和 %s",
			left.TypeName(), right.TypeName())

	case OpMinus, OpStar, OpSlash, OpPercent:
		// typecheck 已保证两边是 int；这里再挡一次，兜住绕过类型检查直接求值的调用方。
		if !left.IsInt() || !right.IsInt() {
			return Value{}, errAtPos(st.at, KindType, "运算符 %s 需要整数，得到 %s 和 %s",
				st.op, left.TypeName(), right.TypeName())
		}

		l, r := left.AsInt(), right.AsInt()
		var res int64

		switch st.op {
		case OpMinus:
			res = l - r
		case OpStar:
			res = l * r
		case OpSlash:
			if r == 0 {
				return Value{}, errAtPos(st.at, KindZeroDivisor, "除数为零")
			}
			res = l / r
		case OpPercent:
			if r == 0 {
				return Value{}, errAtPos(st.at, KindZeroDivisor, "余数为零")
			}
			res = l % r
		}
		return IntValue(res), nil

	case OpEq, OpNeq:
		if left.kind != right.kind {
			return Value{}, errAtPos(st.at, KindType, "%s 两侧类型不同：%s 和 %s",
				st.op, left.TypeName(), right.TypeName())
		}
		eq := left.Equal(right)
		if st.op == OpNeq {
			eq = !eq
		}
		return BoolValue(eq), nil

	case OpLt, OpLe, OpGt, OpGe:
		if !left.IsInt() || !right.IsInt() {
			return Value{}, errAtPos(st.at, KindType, "运算符 %s 需要整数，得到 %s 和 %s",
				st.op, left.TypeName(), right.TypeName())
		}
		l, r := left.AsInt(), right.AsInt()
		var res bool
		switch st.op {
		case OpLt:
			res = l < r
		case OpLe:
			res = l <= r
		case OpGt:
			res = l > r
		case OpGe:
			res = l >= r
		}
		return BoolValue(res), nil

	case OpAnd, OpOr:
		// 走到这里说明左边"放行"了（&& 左 true / || 左 false），结果就是右边。
		if !right.IsBool() {
			return Value{}, errAtPos(st.at, KindType, "运算符 %s 需要 bool，得到 %s",
				st.op, right.TypeName())
		}
		return BoolValue(right.AsBool()), nil

	default:
		return Value{}, errAtPos(st.at, KindInternal, "未知运算符 %d", st.op)
	}
}

// Eval 是一元运算的求值：一元负号和一元 !。
func (u Unary) Eval(env *Env) (Value, error) {
	x, err := u.X.Eval(env)
	if err != nil {
		return Value{}, err
	}
	switch u.Op {
	case OpMinus:
		// 与 Binary 一致：typecheck 已保证 x 是 int，这里再挡一次直接求值的调用方。
		if !x.IsInt() {
			return Value{}, errAtPos(u.At, KindType, "一元负号需要整数，得到 %s", x.TypeName())
		}
		return IntValue(-x.AsInt()), nil

	case OpNot:
		if !x.IsBool() {
			return Value{}, errAtPos(u.At, KindType, "一元 ! 需要 bool，得到 %s", x.TypeName())
		}
		return BoolValue(!x.AsBool()), nil
	default:
		return Value{}, errAtPos(u.At, KindInternal, "未知一元运算符 %d", u.Op)
	}
}

// ---------- 语句 ----------

// Exec 是表达式语句的执行：求值后把值放进 Result，供 REPL / run 展示；
// 语句本身不保留任何状态（树仍可重复读）。
func (s ExprStmt) Exec(env *Env) (Result, error) {
	v, err := s.X.Eval(env)
	if err != nil {
		return Result{}, err
	}
	return Result{Value: v, HasValue: true}, nil
}

// Exec 是 let 的执行：先算右侧，再写入环境，Result 带上所赋的值——
// 这样 "语句[0] 返回值 = ..." 有信息量，比固定返回"无值"更贴近 REPL 直觉。
func (l Let) Exec(env *Env) (Result, error) {
	v, err := l.Value.Eval(env)
	if err != nil {
		return Result{}, err
	}
	env.Set(l.Name, v)
	return Result{Value: v, HasValue: true}, nil
}
