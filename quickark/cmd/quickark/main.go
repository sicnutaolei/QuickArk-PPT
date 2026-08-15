// Command quickark 把课件 PPT 按章节归类归档到文件夹。
//
// 用法：
//
//	quickark [目录] [--yes]            # 整理指定目录（缺省目录时交互式输入）
//	quickark undo <csv路径>            # 撤销，按 CSV 映射还原
//
// 默认先打印预览，确认后才真正移动文件；加 --yes 可跳过确认。
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"quickark"
)

func main() {
	args := os.Args[1:]

	// undo 子命令
	if len(args) > 0 && args[0] == "undo" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法: quickark undo <csv路径>")
			os.Exit(1)
		}
		if err := quickark.Undo(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "撤销失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("已撤销，文件已还原。")
		return
	}

	// 解析 flags 与位置参数（目录可出现在任意位置）
	yes := false
	var dir string
	for _, a := range args {
		switch {
		case a == "--yes" || a == "-y":
			yes = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "未知参数: %s\n", a)
			os.Exit(1)
		case dir == "":
			dir = a
		}
	}

	if dir == "" {
		fmt.Print("请输入要整理的目录（直接回车退出）: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		dir = strings.TrimSpace(line)
		if dir == "" {
			fmt.Println("未输入目录，已退出。")
			return
		}
	}

	// 兼容 Git-Bash 的 /c/... 形式
	dir = normalizePath(dir)

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "目录不存在或不是文件夹: %s\n", dir)
		os.Exit(1)
	}

	plan, err := quickark.ScanDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "扫描失败: %v\n", err)
		os.Exit(1)
	}

	printPreview(plan)

	moveCount := 0
	skipCount := 0
	for _, op := range plan.Ops {
		if op.Skip {
			skipCount++
		} else {
			moveCount++
		}
	}
	if moveCount == 0 {
		fmt.Println("\n没有需要归类的文件。")
		return
	}

	if !yes {
		fmt.Printf("\n将移动 %d 个文件，跳过 %d 个。确认执行？(y/N): ", moveCount, skipCount)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(line), "y") {
			fmt.Println("已取消，未做任何改动。")
			return
		}
	}

	if err := quickark.Apply(plan, false); err != nil {
		fmt.Fprintf(os.Stderr, "执行失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n完成：移动 %d 个文件，跳过 %d 个。\n", moveCount, skipCount)
	if plan.CSVPath != "" {
		fmt.Printf("映射文件: %s\n", plan.CSVPath)
		fmt.Printf("如需撤销: quickark undo %s\n", plan.CSVPath)
	}
	printTree(plan)
}

func printPreview(plan *quickark.Plan) {
	fmt.Printf("目录: %s\n", plan.Dir)
	fmt.Printf("（章号补零到 %d 位，节号补零到 %d 位）\n\n", plan.ChapterWidth, plan.SectionWidth)
	fmt.Printf("%-14s %-16s %s\n", "章", "节", "文件")
	fmt.Println(strings.Repeat("-", 60))
	for _, op := range plan.Ops {
		if op.Skip {
			fmt.Printf("%-14s %-16s %s  [SKIP: %s]\n", "-", "-", filepath.Base(op.Src), op.SkipReason)
			continue
		}
		fmt.Printf("%-14s %-16s %s\n", op.ChapterDir, orDash(op.SectionDir), filepath.Base(op.Src))
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func printTree(plan *quickark.Plan) {
	chapters := map[string]map[string]bool{}
	order := []string{}
	for _, op := range plan.Ops {
		if op.Skip {
			continue
		}
		if _, ok := chapters[op.ChapterDir]; !ok {
			chapters[op.ChapterDir] = map[string]bool{}
			order = append(order, op.ChapterDir)
		}
		if op.SectionDir != "" {
			chapters[op.ChapterDir][op.SectionDir] = true
		}
	}
	if len(order) == 0 {
		return
	}
	fmt.Println("\n生成的目录结构：")
	for _, ch := range order {
		fmt.Printf("  %s/\n", ch)
		for sec := range chapters[ch] {
			fmt.Printf("    %s/\n", sec)
		}
	}
}

// normalizePath 把 Git-Bash 的 /c/... 形式转为 Windows 路径。
func normalizePath(p string) string {
	if strings.HasPrefix(p, "/") && len(p) >= 3 && p[2] == '/' {
		drive := strings.ToUpper(p[1:2])
		return drive + ":" + p[2:]
	}
	return p
}
