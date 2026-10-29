package main

import "strconv"

type ValueKind int

const (
	KindInvalid ValueKind = iota
	KindInt
	KindString
	KindBool
)

// Value 是运行时值。零值 Value{} 是无效值（KindInvalid）——求值出错时统一返回它，
// 调用方必须先看 error 再用 IsInt / IsString / IsBool 判断。
type Value struct {
	kind ValueKind
	i    int64
	s    string
	b    bool
}

// ---------- 构造 ----------

// IntValue 把 int64 包成 Value；值是 int。
func IntValue(i int64) Value { return Value{kind: KindInt, i: i} }

// StringValue 把 string 包成 Value；值是 string。
func StringValue(s string) Value { return Value{kind: KindString, s: s} }

// BoolValue 把 bool 包成 Value；值是 bool。
func BoolValue(b bool) Value { return Value{kind: KindBool, b: b} }

// ---------- 判断 ----------

func (v Value) IsInt() bool    { return v.kind == KindInt }
func (v Value) IsString() bool { return v.kind == KindString }
func (v Value) IsBool() bool   { return v.kind == KindBool }

// ---------- 取值 ----------

// AsInt 取出 int 值；调用方应先确认 IsInt。
func (v Value) AsInt() int64 { return v.i }

// AsString 取出 string 值；调用方应先确认 IsString。
func (v Value) AsString() string { return v.s }

// AsBool 取出 bool 值；调用方应先确认 IsBool。
func (v Value) AsBool() bool { return v.b }

// ---------- 格式化 ----------

// String 实现 fmt.Stringer：int 十进制，string 用 strconv.Quote（带引号、转义）。
// 和 SExpr 里 StringLiteral.writeTo 的写法保持一致，输出能互相对照。
func (v Value) String() string {
	switch v.kind {
	case KindInt:
		return strconv.FormatInt(v.i, 10)
	case KindString:
		return strconv.Quote(v.s)
	case KindBool:
		return strconv.FormatBool(v.b)
	}
	return "?"
}

// TypeName 返回语言层的类型名，求值兜底报错用（"得到 string" 之类）。
func (v Value) TypeName() string {
	switch v.kind {
	case KindInt:
		return "int"
	case KindString:
		return "string"
	case KindBool:
		return "bool"
	}
	return "?"
}

// Equal 报告两个值是否等价。当前表示下就是结构相等（kind / i / s / b 都可比较）；
// 以后若加入 float64、NaN 或"不同表示同语义"的类型，只改这里，
// 测试与 REPL 等调用方不用跟着改。
func (v Value) Equal(w Value) bool { return v == w }
