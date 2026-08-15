// Package quickark 把课件 PPT 按章节归类归档到文件夹结构。
//
// 它是 Python 版 QuickArk_PPT.py 的 Go 重构版：在保留原行为（按章节建目录、
// 把 PPT 移动进去）的基础上，做了两点改进——
//  1. 章节/节支持任意数字（阿拉伯或中文），不再局限于第一~第五章、第一~第五节；
//  2. 生成的文件夹名采用零填充阿拉伯数字（第09章、第31讲），
//     从而根治 Windows「按名称排序」把「第九章」排到「第八章」前面的问题。
package quickark

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 章节匹配：第<数字>章
var reChapter = regexp.MustCompile(`第([0-9零一二三四五六七八九十百千]+)章`)

// 节/讲匹配：第<数字>节 或 第<数字>讲
var reSection = regexp.MustCompile(`第([0-9零一二三四五六七八九十百千]+)(节|讲)`)

// specialSections 特殊节名（无数字），按出现顺序优先匹配。
var specialSections = []string{"整理与提升", "复习课", "提升课"}

// pptExts 处理扩展名（与 pptclean 保持一致：.ppt 与 .pptx 都管）。
var pptExts = map[string]bool{".pptx": true, ".ppt": true}

// Op 表示一次「移动文件到目标目录」的计划项。
type Op struct {
	Src         string // 文件原始绝对路径
	Dst         string // 计划移动到的绝对路径
	ChapterDir  string // 章目录名，如 第09章
	SectionDir  string // 节目录名（可能为空），如 第31讲 / 整理与提升
	TargetDir   string // 最终目标目录（章目录，或 章/节）
	Chapter     int    // 章号（阿拉伯）
	SectionNum  int    // 节号（阿拉伯，特殊节为 0）
	SectionUnit string // 节单位：节 / 讲（特殊节为空）
	Special     string // 特殊节名（无数字时为对应字符串，否则空）
	Skip        bool   // true 表示无法解析、跳过
	SkipReason  string
}

// Plan 一次完整的归类计划。
type Plan struct {
	Dir          string // 被整理的目录
	Ops          []Op
	ChapterWidth int // 章号零填充位数（>=2）
	SectionWidth int // 节号零填充位数（>=2，无数字节时为 2）
	CSVPath      string
}

// ParseName 从文件名解析章节信息。
// 返回：章号、节号(无则0)、节单位(节/讲，特殊节为空)、特殊节名(无则空)、是否解析成功。
func ParseName(name string) (chapter, sectionNum int, sectionUnit, special string, ok bool) {
	m := reChapter.FindStringSubmatch(name)
	if m == nil {
		return 0, 0, "", "", false
	}
	chapter = ParseNumeral(m[1])

	// 优先匹配数字节/讲
	if sm := reSection.FindStringSubmatch(name); sm != nil {
		sectionNum = ParseNumeral(sm[1])
		sectionUnit = sm[2]
		return chapter, sectionNum, sectionUnit, "", true
	}
	// 再匹配特殊节
	for _, s := range specialSections {
		if strings.Contains(name, s) {
			return chapter, 0, "", s, true
		}
	}
	// 只有章、没有节
	return chapter, 0, "", "", true
}

// ParseNumeral 解析数字字符串：纯阿拉伯直接转；否则按中文数字解析。
func ParseNumeral(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return parseChinese(s)
}

func parseChinese(s string) int {
	digits := map[rune]int{
		'零': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4,
		'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
	}
	units := map[rune]int{'十': 10, '百': 100, '千': 1000, '万': 10000}
	total, current := 0, 0
	for _, ch := range s {
		if v, ok := digits[ch]; ok {
			current = v
		} else if u, ok := units[ch]; ok {
			if u == 10000 {
				total = (total + current) * u
				current = 0
			} else {
				if current == 0 {
					current = 1
				}
				total += current * u
				current = 0
			}
		}
	}
	total += current
	return total
}

func pad(n, width int) string {
	return fmt.Sprintf("%0*d", width, n)
}

func digits(n int) int {
	if n == 0 {
		return 1
	}
	d := 0
	for n > 0 {
		n /= 10
		d++
	}
	return d
}

// ScanDir 扫描目录，生成归类计划（不改动任何文件）。
func ScanDir(dir string) (*Plan, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	plan := &Plan{Dir: dir, ChapterWidth: 2, SectionWidth: 2}
	var parsed []Op

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !pptExts[ext] {
			continue
		}
		ch, secNum, secUnit, special, ok := ParseName(e.Name())
		if !ok {
			plan.Ops = append(plan.Ops, Op{
				Src:        filepath.Join(dir, e.Name()),
				Skip:       true,
				SkipReason: "无法解析章节",
			})
			continue
		}
		op := Op{
			Src:         filepath.Join(dir, e.Name()),
			Chapter:     ch,
			SectionNum:  secNum,
			SectionUnit: secUnit,
			Special:     special,
		}
		parsed = append(parsed, op)
	}

	// 计算零填充宽度（按目录内最大数字对齐，下限 2）
	maxCh, maxSec := 0, 0
	for _, op := range parsed {
		if op.Chapter > maxCh {
			maxCh = op.Chapter
		}
		if op.SectionNum > maxSec {
			maxSec = op.SectionNum
		}
	}
	if w := digits(maxCh); w > plan.ChapterWidth {
		plan.ChapterWidth = w
	}
	if maxSec > 0 {
		if w := digits(maxSec); w > plan.SectionWidth {
			plan.SectionWidth = w
		}
	}

	// 构建目标路径（绝对路径，基于 dir），并检测同名冲突
	used := map[string]bool{}
	for _, op := range parsed {
		chapterDir := filepath.Join(dir, "第"+pad(op.Chapter, plan.ChapterWidth)+"章")
		var sectionDir string
		switch {
		case op.SectionNum > 0:
			sectionDir = "第" + pad(op.SectionNum, plan.SectionWidth) + op.SectionUnit
		case op.Special != "":
			sectionDir = op.Special
		}
		targetDir := chapterDir
		if sectionDir != "" {
			targetDir = filepath.Join(chapterDir, sectionDir)
		}
		op.ChapterDir = filepath.Base(chapterDir)
		op.SectionDir = sectionDir
		op.TargetDir = targetDir

		base := filepath.Base(op.Src)
		dst := filepath.Join(targetDir, base)
		// 冲突：已占用（计划内或磁盘上已存在）→ 加 _N 后缀
		i := 2
		for used[dst] || fileExists(dst) {
			dst = filepath.Join(targetDir, insertSuffix(base, i))
			i++
		}
		used[dst] = true
		op.Dst = dst
		plan.Ops = append(plan.Ops, op)
	}
	return plan, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func insertSuffix(name string, n int) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s_%d%s", base, n, ext)
}

// Apply 执行计划：建目录、移动文件，并写出 CSV 映射（便于 undo）。
// 若 dryRun 为 true，仅打印预览、不改动文件、不写 CSV。
func Apply(plan *Plan, dryRun bool) error {
	if dryRun {
		return nil
	}
	created := map[string]bool{}
	for _, op := range plan.Ops {
		if op.Skip {
			continue
		}
		if err := os.MkdirAll(op.TargetDir, 0o755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", op.TargetDir, err)
		}
		created[op.TargetDir] = true
		if err := moveFile(op.Src, op.Dst); err != nil {
			return fmt.Errorf("移动 %s 失败: %w", op.Src, err)
		}
	}
	// 写 CSV 映射
	ts := time.Now().Format("20060102_150405")
	csvPath := filepath.Join(plan.Dir, "quickark_organize_"+ts+".csv")
	if err := writeCSV(csvPath, plan); err != nil {
		return fmt.Errorf("写映射文件失败: %w", err)
	}
	plan.CSVPath = csvPath
	return nil
}

func moveFile(src, dst string) error {
	// 同卷直接 rename；跨卷回退为 拷贝+删除
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if copyFile(src, dst) == nil {
		if err := os.Remove(src); err != nil {
			return err
		}
		return nil
	}
	return os.Rename(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func writeCSV(path string, plan *Plan) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"old", "new", "chapter", "section"}); err != nil {
		return err
	}
	for _, op := range plan.Ops {
		if op.Skip {
			continue
		}
		row := []string{op.Src, op.Dst, op.ChapterDir, op.SectionDir}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// Undo 读取 CSV 映射，把文件移回原位置，并清理已空的章节目录。
func Undo(csvPath string) error {
	f, err := os.Open(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return err
	}
	dirs := map[string]bool{}
	root := filepath.Dir(csvPath)
	for i, row := range rows {
		if i == 0 {
			continue // 表头
		}
		if len(row) < 2 {
			continue
		}
		oldPath, newPath := row[0], row[1]
		if !fileExists(newPath) {
			continue
		}
		if err := moveFile(newPath, oldPath); err != nil {
			return fmt.Errorf("还原 %s 失败: %w", newPath, err)
		}
		// 记录叶子目录及其父（章）目录，便于清理空目录
		d1 := filepath.Dir(newPath)
		dirs[d1] = true
		if d2 := filepath.Dir(d1); d2 != root {
			dirs[d2] = true
		}
	}
	// 由深到浅清理空目录（仅清理本次创建的章节目录）
	deepFirst := make([]string, 0, len(dirs))
	for d := range dirs {
		deepFirst = append(deepFirst, d)
	}
	// 按路径深度降序
	sortByDepthDesc(deepFirst)
	for _, d := range deepFirst {
		_ = removeEmptyDir(d)
	}
	return nil
}

func removeEmptyDir(d string) error {
	entries, err := os.ReadDir(d)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(d)
	}
	return nil
}

func sortByDepthDesc(dirs []string) {
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if strings.Count(dirs[j], string(os.PathSeparator)) > strings.Count(dirs[i], string(os.PathSeparator)) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}
}
