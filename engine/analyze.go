package engine

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AnalyzeEntry is one row from analyze-go --json.
type AnalyzeEntry struct {
	ScanStatus string `json:"scan_status"` // complete | partial | unavailable
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	IsDir      bool   `json:"is_dir"`
	LastAccess string `json:"last_access,omitempty"`
	Insight    bool   `json:"insight,omitempty"`
	Cleanable  bool   `json:"cleanable,omitempty"`
}

// AnalyzeResult is analyze-go's top-level JSON.
type AnalyzeResult struct {
	ScanStatus string         `json:"scan_status"`
	Path       string         `json:"path"`
	Overview   bool           `json:"overview"`
	Entries    []AnalyzeEntry `json:"entries"`
	LargeFiles []AnalyzeEntry `json:"large_files"`
	TotalSize  int64          `json:"total_size"`
	TotalFiles int            `json:"total_files"`
}

// AnalyzeProvider shells out to mole's analyze-go.
type AnalyzeProvider struct {
	bin string
}

// FindAnalyzeProvider locates analyze-go next to the `mo` binary.
func FindAnalyzeProvider() *AnalyzeProvider {
	bin := moleLibexecBin(findMole(), "analyze-go")
	if bin == "" {
		return nil
	}
	return &AnalyzeProvider{bin: bin}
}

func (p *AnalyzeProvider) Available() bool { return p != nil && p.bin != "" }

// Scan runs `analyze-go --json [path]` and decodes the result.
// Empty path = machine overview.
func (p *AnalyzeProvider) Scan(path string) (*AnalyzeResult, error) {
	args := []string{"--json"}
	if path != "" {
		args = append(args, path)
	}
	out, err := exec.Command(p.bin, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("analyze-go: %w", err)
	}
	var r AnalyzeResult
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("decode analyze-go: %w", err)
	}
	return &r, nil
}

// AnalyzeState is the UI state for the analyze page.
type AnalyzeState struct {
	Provider *AnalyzeProvider
	// Crumb is the breadcrumb path stack; empty = overview.
	Crumbs []string
	// Result is the current level's scan result.
	Result *AnalyzeResult
	// Cache memoizes scanned dirs to avoid re-exec.
	Cache map[string]*AnalyzeResult
	// Loading marks an in-flight scan.
	Loading   bool
	ScanPath  string // path being scanned, for spinner label
	Err       string
	StartedAt time.Time
	// ViewMode: 0 = 按目录 (tree+treemap), 1 = 按类型 (kind groups).
	ViewMode int
}

// NewAnalyzeState builds state, probing for analyze-go.
func NewAnalyzeState() *AnalyzeState {
	return &AnalyzeState{
		Provider: FindAnalyzeProvider(),
		Cache:    make(map[string]*AnalyzeResult),
	}
}

// ScanTo navigates: "" = overview, else drill into path. Cached hits skip exec.
// onDone runs on the main thread via win.Update.
func (s *AnalyzeState) ScanTo(path string, onDone func()) {
	if cached, ok := s.Cache[path]; ok {
		s.Result = cached
		s.pushCrumb(path)
		if onDone != nil {
			onDone()
		}
		return
	}
	if s.Loading {
		return
	}
	s.Loading = true
	s.ScanPath = path
	s.Err = ""
	go func() {
		res, err := s.Provider.Scan(path)
		if err != nil {
			s.Err = err.Error()
		} else {
			s.Result = res
			s.Cache[path] = res
			s.pushCrumb(path)
		}
		s.Loading = false
		if onDone != nil {
			onDone()
		}
	}()
}

// Back pops one level from the crumb stack and rescans the parent.
func (s *AnalyzeState) Back(onDone func()) {
	if len(s.Crumbs) <= 1 {
		return
	}
	s.Crumbs = s.Crumbs[:len(s.Crumbs)-1]
	s.ScanTo(s.Crumbs[len(s.Crumbs)-1], onDone)
}

// JumpToCrumb navigates to an ancestor level by index.
func (s *AnalyzeState) JumpToCrumb(i int, onDone func()) {
	if i < 0 || i >= len(s.Crumbs) {
		return
	}
	s.Crumbs = s.Crumbs[:i+1]
	s.ScanTo(s.Crumbs[i], onDone)
}

func (s *AnalyzeState) pushCrumb(path string) {
	if path == "" {
		s.Crumbs = []string{""}
		return
	}
	// Trim crumbs if we drilled into a child of the current level then
	// jumped sideways; otherwise append.
	if len(s.Crumbs) == 0 || s.Crumbs[len(s.Crumbs)-1] != path {
		// If path is already in crumbs (going back), trim to it.
		for i, p := range s.Crumbs {
			if p == path {
				s.Crumbs = s.Crumbs[:i+1]
				return
			}
		}
		s.Crumbs = append(s.Crumbs, path)
	}
}

// CrumbLabel returns display name for a crumb.
func CrumbLabel(path string) string {
	if path == "" {
		return "整个磁盘"
	}
	return filepath.Base(path)
}

// TreeNode is a directory tree node for the treemap.
type TreeNode struct {
	Name     string
	Path     string
	Size     int64
	IsDir    bool
	Children []*TreeNode
	entry    *AnalyzeEntry
}

// BuildTree converts flat entries into a single-level tree for treemap.
// Entries are already one level deep (children of Result.Path).
func BuildTree(result *AnalyzeResult) []*TreeNode {
	if result == nil {
		return nil
	}
	nodes := make([]*TreeNode, 0, len(result.Entries))
	for i := range result.Entries {
		e := &result.Entries[i]
		nodes = append(nodes, &TreeNode{
			Name:  e.Name,
			Path:  e.Path,
			Size:  e.Size,
			IsDir: e.IsDir,
			entry: e,
		})
	}
	return nodes
}

// FileKind classifies an entry by its extension into Mole's analyze kinds.
type FileKind string

const (
	KindImage     FileKind = "图片"
	KindVideo     FileKind = "视频"
	KindAudio     FileKind = "音频"
	KindDocument  FileKind = "文档"
	KindArchive   FileKind = "压缩包"
	KindInstaller FileKind = "安装包"
	KindDiskImage FileKind = "磁盘镜像"
	KindDeveloper FileKind = "开发文件"
	KindDesign    FileKind = "设计"
	KindBackup    FileKind = "备份"
	KindFont      FileKind = "字体"
	KindBook      FileKind = "图书"
	KindApp       FileKind = "应用"
	KindAlias     FileKind = "替身"
	KindData      FileKind = "数据"
	KindOther     FileKind = "其他"
	KindFolder    FileKind = "文件夹"
)

// kindByExt maps lowercase extensions (no dot) to kinds.
var kindByExt = map[string]FileKind{
	"jpg": KindImage, "jpeg": KindImage, "png": KindImage, "gif": KindImage,
	"heic": KindImage, "heif": KindImage, "webp": KindImage, "tiff": KindImage,
	"bmp": KindImage, "svg": KindImage, "icns": KindImage, "raw": KindImage,
	"dng": KindImage, "cr2": KindImage, "nef": KindImage,

	"mp4": KindVideo, "mov": KindVideo, "mkv": KindVideo, "avi": KindVideo,
	"webm": KindVideo, "m4v": KindVideo, "flv": KindVideo, "wmv": KindVideo,

	"mp3": KindAudio, "aac": KindAudio, "flac": KindAudio, "wav": KindAudio,
	"ogg": KindAudio, "m4a": KindAudio, "aiff": KindAudio, "opus": KindAudio,

	"pdf": KindDocument, "doc": KindDocument, "docx": KindDocument,
	"txt": KindDocument, "rtf": KindDocument, "md": KindDocument,
	"pages": KindDocument, "numbers": KindDocument, "key": KindDocument,
	"xls": KindDocument, "xlsx": KindDocument, "ppt": KindDocument,
	"pptx": KindDocument, "csv": KindDocument, "epub": KindBook, "mobi": KindBook,

	"zip": KindArchive, "tar": KindArchive, "gz": KindArchive, "bz2": KindArchive,
	"xz": KindArchive, "7z": KindArchive, "rar": KindArchive, "zst": KindArchive,

	"pkg": KindInstaller, "mpkg": KindInstaller,
	"dmg": KindDiskImage, "iso": KindDiskImage, "sparseimage": KindDiskImage,

	"go": KindDeveloper, "py": KindDeveloper, "js": KindDeveloper,
	"ts": KindDeveloper, "tsx": KindDeveloper, "jsx": KindDeveloper,
	"java": KindDeveloper, "kt": KindDeveloper, "rs": KindDeveloper,
	"cpp": KindDeveloper, "c": KindDeveloper, "h": KindDeveloper,
	"swift": KindDeveloper, "rb": KindDeveloper, "php": KindDeveloper,
	"sh": KindDeveloper, "json": KindDeveloper, "yaml": KindDeveloper,
	"yml": KindDeveloper, "toml": KindDeveloper, "xml": KindDeveloper,
	"html": KindDeveloper, "css": KindDeveloper, "scss": KindDeveloper,
	"sql": KindDeveloper, "ipynb": KindDeveloper,

	"sketch": KindDesign, "fig": KindDesign, "xd": KindDesign,
	"psd": KindDesign, "ai": KindDesign, "indd": KindDesign,
	"afdesign": KindDesign, "afphoto": KindDesign,

	"bak": KindBackup, "backup": KindBackup, "old": KindBackup,

	"ttf": KindFont, "otf": KindFont, "woff": KindFont, "woff2": KindFont,

	"app": KindApp, "appex": KindApp,

	"sqlite": KindData, "sqlite3": KindData, "db": KindData,
	"plist": KindData, "log": KindData, "bin": KindData, "dat": KindData,
}

// KindOf classifies an entry by extension. Directories return KindFolder.
func KindOf(e AnalyzeEntry) FileKind {
	if e.IsDir {
		return KindFolder
	}
	name := strings.ToLower(e.Name)
	if strings.HasSuffix(name, ".app") || strings.HasSuffix(name, ".dmg") {
		return kindByExt[strings.TrimPrefix(filepath.Ext(name), ".")]
	}
	ext := strings.TrimPrefix(filepath.Ext(name), ".")
	if k, ok := kindByExt[ext]; ok {
		return k
	}
	// macOS alias files often have no extension; treat small unknowns as data.
	return KindOther
}

// KindGroup aggregates entries of one kind for the "按类型" view.
type KindGroup struct {
	Kind    FileKind
	Count   int
	Size    int64
	Entries []AnalyzeEntry
}

// GroupByKind buckets entries (files only — dirs go to the path view).
func GroupByKind(entries []AnalyzeEntry) []KindGroup {
	groups := map[FileKind]*KindGroup{}
	for _, e := range entries {
		if e.IsDir {
			continue
		}
		k := KindOf(e)
		g, ok := groups[k]
		if !ok {
			g = &KindGroup{Kind: k}
			groups[k] = g
		}
		g.Count++
		g.Size += e.Size
		g.Entries = append(g.Entries, e)
	}
	out := make([]KindGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}
