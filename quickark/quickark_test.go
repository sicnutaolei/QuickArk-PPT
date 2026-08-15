package quickark

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestParseNumeral(t *testing.T) {
	cases := map[string]int{
		"9":    9,
		"31":   31,
		"100":  100,
		"九":    9,
		"十一":   11,
		"二十":   20,
		"二十一":  21,
		"一百零五": 105,
		"一千零一": 1001,
		"三":    3,
	}
	for in, want := range cases {
		if got := ParseNumeral(in); got != want {
			t.Errorf("ParseNumeral(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseName(t *testing.T) {
	cases := []struct {
		name       string
		wantCh     int
		wantSec    int
		wantUnit   string
		wantSpecial string
		wantOk     bool
	}{
		{"第九章　第31讲　物质的聚集状态.pptx", 9, 31, "讲", "", true},
		{"第一章　第1讲　绪论.pptx", 1, 1, "讲", "", true},
		{"第10章　第5节　晶体常识.ppt", 10, 5, "节", "", true},
		{"第三章　整理与提升.pptx", 3, 0, "", "整理与提升", true},
		{"第五章　复习课.pptx", 5, 0, "", "复习课", true},
		{"第七章　提升课.pptx", 7, 0, "", "提升课", true},
		{"第11章　第12讲　单元测试.pptx", 11, 12, "讲", "", true},
		{"封面.pptx", 0, 0, "", "", false},
	}
	for _, c := range cases {
		ch, sec, unit, special, ok := ParseName(c.name)
		if ok != c.wantOk || ch != c.wantCh || sec != c.wantSec || unit != c.wantUnit || special != c.wantSpecial {
			t.Errorf("ParseName(%q) = (%d,%d,%q,%q,%v), want (%d,%d,%q,%q,%v)",
				c.name, ch, sec, unit, special, ok, c.wantCh, c.wantSec, c.wantUnit, c.wantSpecial, c.wantOk)
		}
	}
}

func TestScanDirAndApply(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"第八章　第30讲　分子的性质.pptx",
		"第九章　第31讲　物质的聚集状态.pptx",
		"第十章　第5讲　晶体常识.ppt",
		"第十一章　第12讲　单元测试.pptx",
		"第三章　整理与提升.pptx",
		"封面.pptx",
		"第一章　第1讲　绪论.pptx",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// 检查章号补零（最大章 11 → 宽度 2）
	if plan.ChapterWidth != 2 {
		t.Errorf("ChapterWidth = %d, want 2", plan.ChapterWidth)
	}
	// 第09章 应在 第08章 之后（关键 bug 修复）
	chapters := map[string]bool{}
	for _, op := range plan.Ops {
		if !op.Skip {
			chapters[op.ChapterDir] = true
		}
	}
	if !chapters["第08章"] || !chapters["第09章"] || !chapters["第10章"] || !chapters["第11章"] {
		t.Errorf("缺少预期的章目录: %v", chapters)
	}

	// 执行
	if err := Apply(plan, false); err != nil {
		t.Fatal(err)
	}
	if plan.CSVPath == "" {
		t.Fatal("未生成 CSV 映射文件")
	}

	// 验证目录结构存在且排序正确
	got := []string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			got = append(got, e.Name())
		}
	}
	sort.Strings(got)
	wantChapters := []string{"第08章", "第09章", "第10章", "第11章", "第03章"}
	for _, w := range wantChapters {
		found := false
		for _, g := range got {
			if g == w {
				found = true
			}
		}
		if !found {
			t.Errorf("未找到章目录 %q，现有: %v", w, got)
		}
	}
	// 第08章 应排在第09章 之前（字符串排序）
	if idx(got, "第08章") > idx(got, "第09章") {
		t.Errorf("第08章 未排在第09章 之前: %v", got)
	}

	// 第09章 下应有 第31讲
	secEntries, _ := os.ReadDir(filepath.Join(dir, "第09章"))
	if len(secEntries) != 1 || secEntries[0].Name() != "第31讲" {
		t.Errorf("第09章 下结构异常: %v", secEntries)
	}

	// 特殊节：第03章/整理与提升
	if _, err := os.Stat(filepath.Join(dir, "第03章", "整理与提升")); err != nil {
		t.Errorf("特殊节目录缺失: %v", err)
	}

	// 封面.pptx 应保留在原目录（跳过）
	if _, err := os.Stat(filepath.Join(dir, "封面.pptx")); err != nil {
		t.Errorf("封面.pptx 应被跳过留在原处: %v", err)
	}

	// undo 还原
	if err := Undo(plan.CSVPath); err != nil {
		t.Fatal(err)
	}
	// 文件应回到原目录
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("undo 后 %q 未还原: %v", f, err)
		}
	}
	// 章节目录应已被清理
	for _, ch := range []string{"第08章", "第09章", "第10章", "第11章", "第03章"} {
		if _, err := os.Stat(filepath.Join(dir, ch)); err == nil {
			t.Errorf("undo 后章目录 %q 应已删除", ch)
		}
	}
}

func TestConflictSuffix(t *testing.T) {
	dir := t.TempDir()
	// 目标目录里预先放一个同名文件，制造冲突
	chapDir := "第09章"
	if err := os.MkdirAll(filepath.Join(dir, chapDir), 0o755); err != nil {
		t.Fatal(err)
	}
	// 预先放的文件名需与源文件最终落地的文件名一致（工具不改文件名，只移动）
	pre := "第九章　相同名.pptx"
	if err := os.WriteFile(filepath.Join(dir, chapDir, pre), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 源文件：第九章，映射到 第09章，与已存在文件同名 → 应加 _N 后缀
	if err := os.WriteFile(filepath.Join(dir, pre), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	hasSuffix := false
	for _, op := range plan.Ops {
		if op.Skip {
			continue
		}
		if contains(filepath.Base(op.Dst), "_2.") {
			hasSuffix = true
		}
	}
	if !hasSuffix {
		t.Errorf("同名冲突未加 _N 后缀")
	}
}

func idx(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
