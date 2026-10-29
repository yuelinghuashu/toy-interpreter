package main

import "fmt"

// 第 1 层：词法。Token 的类型与结构在这里；怎么把它们从源码里扫出来见 lexer.go。

// TokenType 是 token 的类型。
type TokenType int

const (
	ILLEGAL             TokenType = iota // 非法字符
	EOF                                  // 文件结束
	NUMBER                               // 整数
	STRING                               // 字符串
	UNTERMINATED_STRING                  // 未闭合的字符串
	BOOLEAN                              // 布尔值
	PLUS                                 // 加号
	MINUS                                // 减号
	STAR                                 // 乘号
	SLASH                                // 除号
	PERCENT                              // 取模
	LPAREN                               // 左括号
	RPAREN                               // 右括号
	IDENTIFIER                           // 变量名
	ASSIGN                               // 赋值号
	LET                                  // let 关键字
	EQ                                   // ==
	NEQ                                  // !=
	LT                                   // <
	LE                                   // <=
	GT                                   // >
	GE                                   // >=
	ANDAND                               // &&
	OROR                                 // ||
	BANG                                 // !
	NEWLINE                              // 换行符

	// 不是 token，只用来表示总数；新增 token 必须插在它前面。
	tokenTypeCount
)

// tokenNames 与上面的常量一一对应：顺序必须一致，长度由下面的断言保证。
var tokenNames = [...]string{
	ILLEGAL:             "ILLEGAL",
	EOF:                 "EOF",
	NUMBER:              "NUMBER",
	STRING:              "STRING",
	UNTERMINATED_STRING: "UNTERMINATED_STRING",
	BOOLEAN:             "BOOLEAN",
	PLUS:                "PLUS",
	MINUS:               "MINUS",
	STAR:                "STAR",
	SLASH:               "SLASH",
	PERCENT:             "PERCENT",
	LPAREN:              "LPAREN",
	RPAREN:              "RPAREN",
	IDENTIFIER:          "IDENTIFIER",
	ASSIGN:              "ASSIGN",
	LET:                 "LET",
	EQ:                  "EQ",
	NEQ:                 "NEQ",
	LT:                  "LT",
	LE:                  "LE",
	GT:                  "GT",
	GE:                  "GE",
	ANDAND:              "ANDAND",
	OROR:                "OROR",
	BANG:                "BANG",
	NEWLINE:             "NEWLINE",
}

// 两个方向的负长度断言：少名字或多名字都会让其中一个数组长度为负，编译失败。
// 顺序错位不在断言覆盖内（长度相等），由「全部单字符符号」用例兜底。
var _ [int(tokenTypeCount) - len(tokenNames)]struct{}
var _ [len(tokenNames) - int(tokenTypeCount)]struct{}

// init 复核 tokenNames 里没有空名字：长度已由上面的双向断言在编译期钉死，
// 这里只兜住"某一项被写成空串"——否则 String() 会静默返回空串。
// 顺序由键控字面量（ILLEGAL: ...）的下标固定，不需要运行时检查。
func init() {
	for i := range tokenTypeCount {
		if tokenNames[i] == "" {
			panic(fmt.Sprintf("tokenNames[%d] (token %d) 未定义", int(i), int(i)))
		}
	}
}

func (t TokenType) String() string {
	if int(t) >= 0 && int(t) < int(tokenTypeCount) {
		return tokenNames[int(t)]
	}
	return "UNKNOWN"
}

// Token 是一次扫描的产物。Literal 是源码原文片段，不做语义加工；例外是
// NEWLINE：\n、\r\n、孤立 \r 统一成 "\n"。
type Token struct {
	Type    TokenType // token 类型
	Literal string    // 原始文本
	Line    int       // 从 1 开始
	Column  int       // 从 1 开始
}
