// Command pptclean 是一个可复用的 PPT 课件文件名清洗工具。
//
// 用法：
//   pptclean [dir] [--units 章,讲,节] [--width N] [--yes]
//   pptclean undo <csv>
//
// 不带 dir 时会交互式提示输入目录；默认先打印预览，确认后才真正改名。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"pptclean"
)

func main() {
	// undo 子命令
	if len(os.Args) > 1 && os.Args[1] == "undo" {
		if len(os.Args) < 3 {
			fmt.Println("用法: pptclean undo <csv路径>")
			os.Exit(1)
		}
		if err := pptclean.Undo(os.Args[2]); err != nil {
			fmt.Println("undo 失败:", err)
			os.Exit(1)
		}
		fmt.Println("已按映射文件回滚完成。")
		return
	}

	fs := flag.NewFlagSet("pptclean", flag.ContinueOnError)
	unitsFlag := fs.String("units", "", "自定义识别单位，逗号分隔，如 章,讲,节")
	widthFlag := fs.Int("width", 0, "强制零填充位数（0=按目录自动，下限2）")
	yesFlag := fs.Bool("yes", false, "跳过确认，直接执行")

	// 分离目录参数（首个不以 '-' 开头的参数），其余交给 flag 解析，
	// 这样目录可以放在 flags 之前或之后。
	var dirArg string
	var rest []string
	for _, a := range os.Args[1:] {
		if dirArg == "" && !strings.HasPrefix(a, "-") {
			dirArg = a
		} else {
			rest = append(rest, a)
		}
	}
	_ = fs.Parse(rest)

	dir := dirArg
	if dir == "" {
		dir = fs.Arg(0)
	}
	if dir == "" {
		fmt.Print("请输入目标目录路径: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		dir = strings.TrimSpace(line)
	}
	if dir == "" {
		fmt.Println("未提供目录，已退出。")
		os.Exit(1)
	}

	var units []string
	if *unitsFlag != "" {
		for _, u := range strings.Split(*unitsFlag, ",") {
			if u = strings.TrimSpace(u); u != "" {
				units = append(units, u)
			}
		}
	}

	plans, err := pptclean.ScanDir(dir, pptclean.ScanOptions{Units: units, Width: *widthFlag})
	if err != nil {
		fmt.Println("扫描失败:", err)
		os.Exit(1)
	}

	fmt.Println("\n=== 预览（尚未修改任何文件）===")
	changed, skipped := 0, 0
	for _, p := range plans {
		switch {
		case p.Skipped:
			fmt.Printf("[SKIP]     %s\n", p.Old)
			skipped++
		case p.Conflict:
			fmt.Printf("[CONFLICT] %s  ->  %s\n", p.Old, p.New)
			changed++
		default:
			fmt.Printf("           %s  ->  %s\n", p.Old, p.New)
			changed++
		}
	}
	fmt.Printf("\n将改动 %d 个文件，跳过 %d 个。\n", changed, skipped)

	if !*yesFlag {
		fmt.Print("确认执行? (y/N): ")
		ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			fmt.Println("已取消，未修改任何文件。")
			return
		}
	}

	csvPath, err := pptclean.Apply(dir, plans)
	if err != nil {
		fmt.Println("执行出错:", err)
		os.Exit(1)
	}
	fmt.Printf("\n已完成。映射文件: %s\n", csvPath)

	// 自检：打印改名后按名称升序的最终清单
	var newNames []string
	for _, p := range plans {
		if p.Status == "renamed" {
			newNames = append(newNames, p.New)
		}
	}
	sort.Strings(newNames)
	fmt.Println("\n=== 改名后按名称升序（应已正确排序）===")
	for _, n := range newNames {
		fmt.Println(n)
	}
}
