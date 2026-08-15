package pptclean

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestParseChineseNumeral(t *testing.T) {
	cases := map[string]int{
		"九":        9,
		"八":        8,
		"十":        10,
		"十一":      11,
		"二十":      20,
		"二十一":    21,
		"三十":      30,
		"三十一":    31,
		"一百":      100,
		"一百零五":  105,
		"一千零一":  1001,
		"一千二百三十四": 1234,
		"两":        2,
		"31":        31,
		"105":       105,
	}
	for in, want := range cases {
		got, err := ParseChineseNumeral(in)
		if err != nil {
			t.Errorf("ParseChineseNumeral(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseChineseNumeral(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTransformName(t *testing.T) {
	re := buildRegex(DefaultUnits)
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"第九章　第31讲　物质的聚集状态　常见晶体类型", 2, "第09章　第31讲　物质的聚集状态　常见晶体类型"},
		{"第八章　第30讲　分子的性质　配合物与超分子", 2, "第08章　第30讲　分子的性质　配合物与超分子"},
		{"第十章　第5讲　xxx", 2, "第10章　第05讲　xxx"},
		{"第十一章　第12讲　yyy", 2, "第11章　第12讲　yyy"},
		{"封面.pptx", 2, "封面.pptx"}, // 无编号，保持原样
	}
	for _, c := range cases {
		got := TransformName(c.in, re, c.width)
		if got != c.want {
			t.Errorf("TransformName(%q,width=%d) = %q, want %q", c.in, c.width, got, c.want)
		}
	}
}

// TestSortCorrectness 验证清洗后按名称排序的顺序正确：
// 第九章 必须落在 第八章 之后；第05讲 必须落在 第12讲 之前；10/11 章正确接续。
func TestSortCorrectness(t *testing.T) {
	re := buildRegex(DefaultUnits)
	names := []string{
		"第九章　第31讲　物质的聚集状态　常见晶体类型",
		"第八章　第30讲　分子的性质　配合物与超分子",
		"第十章　第5讲　xxx",
		"第十一章　第12讲　yyy",
		"第7章　第3讲　zzz",
	}
	width := 2
	transformed := make([]string, len(names))
	for i, n := range names {
		transformed[i] = TransformName(n, re, width)
	}
	sort.Strings(transformed)

	// 期望顺序：07,08,09,10,11
	wantOrder := []string{
		"第07章　第03讲　zzz",
		"第08章　第30讲　分子的性质　配合物与超分子",
		"第09章　第31讲　物质的聚集状态　常见晶体类型",
		"第10章　第05讲　xxx",
		"第11章　第12讲　yyy",
	}
	for i := range wantOrder {
		if transformed[i] != wantOrder[i] {
			t.Errorf("排序位置 %d: 得到 %q, 期望 %q", i, transformed[i], wantOrder[i])
		}
	}
}

// TestScanAndApply 端到端：建临时目录、扫描、应用、验证、undo 还原。
func TestScanAndApply(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"第九章　第31讲　A.pptx",
		"第八章　第30讲　B.pptx",
		"第5讲　C.pptx",
		"第十章　D.ppt",
		"封面.pptx",
		"第九章　第32讲　E.pptx", // 同第九章但讲次不同，新名应不同，不会冲突
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	plans, err := ScanDir(dir, ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// 封面应被跳过
	for _, p := range plans {
		if p.Old == "封面.pptx" && !p.Skipped {
			t.Errorf("封面应被标记 Skipped")
		}
	}

	csvPath, err := Apply(dir, plans)
	if err != nil {
		t.Fatal(err)
	}

	// 验证改名后存在且排序正确
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		if e.IsDir() || !isPpt(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	// 验证编号文件之间的相对顺序正确（封面.pptx 因「封」<「第」自然排在最前，属预期）。
	wantSeq := []string{
		"第05讲　C.pptx",
		"第08章　第30讲　B.pptx",
		"第09章　第31讲　A.pptx",
		"第09章　第32讲　E.pptx",
		"第10章　D.ppt",
	}
	gotSeq := []string{}
	for _, n := range names {
		if strings.HasPrefix(n, "第") {
			gotSeq = append(gotSeq, n)
		}
	}
	for i := range wantSeq {
		if gotSeq[i] != wantSeq[i] {
			t.Errorf("编号文件排序位置 %d: 得到 %q, 期望 %q（完整序: %v）", i, gotSeq[i], wantSeq[i], gotSeq)
		}
	}

	// undo 还原
	if err := Undo(csvPath); err != nil {
		t.Fatal(err)
	}
	entries, _ = os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		found := false
		for _, f := range files {
			if e.Name() == f {
				found = true
			}
		}
		if !found && !strings.HasPrefix(e.Name(), "pptclean_rename_") {
			t.Errorf("undo 后残留未还原文件: %q", e.Name())
		}
	}
}
