package main

// 运算符词汇表：Op 枚举 + 名字表 + 二元运算表（binOps）。parser 做 token → Op，
// ast 把 Op 存进 Binary / Unary，eval 按 Op 分派——没有哪一层独占它，所以单独
// 成文件。

// Op 是运算符的标识，二元和一元共用（一元负号复用 OpMinus，一元 ! 用 OpNot）。
// 它故意比 TokenType 窄：Binary / Unary 的 Op 只可能是表里这几种运算，不可能
// 出现 NUMBER、EOF 之类。token → Op 的转换放在 parser 里做——只有那里知道当前
// token 处于"运算符"位置。
type Op uint8

const (
	OpPlus Op = iota
	OpMinus
	OpStar
	OpSlash
	OpPercent

	OpEq
	OpNeq
	OpLt
	OpLe
	OpGt
	OpGe
	OpAnd
	OpOr
	OpNot // 一元，不进 binOps

	// 不是运算符，只用来表示总数；新增运算符必须插在它前面。
	opCount
)

// opNames 与上面的常量一一对应，供 Op.String 使用。沿用 token 名而不是符号，
// 这样 S-表达式里的名字和词法层看到的名字一致，对照着读最直接。
var opNames = [...]string{
	"PLUS", "MINUS", "STAR", "SLASH", "PERCENT",
	"EQ", "NEQ", "LT", "LE", "GT", "GE", "AND", "OR", "NOT",
}

// 两个方向的负长度断言：少名字或多名字都会让其中一个数组长度为负，编译失败
// （与 token.go 里 tokenNames 同款）。
var _ [int(opCount) - len(opNames)]struct{}
var _ [len(opNames) - int(opCount)]struct{}

func (o Op) String() string {
	if o < opCount {
		return opNames[o]
	}
	return "?"
}

// ---------- 二元运算表 ----------

// binOp 声明一个二元运算符：绑定力 + 对应的 Op。放这里而不是 parser.go：
// 优先级是运算符自己的属性，跟 token、名字一起放，parser 只照着爬。
type binOp struct {
	prec int // 绑定力：数值越大结合越紧
	op   Op
}

// isBinary 报告这个表项是不是二元运算符。非运算符的 token 在表里落在零值
// （prec 0），所以它同时是"表项是否有效"的判断，和优先级判断分开说。
func (b binOp) isBinary() bool { return b.prec != 0 }

// binOps 是全部二元运算符，按 TokenType 直接索引：TokenType 是密集小枚举
// （0..tokenTypeCount），数组就是一次下标寻址，也省掉 map 的两个返回值。
// 一元运算不进这张表（目前有负号和 !，由 parsePrimary 单独处理）。
// 加一个二元运算符要动这几处：token.go 的 TokenType 与名字表、lexer.go 的
// oneCharPuncts / twoCharPuncts（新符号才需要）、这张表、上面的 Op 常量
// （连带 opNames，双向断言会提醒）、typecheck.go 的 typeCheck 分支、
// eval.go 的 case 和 demo_test.go 的 opSamples；解析器一行不用改。
//
// 零值 {prec: 0} 表示"这个 token 不是二元运算符"，用 isBinary 判断；
// 真实运算符的 prec 一律 >= 1，所以 0 不会和合法值撞。
var binOps = [tokenTypeCount]binOp{
	OROR:    {prec: 1, op: OpOr},
	ANDAND:  {prec: 2, op: OpAnd},
	EQ:      {prec: 3, op: OpEq},
	NEQ:     {prec: 3, op: OpNeq},
	LT:      {prec: 3, op: OpLt},
	LE:      {prec: 3, op: OpLe},
	GT:      {prec: 3, op: OpGt},
	GE:      {prec: 3, op: OpGe},
	PLUS:    {prec: 4, op: OpPlus},
	MINUS:   {prec: 4, op: OpMinus},
	STAR:    {prec: 5, op: OpStar},
	SLASH:   {prec: 5, op: OpSlash},
	PERCENT: {prec: 5, op: OpPercent},
}

// minPrecLevel 是爬升的入口门槛，也是括号里的重置值：必须不高于表里任何
// 运算符的 prec，否则那个运算符永远解析不到（TestBinOpsAreFullyWired 钉死这条）。
// 取 0 就够："不是二元运算符"由 isBinary 单独判断，不靠这个数字。
const minPrecLevel = 0
