package main

import (
	"strings"
	"testing"
)

func TestBinOpTable(t *testing.T) {
	seen := make(map[Op]TokenType, len(binOps))
	for i, info := range binOps {
		if !info.isBinary() {
			continue
		}
		tok := TokenType(i)
		if info.prec < 1 {
			t.Errorf("%s 的绑定力 = %d：必须 >= 1", tok, info.prec)
		}
		if info.op >= opCount {
			t.Errorf("%s 的 Op = %d，超出 opCount(%d)", tok, info.op, opCount)
		}
		if prev, ok := seen[info.op]; ok {
			t.Errorf("Op %s 同时挂在 %s 与 %s 上", info.op, prev, tok)
		}
		seen[info.op] = tok
	}

	var unaryOnlyOps = map[Op]bool{OpNot: true}
	for op := range opCount {
		if unaryOnlyOps[op] {
			continue
		}
		if _, ok := seen[op]; !ok {
			t.Errorf("Op %s 不在 binOps 里：解析器永远产不出它", op)
		}
	}
}

// opSamples 给每个二元 Op 一段完整源码：左右操作数类型满足该运算符的要求，
// 词法、解析、求值三件事一起验证。加新运算符时漏补样本，这里会红。
var opSamples = map[Op]string{
	OpPlus:    "1+2",
	OpMinus:   "1-2",
	OpStar:    "1*2",
	OpSlash:   "1/2",
	OpPercent: "1%2",
	OpEq:      "1==2",
	OpNeq:     "1!=2",
	OpLt:      "1<2",
	OpLe:      "1<=2",
	OpGt:      "1>2",
	OpGe:      "1>=2",
	OpAnd:     "true&&false",
	OpOr:      "true||false",
}

func TestBinOpsAreFullyWired(t *testing.T) {
	for i, info := range binOps {
		if !info.isBinary() {
			continue
		}
		_ = TokenType(i) // 数组下标与 token 对应，未来按 token 反查时用得到
		t.Run(info.op.String(), func(t *testing.T) {
			src, ok := opSamples[info.op]
			if !ok {
				t.Fatalf("Op %s 缺少 opSamples 样本：加运算符时忘了补测试", info.op)
			}
			full := src + "\n"
			stmts := parseProgram(t, full)
			if len(stmts) != 1 {
				t.Fatalf("%q 解析出 %d 条语句，期望 1 条", full, len(stmts))
			}
			stmt, ok := stmts[0].(ExprStmt)
			if !ok {
				t.Fatalf("%q 的语句是 %T，期望 ExprStmt", full, stmts[0])
			}
			bin, ok := stmt.X.(Binary)
			if !ok {
				t.Fatalf("%q 的表达式是 %T，期望 Binary", full, stmt.X)
			}
			if bin.Op != info.op {
				t.Fatalf("%q 解析出的 Op = %s，期望 %s", full, bin.Op, info.op)
			}
			if got, want := SExpr(stmts[0]), "("+info.op.String()+" "; !strings.HasPrefix(got, want) {
				t.Fatalf("%q 的 S-表达式 = %q，期望以 %q 开头", full, got, want)
			}
			if _, err := stmts[0].Exec(NewEnv()); err != nil {
				t.Fatalf("求值 %q 失败：%v（eval.go 的 case 没接通）", full, err)
			}
		})
	}
}
