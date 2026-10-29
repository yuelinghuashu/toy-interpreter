// 最小完整案例：从零写一个支持整数 / 字符串 / 布尔、let 变量、算术（+ - * / %）、
// 比较（== != < <= > >=）、逻辑（&& || !）、优先级与括号、按换行分句的三层
// 解释器（词法 → 语法 → 求值），并在求值前插入一层静态类型检查。
//
//	token.go     第 1 层：TokenType / Token 的定义与名字表
//	lexer.go     第 1 层：Lexer 逐字节扫描源码
//	node.go      第 2 层：AST 节点（表达式 + 语句）与 S-表达式渲染
//	ast.go       第 2 层：Node / Expr / Stmt 接口、Position、SExpr 入口
//	operator.go  第 2/3 层：Op 运算符词汇表（token → Op 在 parser，分派在 eval）
//	parser.go    第 2 层：优先级爬升解析器 + 字符串反转义
//	typecheck.go 第 2.5 层：静态类型检查（Expr 有类型、Stmt 只报错）
//	env.go       第 3 层：运行时变量环境
//	value.go     第 3 层：Value（int / string / bool）与格式化
//	eval.go      第 3 层：Expr 的 Eval、Stmt 的 Exec
//	repl.go      交互模式：按行解析求值，支持 :help / :env / :quit
//	main.go      本文件：读示例源码，把各层串起来跑一遍
//	examples/    按语法分主题的示例源码：01 注释与空白 … 08 综合
//
// 支持：十进制 / 0x / 0b / 0o 整数、字符串（+ 拼接）、true / false、let 变量、
// + - * / %、比较 == != < <= > >=、逻辑 && || 与一元 !、一元负号、括号与优先级、
// 换行分隔语句，以及求值前的静态类型检查；没有错误恢复和溢出检查——是教学骨架，
// 不是生产实现。
// 运行：go run .（默认依次跑 examples/ 下所有 .toy）；go run . 文件或目录 可只跑指定范围；
//
//	go run . -repl 进入交互模式。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// defaultExamplesDir 是不带参数运行时依次跑的示例目录。
const defaultExamplesDir = "examples"

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

	path := defaultExamplesDir
	if args := flag.Args(); len(args) > 0 {
		path = args[0]
	}

	programs, err := loadPrograms(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取 %s 失败：%v（默认读 examples/，也可传入 .toy 文件或目录）\n", path, err)
		os.Exit(1)
	}

	// 退出码只在 main 里决定（os.Exit 会掐掉 defer），run 只返回错误。
	out := os.Stdout
	for i, p := range programs {
		if i > 0 {
			fmt.Fprintln(out)
		}
		if len(programs) > 1 {
			fmt.Fprintf(out, "### %s\n", p.name)
		}
		if err := run(p.src, out); err != nil {
			fmt.Fprintf(os.Stderr, "%s：%v\n", p.name, err)
			os.Exit(1)
		}
	}
}

// program 是一份待跑的源码：Name 用于多文件输出的标题，Src 是文件内容。
type program struct {
	name string
	src  string
}

// loadPrograms 读取 path：是目录就取其中所有 .toy（按文件名排序，os.ReadDir 已排序），
// 是文件就只取它自己。一个文件都没读到时报错，而不是静默打印空输出。
func loadPrograms(path string) ([]program, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return []program{{name: path, src: string(src)}}, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var programs []program
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toy") {
			continue
		}
		file := filepath.Join(path, e.Name())
		src, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		programs = append(programs, program{name: file, src: string(src)})
	}
	if len(programs) == 0 {
		return nil, fmt.Errorf("%s 下没有 .toy 文件", path)
	}
	return programs, nil
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
		fmt.Fprintf(out, "%-2d:%-4d %-19s %s\n",
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

	vals := make([]Result, len(stmts))
	env := NewEnv()
	for i, s := range stmts {
		res, err := s.Exec(env)
		if err != nil {
			return fmt.Errorf("求值失败：%w", err)
		}
		vals[i] = res
		if res.HasValue {
			fmt.Fprintf(out, "  语句[%d] 返回值 = %s\n", i, res.Value)
		}
	}

	// ---------- 最终结果 ----------
	fmt.Fprintln(out, "== 4. 最终结果 ==")
	// 复用第 3 节的结果：每条语句只执行一次，这一节只是把 AST 和值拼起来看。
	// 没有值的语句（将来的 while / if）只打 AST。
	for i, s := range stmts {
		if vals[i].HasValue {
			fmt.Fprintf(out, "  语句[%d] %s = %s\n", i, SExpr(s), vals[i].Value)
		} else {
			fmt.Fprintf(out, "  语句[%d] %s\n", i, SExpr(s))
		}
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
