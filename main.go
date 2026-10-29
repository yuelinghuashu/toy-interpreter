// 最小完整案例：从零写一个支持整数 / 字符串、let 变量、+ - * / %、
// 优先级与括号、按换行分句的三层解释器（词法 → 语法 → 求值），并在求值前
// 插入一层静态类型检查。
//
//	token.go     第 1 层：TokenType / Token 的定义与名字表
//	lexer.go     第 1 层：Lexer 逐字节扫描源码
//	node.go      第 2 层：AST 节点（字面量 / 标识符 / 一元 / 二元 / let）与 S-表达式渲染
//	ast.go       第 2 层：Node 接口、Position、SExpr 入口
//	operator.go  第 2/3 层：Op 运算符词汇表（token → Op 在 parser，分派在 eval）
//	parser.go    第 2 层：优先级爬升解析器 + 字符串反转义
//	typecheck.go 第 2.5 层：静态类型检查（Type / TypeEnv）
//	env.go       第 3 层：运行时变量环境
//	value.go     第 3 层：Value（int / string）与格式化
//	eval.go      第 3 层：各节点的 Eval
//	repl.go      交互模式：按行解析求值，支持 :help / :env / :quit
//	main.go      本文件：读源码文件，把各层串起来跑一遍
//
// 支持：十进制 / 0x / 0b / 0o 整数、字符串（+ 拼接）、let 变量、+ - * / %、
// 一元负号、括号与优先级、换行分隔语句，以及求值前的静态类型检查；没有错误
// 恢复和溢出检查——是教学骨架，不是生产实现。
// 运行：go run .（默认读同目录的 input.toy）；go run . 文件名 可换成别的源码；
//
//	go run . -repl 进入交互模式。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	replMode := flag.Bool("repl", false, "是否进入 REPL 模式")
	flag.Parse()

	if *replMode {
		if err := repl(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	path := "input.toy"
	if args := flag.Args(); len(args) > 0 {
		path = args[0]
	}
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取 %s 失败：%v（请在 demo 目录下运行，或传入文件路径）\n", path, err)
		os.Exit(1)
	}
	// 退出码只在 main 里决定（os.Exit 会掐掉 defer），run 只返回错误。
	if err := run(string(src), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 把各层串起来跑一遍（词法 → 语法 → 类型检查 → 求值），所有输出写到 out，
// 失败原因原样返回给调用方。
// 这里不碰 os.Exit：它可被测试（也能把 out 换成 buffer 做 golden），
// 出错返回时 deferred 的清理也不会被掐掉。
func run(src string, out io.Writer) error {
	// ---------- 第 1 层：词法 ----------
	fmt.Fprintln(out, "== 1. 词法 ==")
	lexer := NewLexer(src)
	for {
		tok := lexer.NextToken()
		fmt.Fprintf(out, "%-2d:%-4d %-12s %s\n",
			tok.Line, tok.Column, tok.Type, escapeTokenLiteral(tok.Literal))
		if tok.Type == EOF {
			break
		}
	}

	// ---------- 第 2 层：语法 ----------
	fmt.Fprintln(out, "== 2. 语法（AST）==")
	parser := NewParser(NewLexer(src))
	stmts, err := parser.ParseProgram()
	if err != nil {
		return fmt.Errorf("解析失败：%w", err)
	}
	for i, s := range stmts {
		fmt.Fprintf(out, "  语句[%d] = %s\n", i, SExpr(s))
	}

	// ---------- 第 2.5 层：静态类型检查 ----------
	fmt.Fprintln(out, "== 2.5 类型检查 ==")
	if err := CheckProgram(stmts); err != nil {
		return fmt.Errorf("类型检查失败：%w", err)
	}
	fmt.Fprintln(out, "  (通过)")

	// ---------- 第 3 层：求值 ----------
	fmt.Fprintln(out, "== 3. 求值 ==")
	if len(stmts) == 0 {
		fmt.Fprintln(out, "  (空程序，没有语句)")
		return nil
	}

	vals := make([]Value, len(stmts))
	env := NewEnv()
	for i, s := range stmts {
		v, err := s.Eval(env)
		if err != nil {
			return fmt.Errorf("求值失败：%w", err)
		}
		vals[i] = v
		fmt.Fprintf(out, "  语句[%d] 返回值 = %s\n", i, v)
	}

	// ---------- 最终结果 ----------
	fmt.Fprintln(out, "== 4. 最终结果 ==")
	// 复用第 3 节的结果：每条语句只求值一次，这一节只是把 AST 和值拼起来看。
	for i, s := range stmts {
		fmt.Fprintf(out, "  语句[%d] %s = %s\n", i, SExpr(s), vals[i])
	}
	return nil
}

// escapeTokenLiteral 把 token 字面量里的控制字符显示成可见的两字符转义：
// NEWLINE 的 Literal 是真换行符，直接 %s 会多出一行空行、打乱输出对齐；
// 字符串里若含真 \t，同理会把列对齐撑歪。
func escapeTokenLiteral(lit string) string {
	lit = strings.ReplaceAll(lit, "\n", `\n`)
	lit = strings.ReplaceAll(lit, "\t", `\t`)
	return strings.ReplaceAll(lit, "\r", `\r`)
}
