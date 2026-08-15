// Package pptclean 提供可复用的 PPT 课件文件名清洗能力。
//
// 核心目标：把文件名里「第<数字><单位>」形式的编号统一归一为零填充阿拉伯数字，
// 使 Windows 资源管理器「按名称排序」即为正确教学顺序。
// 例如：
//   第九章　第31讲　...  ->  第09章　第31讲　...
//   第八章　第30讲　...  ->  第08章　第30讲　...
// 由于「章」在字符串中位于「讲」之前，且数字已零填充，嵌套排序天然正确。
package pptclean

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultUnits 是默认识别的单位集合（出现在「第<数字>」之后的分类词）。
var DefaultUnits = []string{"章", "讲", "节", "课", "单元", "篇", "部分", "回", "幕"}

// FilePlan 描述单个文件的重命名计划。
type FilePlan struct {
	Old      string // 原文件名（含扩展名）
	New      string // 计划/实际的新文件名（含扩展名）
	Changed  bool   // 文件名是否会变化
	Skipped  bool   // 无可识别编号 -> 保持原样
	Conflict bool   // 发生重名冲突，已追加 _N 后缀
	Error    string // 执行阶段的错误信息
	Status   string // renamed / skipped / error
}

// ScanOptions 控制扫描与规划行为。
type ScanOptions struct {
	Units []string // 自定义单位集合；为空则用 DefaultUnits
	Width int      // 强制零填充位数；<=0 表示按目录内最大数字自动对齐（下限 2）
}

// cnDigits 中文/大写数字到数值的映射。
var cnDigits = map[rune]int{
	'零': 0, '〇': 0,
	'一': 1, '壹': 1,
	'二': 2, '两': 2, '贰': 2,
	'三': 3, '叁': 3,
	'四': 4, '肆': 4,
	'五': 5, '伍': 5,
	'六': 6, '陆': 6,
	'七': 7, '柒': 7,
	'八': 8, '捌': 8,
	'九': 9, '玖': 9,
}

// ParseChineseNumeral 把中文/大写数字（或纯阿拉伯数字字符串）解析为整数。
// 支持范围覆盖到数千，足以应对教学章节/讲次：
//   十一、二十、二十一、一百、一百零五、一千零一、一千二百三十四 等。
func ParseChineseNumeral(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("空字符串")
	}
	// 纯阿拉伯数字优先
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}

	section := 0 // 已结算的累计值
	number := 0  // 当前正在累积的数字
	for _, ch := range s {
		if d, ok := cnDigits[ch]; ok {
			number = d
			continue
		}
		switch ch {
		case '十':
			if number == 0 {
				number = 1
			}
			section += number * 10
			number = 0
		case '百':
			if number == 0 {
				number = 1
			}
			section += number * 100
			number = 0
		case '千':
			if number == 0 {
				number = 1
			}
			section += number * 1000
			number = 0
		case '零', '〇':
			// 仅作分隔，数字清零
			number = 0
		default:
			return 0, fmt.Errorf("无法解析的中文数字: %q", s)
		}
	}
	total := section + number
	if total == 0 {
		return 0, fmt.Errorf("无效的中文数字: %q", s)
	}
	return total, nil
}

// buildRegex 根据单位集合构造「第<数字><单位>」的正则。
// 单位按长度降序排列，保证「单元」「部分」等多字单位优先匹配。
func buildRegex(units []string) *regexp.Regexp {
	sorted := append([]string{}, units...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	parts := make([]string, len(sorted))
	for i, u := range sorted {
		parts[i] = regexp.QuoteMeta(u)
	}
	unitAlt := strings.Join(parts, "|")
	pat := `第([零一二三四五六七八九十百千两\d]+)(` + unitAlt + `)`
	return regexp.MustCompile(pat)
}

// TransformName 把 name 中所有「第<数字><单位>」编号归一为零填充阿拉伯数字。
// 只改动编号 token，保留其余内容（含全角空格　与中文）。
func TransformName(name string, re *regexp.Regexp, width int) string {
	return re.ReplaceAllStringFunc(name, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if sub == nil {
			return m
		}
		numStr, unit := sub[1], sub[2]
		n, err := ParseChineseNumeral(numStr)
		if err != nil {
			return m // 解析失败则保留原样
		}
		return fmt.Sprintf("第%0*d%s", width, n, unit)
	})
}

// isPpt 判断文件名是否为 PowerPoint 文件（大小写不敏感）。
func isPpt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".ppt" || ext == ".pptx"
}

// insertSuffix 在扩展名前插入 _N 后缀。
func insertSuffix(name string, n int) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s_%d%s", base, n, ext)
}

// ScanDir 扫描 dir 下（仅当前目录、仅 .ppt/.pptx）的文件并生成重命名计划。
// 两遍扫描：第一遍确定零填充宽度（取被识别数字的最大值位数，下限 2）；
// 第二遍重算新名并检测重名冲突（重复者追加 _N 后缀）。
func ScanDir(dir string, opts ScanOptions) ([]FilePlan, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	units := opts.Units
	if len(units) == 0 {
		units = DefaultUnits
	}
	re := buildRegex(units)

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if isPpt(e.Name()) {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	plans := make([]FilePlan, len(files))

	// 第一遍：用临时宽度 2 计算新名，并收集最大数字以确定最终宽度。
	maxNum := 0
	hasAny := false
	for i, f := range files {
		newName := TransformName(f, re, 2)
		changed := newName != f
		plans[i] = FilePlan{Old: f, New: newName, Changed: changed, Skipped: !changed}
		if changed {
			for _, sm := range re.FindAllStringSubmatch(newName, -1) {
				if v, err := strconv.Atoi(sm[1]); err == nil {
					if v > maxNum {
						maxNum = v
					}
					hasAny = true
				}
			}
		}
	}

	// 确定最终宽度。
	width := opts.Width
	if width <= 0 {
		width = 2
		if hasAny {
			if w := len(strconv.Itoa(maxNum)); w > width {
				width = w
			}
		}
	}

	// 第二遍：用最终宽度重算，并检测冲突。
	seen := map[string]int{}
	for i := range plans {
		if plans[i].Skipped {
			continue
		}
		plans[i].New = TransformName(plans[i].Old, re, width)
		seen[plans[i].New]++
	}
	counts := map[string]int{}
	for i := range plans {
		if plans[i].Skipped {
			continue
		}
		base := plans[i].New
		if seen[base] > 1 {
			counts[base]++
			if counts[base] == 1 {
				continue // 首个同名者保留原名
			}
			plans[i].New = insertSuffix(base, counts[base])
			plans[i].Conflict = true
		}
	}
	return plans, nil
}

// Apply 执行重命名，并把 old->new 映射写入 CSV（含时间戳），返回 CSV 路径。
// 只有实际改名的行状态为 "renamed"，便于 Undo 精确回滚。
func Apply(dir string, plans []FilePlan) (string, error) {
	ts := time.Now().Format("20060102_150405")
	csvPath := filepath.Join(dir, "pptclean_rename_"+ts+".csv")
	f, err := os.Create(csvPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"old", "new", "status"}); err != nil {
		return csvPath, err
	}

	for i := range plans {
		p := &plans[i]
		if p.Skipped {
			p.Status = "skipped"
			_ = w.Write([]string{p.Old, p.Old, "skipped"})
			continue
		}
		oldPath := filepath.Join(dir, p.Old)
		newPath := filepath.Join(dir, p.New)
		if err := os.Rename(oldPath, newPath); err != nil {
			p.Status = "error"
			p.Error = err.Error()
			_ = w.Write([]string{p.Old, p.New, "error:" + err.Error()})
			continue
		}
		p.Status = "renamed"
		_ = w.Write([]string{p.Old, p.New, "renamed"})
	}
	if err := w.Error(); err != nil {
		return csvPath, err
	}
	return csvPath, nil
}

// Undo 读取 Apply 生成的 CSV，把所有状态为 "renamed" 的改名反向恢复。
func Undo(csvPath string) error {
	f, err := os.Open(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return err
	}
	dir := filepath.Dir(csvPath)
	for i, row := range rows {
		if i == 0 { // 表头
			continue
		}
		if len(row) < 3 || row[2] != "renamed" {
			continue
		}
		oldPath := filepath.Join(dir, row[0])
		newPath := filepath.Join(dir, row[1])
		if err := os.Rename(newPath, oldPath); err != nil {
			return fmt.Errorf("undo 失败 %s -> %s: %w", row[1], row[0], err)
		}
	}
	return nil
}
