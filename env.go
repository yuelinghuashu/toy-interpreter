package main

// 第 3 层：求值环境。Env 保存变量绑定；Eval 从这里读，Let 往这里写。
// 绑定值是 Value（int / string / bool），与 Expr.Eval 的返回值一致，链路上不需要转换。

// Env 是变量环境。按值传递指针，调用方共享同一份绑定。
type Env struct {
	vars map[string]Value
}

// NewEnv 返回一个空环境。
func NewEnv() *Env {
	return &Env{vars: make(map[string]Value)}
}

// Get 读取变量；ok=false 表示未定义。
func (e *Env) Get(name string) (Value, bool) {
	v, ok := e.vars[name]
	return v, ok
}

// Set 写入变量。允许覆盖——重复 let 不报错，和 REPL 直觉一致；
// 想改成"重复定义报错"要同时改这里和 Let.typeCheck（类型层也允许覆盖），
// 并同步更新测试与 examples/04-let-bindings.toy 里的重绑定示例。
func (e *Env) Set(name string, v Value) {
	e.vars[name] = v
}
