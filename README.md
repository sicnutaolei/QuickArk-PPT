# QuickArk-PPT

本仓库收录**三个功能互补、互不冲突**的 PPT 课件整理工具，可单独使用，也可搭配使用：

| 工具 | 语言 | 核心功能 | 是否移动文件 |
| --- | --- | --- | --- |
| **QuickArk_PPT.py** | Python | 按章节把课件**归类归档**到文件夹（原版，仅支持第一~第五章/节） | 是（建目录并移动） |
| **quickark/** | Go | 同上，但**泛化到任意章/节**且文件夹命名零填充，排序正确（Python 版的重构） | 是（建目录并移动） |
| **pptclean/** | Go | **清洗文件名**，让「按名称排序」顺序正确 | 否（原地重命名） |

> 典型搭配：先用 `pptclean` 规整文件名 → 再用 `quickark`（或 Python 版）把课件按章节归档。

---

## 1. QuickArk_PPT.py（Python · 课件归类归档 · 原版）
按学期、章节、教学目标自动把杂乱课件分级归档进清晰目录，方便查找与复习。
仅支持第一~第五章、第一~第五节，且文件名保留中文数字。
- 详细说明与用法见 **[QuickArk_PPT.md](QuickArk_PPT.md)**（即原 README）。

## 2. quickark/（Go · 课件归类归档 · Python 版重构）
`QuickArk_PPT.py` 的 Go 重构版：行为与 Python 版一致（按章节建目录并移动），但做了两点改进——
章节/节支持**任意数字**（阿拉伯或中文），生成的文件夹名用**零填充阿拉伯数字**（`第09章`、`第31讲`），
从而根治 Windows 把「第九章」排到「第八章」前面的排序 bug；并新增 dry-run 预览、确认、CSV 映射与 `undo` 回滚。
- 详细说明、构建与用法见 **[quickark/README.md](quickark/README.md)**。

## 3. pptclean/（Go · 文件名清洗排序）
解决 Windows「按名称排序」把「第九章」排到「第八章」前面这类问题：
把 `第X章 / 第X讲` 的中文数字归一为零填充阿拉伯数字（如 `第九章 第31讲` → `第09章 第31讲`），
阿拉伯数字也会补零（`第5讲` → `第05讲`），从而实现正确的章节/讲次顺序。
- 详细说明、构建与用法见 **[pptclean/README.md](pptclean/README.md)**。

---

## 快速开始

```bash
# 方式一：先规整文件名，再归类归档（Go 版）
cd pptclean && go build -o pptclean.exe ./cmd/pptclean
./pptclean "D:\课件\...\教师用书配套课件"      # 预览后确认改名

cd ../quickark && go build -o quickark.exe ./cmd/quickark
./quickark "D:\课件\...\教师用书配套课件"       # 按章节归档进文件夹

# 方式二：只想排序，不动目录结构
# 仅使用 pptclean 即可

# 方式三：用原 Python 版归档（功能同 quickark，但章节范围受限）
python QuickArk_PPT.py
```

三个工具各自独立，按需取用。
