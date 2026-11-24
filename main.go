// game24 — 24 点计算器
//
// 用法:
//
//	game24 3 3 8 8          直接计算并列出所有解法
//	game24 -t 36 3 4 5 6    指定目标值 (默认 24)
//	game24                  进入交互模式, 反复输入数字
//
// 支持整数、小数、分数 (如 2.5、1/3)。内部使用精确有理数运算,
// 不会因浮点误差漏解 (例如 3 3 8 8 → 8/(3-8/3))。
// 输入不超过 5 个数字时穷举所有可能, 列出全部不重复解法;
// 6~8 个数字时自动切换为快速模式 (可能省略部分等价写法)。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
)

// 表达式树节点; op 为空表示叶子 (输入的数字)
type exprNode struct {
	op   string
	l, r *exprNode
	lit  string
	prec int // 1=加减, 2=乘除, 3=叶子
}

// 参与运算的一个值: 数值 + 对应的表达式
type val struct {
	num  *big.Rat
	e    *exprNode
	prec int
}

const (
	precAdd  = 1
	precMul  = 2
	precAtom = 3
	maxNums  = 8 // 数字个数上限
	fullMax  = 5 // 不超过该个数时穷举全部解法
)

var one = big.NewRat(1, 1)

var (
	target   *big.Rat
	usePrune bool            // 快速模式: 状态剪枝, 换速度
	seen     map[string]bool // 快速模式: 已展开的数值多重集
	found    map[string]bool // 结果去重 (规范化表达式键)
	results  []string
)

func flatten(e *exprNode, op string) []*exprNode {
	if e.op != op {
		return []*exprNode{e}
	}
	return append(flatten(e.l, op), flatten(e.r, op)...)
}

// canon 生成表达式的规范化键: 可交换的加/乘链拍平后排序,
// 只差交换律、结合律顺序的表达式共用同一个键, 用于结果去重。
func canon(e *exprNode) string {
	if e.op == "" {
		return e.lit
	}
	if e.op == "+" || e.op == "*" {
		parts := flatten(e, e.op)
		ss := make([]string, len(parts))
		for i, p := range parts {
			ss[i] = canon(p)
		}
		sort.Strings(ss)
		return strings.Join(ss, e.op)
	}
	return canon(e.l) + e.op + canon(e.r)
}

// canonTree 把表达式树转成规范化形态: 加/乘链拍平、按规范化键排序
// 后左折叠, 使同一解法总是渲染成同一个固定写法。
func canonTree(e *exprNode) *exprNode {
	if e.op == "" {
		return e
	}
	if e.op == "+" || e.op == "*" {
		parts := flatten(e, e.op)
		for i, p := range parts {
			parts[i] = canonTree(p)
		}
		ss := make([]string, len(parts))
		for i, p := range parts {
			ss[i] = canon(p)
		}
		// 按规范化键插入排序 (同步移动 ss 与 parts)
		for i := 1; i < len(parts); i++ {
			for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
				ss[j], ss[j-1] = ss[j-1], ss[j]
				parts[j], parts[j-1] = parts[j-1], parts[j]
			}
		}
		t := parts[0]
		for _, p := range parts[1:] {
			t = &exprNode{op: e.op, l: t, r: p, prec: e.prec}
		}
		return t
	}
	return &exprNode{op: e.op, l: canonTree(e.l), r: canonTree(e.r), prec: e.prec}
}

// render 按运算优先级输出表达式, 尽量少加括号
func render(e *exprNode) string {
	if e.op == "" {
		return e.lit
	}
	ls, rs := render(e.l), render(e.r)
	if e.l.prec < e.prec {
		ls = "(" + ls + ")"
	}
	// 右侧与当前运算同级时, 减法和除法不满足结合律, 必须加括号
	if e.r.prec < e.prec || (e.r.prec == e.prec && (e.op == "-" || e.op == "/")) {
		rs = "(" + rs + ")"
	}
	return ls + e.op + rs
}

// stateKey 当前数值多重集的规范化键 (快速模式剪枝用)
func stateKey(vals []val) string {
	ss := make([]string, len(vals))
	for i, v := range vals {
		ss[i] = v.num.RatString()
	}
	sort.Strings(ss)
	return strings.Join(ss, ",")
}

func solve(vals []val) {
	if usePrune {
		key := stateKey(vals)
		if seen[key] {
			return
		}
		seen[key] = true
	}

	if len(vals) == 1 {
		if vals[0].num.Cmp(target) == 0 {
			key := canon(vals[0].e)
			if !found[key] {
				found[key] = true
				results = append(results, render(canonTree(vals[0].e)))
			}
		}
		return
	}

	// 按数值排序: 值相同的数字完全可互换, 数值对可以安全去重
	sort.Slice(vals, func(i, j int) bool { return vals[i].num.Cmp(vals[j].num) < 0 })

	n := len(vals)
	for i := 0; i < n; i++ {
		if i > 0 && vals[i].num.Cmp(vals[i-1].num) == 0 {
			continue // 与前一个等值, 作为左操作数是重复路径
		}
		for j := i + 1; j < n; j++ {
			if j > i+1 && vals[j].num.Cmp(vals[j-1].num) == 0 {
				continue // 该数值组合已经用过等值的数字尝试过
			}
			a, b := vals[i], vals[j]

			rest := make([]val, 0, n-2)
			for k := 0; k < n; k++ {
				if k != i && k != j {
					rest = append(rest, vals[k])
				}
			}

			add := func(x, y val, op string, prec int, r *big.Rat) {
				out := val{
					num:  r,
					e:    &exprNode{op: op, l: x.e, r: y.e, prec: prec},
					prec: prec,
				}
				// 恒等运算化简: ×1、/1、+0、-0、×0 直接约掉,
				// 数值不变, 但输出更干净、去重更彻底
				switch op {
				case "+":
					if x.num.Sign() == 0 {
						out = y
					} else if y.num.Sign() == 0 {
						out = x
					}
				case "-":
					if y.num.Sign() == 0 {
						out = x
					}
				case "*":
					if x.num.Sign() == 0 {
						out = x
					} else if y.num.Sign() == 0 {
						out = y
					} else if x.num.Cmp(one) == 0 {
						out = y
					} else if y.num.Cmp(one) == 0 {
						out = x
					}
				case "/":
					if y.num.Cmp(one) == 0 {
						out = x
					} else if x.num.Sign() == 0 {
						out = x
					}
				}
				next := make([]val, 0, n-1)
				next = append(next, rest...)
				next = append(next, out)
				solve(next)
			}

			// 加、乘可交换只算一种顺序; 两数相等时减、除也只算一种
			eq := a.num.Cmp(b.num) == 0
			add(a, b, "+", precAdd, new(big.Rat).Add(a.num, b.num))
			add(a, b, "*", precMul, new(big.Rat).Mul(a.num, b.num))
			add(a, b, "-", precAdd, new(big.Rat).Sub(a.num, b.num))
			if !eq {
				add(b, a, "-", precAdd, new(big.Rat).Sub(b.num, a.num))
			}
			if b.num.Sign() != 0 {
				add(a, b, "/", precMul, new(big.Rat).Quo(a.num, b.num))
			}
			if a.num.Sign() != 0 && !eq {
				add(b, a, "/", precMul, new(big.Rat).Quo(b.num, a.num))
			}
		}
	}
}

// run 求解并返回去重、排序后的表达式列表
func run(vals []val) []string {
	usePrune = len(vals) > fullMax
	seen = make(map[string]bool)
	found = make(map[string]bool)
	results = nil
	solve(vals)
	sort.Strings(results)
	return results
}

func parseNums(fields []string) ([]val, error) {
	vals := make([]val, 0, len(fields))
	for _, f := range fields {
		if len(f) > 20 {
			return nil, fmt.Errorf("数字太长, 无法识别: %q", f)
		}
		r, ok := new(big.Rat).SetString(f)
		if !ok {
			return nil, fmt.Errorf("无法识别的数字: %q (支持整数/小数/分数, 例如 3、2.5、1/3)", f)
		}
		lit := f
		if r.Sign() < 0 {
			lit = "(" + f + ")"
		}
		vals = append(vals, val{
			num:  r,
			e:    &exprNode{lit: lit, prec: precAtom},
			prec: precAtom,
		})
	}
	return vals, nil
}

func doRun(fields []string) {
	if len(fields) < 1 || len(fields) > maxNums {
		fmt.Printf("请输入 1~%d 个数字 (空格分隔)\n", maxNums)
		return
	}
	vals, err := parseNums(fields)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(vals) > fullMax {
		fmt.Printf("(输入了 %d 个数字, 已启用快速模式: 可能省略部分等价写法)\n", len(vals))
	}
	sols := run(vals)
	if len(sols) == 0 {
		fmt.Println("无解")
		return
	}
	fmt.Printf("共 %d 种解法:\n", len(sols))
	for _, s := range sols {
		fmt.Println("  " + s)
	}
}

func main() {
	t := flag.String("t", "24", "目标值 (支持整数/小数/分数)")
	flag.Parse()

	tr, ok := new(big.Rat).SetString(*t)
	if !ok {
		fmt.Fprintln(os.Stderr, "无效的目标值:", *t)
		os.Exit(1)
	}
	target = tr

	// 命令行带了数字: 直接计算一次然后退出
	if args := flag.Args(); len(args) > 0 {
		doRun(args)
		return
	}

	// 否则进入交互模式
	sc := bufio.NewScanner(os.Stdin)
	fmt.Println("24 点计算器 — 输入 1~8 个数字 (空格分隔), 例如: 3 3 8 8  (输入 q 退出)")
	fmt.Printf("当前目标值: %s\n", target.RatString())
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			break // EOF
		}
		line := strings.TrimSpace(sc.Text())
		line = strings.ReplaceAll(line, string(rune(0xFEFF)), "") // 去掉管道输入可能带的 UTF-8 BOM
		if line == "" {
			continue
		}
		if line == "q" || line == "quit" || line == "exit" {
			break
		}
		doRun(strings.Fields(line))
	}
}
