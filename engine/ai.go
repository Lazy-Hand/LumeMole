package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AIItem is one cleanable thing in the AI-tools page.
type AIItem struct {
	ID       string
	Tool     string // Claude Code | Codex | Cursor | Gemini | Ollama
	Kind     string // sessions | cache | backup | oldbin | db-backup
	Title    string
	Detail   string
	Path     string   // file or dir to trash (empty for command-backed items)
	Size     int64
	Count    int      // sessions/files inside, when meaningful
	Band     AIBand
	Selected bool
}

// AIBand groups items by how careful the user should be.
type AIBand int

const (
	// BandSafe — caches and stale backups, regenerate themselves.
	BandSafe AIBand = iota
	// BandReview — conversations and archives; possibly the only copy.
	BandReview
	// BandTool — whole-tool maintenance (unused tools, model pruning).
	BandTool
)

// AIReport is the scan result for the AI-tools page.
type AIReport struct {
	Items    []AIItem
	Scanned  time.Time
	Warnings []string
}

// Total returns the selected bytes.
func (r *AIReport) Total() int64 {
	var n int64
	for _, it := range r.Items {
		if it.Selected {
			n += it.Size
		}
	}
	return n
}

// AIScanner finds cleanable AI-tool data.
type AIScanner struct {
	home         string
	sessionMaxAge time.Duration // sessions older than this are candidates
}

// NewAIScanner with a session retention threshold (default 30d).
func NewAIScanner() *AIScanner {
	h, _ := os.UserHomeDir()
	return &AIScanner{home: h, sessionMaxAge: 30 * 24 * time.Hour}
}

// Scan enumerates cleanable data across the known AI tools.
func (s *AIScanner) Scan() *AIReport {
	r := &AIReport{Scanned: time.Now()}
	r.Items = append(r.Items, s.claude()...)
	r.Items = append(r.Items, s.codex()...)
	r.Items = append(r.Items, s.cursor()...)
	r.Items = append(r.Items, s.gemini()...)
	r.Items = append(r.Items, s.ollama()...)
	sort.Slice(r.Items, func(i, j int) bool {
		if r.Items[i].Band != r.Items[j].Band {
			return r.Items[i].Band < r.Items[j].Band
		}
		return r.Items[i].Size > r.Items[j].Size
	})
	for i := range r.Items {
		r.Items[i].Selected = r.Items[i].Band == BandSafe
	}
	return r
}

// --- Claude Code ---

func (s *AIScanner) claude() []AIItem {
	root := filepath.Join(s.home, ".claude")
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	var out []AIItem
	// Old session transcripts: ~/.claude/projects/<slug>/*.jsonl
	proj := filepath.Join(root, "projects")
	if paths, size := oldFiles(proj, ".jsonl", s.sessionMaxAge); len(paths) > 0 {
		out = append(out, AIItem{
			ID: "claude-sessions", Tool: "Claude Code", Kind: "sessions",
			Title:  "Claude Code 聊天记录",
			Detail: fmt.Sprintf("%d 个对话超过 %d 天未更新，清理后无法在历史中回看", len(paths), int(s.sessionMaxAge.Hours()/24)),
			Path:   proj, Size: size, Count: len(paths), Band: BandReview,
		})
	}
	// Debug/paste caches — safe.
	for _, sub := range []struct{ dir, title string }{
		{"cache", "缓存"},
		{"paste-cache", "粘贴缓存"},
		{"file-history", "文件历史缓存"},
	} {
		p := filepath.Join(root, sub.dir)
		if sz := dirSizeQuiet(p); sz > 1<<20 {
			out = append(out, AIItem{
				ID: "claude-" + sub.dir, Tool: "Claude Code", Kind: "cache",
				Title:  "Claude Code " + sub.title,
				Detail: "需要时自动重建",
				Path:   p, Size: sz, Band: BandSafe,
			})
		}
	}
	return out
}

// --- Codex ---

func (s *AIScanner) codex() []AIItem {
	root := filepath.Join(s.home, ".codex")
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	var out []AIItem
	// Old rollout sessions (~/.codex/sessions/YYYY/MM/DD/*.jsonl[.zst]).
	sess := filepath.Join(root, "sessions")
	if paths, size := oldSessionFiles(sess, s.sessionMaxAge); len(paths) > 0 {
		out = append(out, AIItem{
			ID: "codex-sessions", Tool: "Codex", Kind: "sessions",
			Title:  "Codex 聊天记录",
			Detail: fmt.Sprintf("%d 个对话超过 %d 天未更新，清理后无法在 Codex 中继续", len(paths), int(s.sessionMaxAge.Hours()/24)),
			Path:   sess, Size: size, Count: len(paths), Band: BandReview,
		})
	}
	// SQLite log backups (logs_*.sqlite.backup-*): leftover checkpoints.
	if ents, err := os.ReadDir(root); err == nil {
		for _, e := range ents {
			if strings.Contains(e.Name(), ".sqlite.backup") {
				p := filepath.Join(root, e.Name())
				out = append(out, AIItem{
					ID: "codex-dbbackup-" + e.Name(), Tool: "Codex", Kind: "db-backup",
					Title:  "Codex 数据库备份",
					Detail: e.Name() + "，旧检查点文件",
					Path:   p, Size: dirSizeQuiet(p), Band: BandSafe,
				})
			}
		}
	}
	if sz := dirSizeQuiet(filepath.Join(root, "cache")); sz > 1<<20 {
		out = append(out, AIItem{
			ID: "codex-cache", Tool: "Codex", Kind: "cache",
			Title: "Codex 缓存", Detail: "需要时自动重建",
			Path: filepath.Join(root, "cache"), Size: sz, Band: BandSafe,
		})
	}
	// Orphaned visualizations of deleted sessions would go here (Codex
	// stores generated images keyed by session); left for a later pass —
	// the mapping needs the thread sqlite.
	return out
}

// --- Cursor ---

func (s *AIScanner) cursor() []AIItem {
	root := filepath.Join(s.home, "Library", "Application Support", "Cursor")
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	var out []AIItem
	// Timeline snapshots: versioned per-file history, the biggest chunk.
	if sz := dirSizeQuiet(filepath.Join(root, "snapshots")); sz > 1<<20 {
		out = append(out, AIItem{
			ID: "cursor-snapshots", Tool: "Cursor", Kind: "backup",
			Title:  "Cursor 文件时间线快照",
			Detail: "本地历史快照，删除后无法回滚到旧版本",
			Path:   filepath.Join(root, "snapshots"), Size: sz, Band: BandReview,
		})
	}
	if sz := dirSizeQuiet(filepath.Join(root, "logs")); sz > 1<<20 {
		out = append(out, AIItem{
			ID: "cursor-logs", Tool: "Cursor", Kind: "cache",
			Title: "Cursor 日志", Detail: "运行日志，自动重建",
			Path: filepath.Join(root, "logs"), Size: sz, Band: BandSafe,
		})
	}
	return out
}

// --- Gemini ---

func (s *AIScanner) gemini() []AIItem {
	root := filepath.Join(s.home, ".gemini")
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	var out []AIItem
	// ~/.gemini/bin holds downloaded CLI binaries; stray installer temp
	// files (locks, partial tarballs) are safe to drop.
	bin := filepath.Join(root, "bin")
	if ents, err := os.ReadDir(bin); err == nil {
		var size int64
		var n int
		for _, e := range ents {
			nm := e.Name()
			if strings.HasSuffix(nm, ".lock") || strings.HasSuffix(nm, ".tmp") ||
				strings.Contains(nm, ".tmp.") || strings.HasSuffix(nm, ".tar.gz") {
				size += dirSizeQuiet(filepath.Join(bin, nm))
				n++
			}
		}
		if n > 0 && size > 1<<20 {
			out = append(out, AIItem{
				ID: "gemini-bin-junk", Tool: "Gemini", Kind: "oldbin",
				Title:  "Gemini CLI 安装残留",
				Detail: fmt.Sprintf("%d 个安装临时文件（锁文件与未完成下载）", n),
				Path:   bin, Size: size, Band: BandSafe,
			})
		}
	}
	if sz := dirSizeQuiet(filepath.Join(root, "logs")); sz > 1<<20 {
		out = append(out, AIItem{
			ID: "gemini-logs", Tool: "Gemini", Kind: "cache",
			Title: "Gemini 日志", Detail: "运行日志，自动重建",
			Path: filepath.Join(root, "logs"), Size: sz, Band: BandSafe,
		})
	}
	return out
}

// --- Ollama ---

func (s *AIScanner) ollama() []AIItem {
	// Model pruning needs the ollama server; if its dir exists at all we
	// surface a review item that shells out — models are big and the list
	// command is cheap.
	if _, err := os.Stat(filepath.Join(s.home, ".ollama")); err != nil {
		return nil
	}
	// TODO: `ollama list` + last-used heuristic. Skipping keeps the scan
	// honest — we don't delete models blindly.
	return nil
}

// oldFiles returns regular files under root matching suffix, modified
// before now-age. Returns paths and total size.
func oldFiles(root, suffix string, age time.Duration) ([]string, int64) {
	var paths []string
	var size int64
	cutoff := time.Now().Add(-age)
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || !fi.Mode().IsRegular() {
			return nil
		}
		if strings.HasSuffix(fi.Name(), suffix) && fi.ModTime().Before(cutoff) {
			paths = append(paths, p)
			size += fi.Size()
		}
		return nil
	})
	return paths, size
}

// oldSessionFiles is oldFiles for .jsonl and .jsonl.zst both.
func oldSessionFiles(root string, age time.Duration) ([]string, int64) {
	var paths []string
	var size int64
	cutoff := time.Now().Add(-age)
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || !fi.Mode().IsRegular() {
			return nil
		}
		if (strings.HasSuffix(fi.Name(), ".jsonl") || strings.HasSuffix(fi.Name(), ".jsonl.zst")) &&
			fi.ModTime().Before(cutoff) {
			paths = append(paths, p)
			size += fi.Size()
		}
		return nil
	})
	return paths, size
}

// dirSizeQuiet is dirSize that swallows errors (returns 0).
func dirSizeQuiet(path string) int64 {
	if _, err := os.Stat(path); err != nil {
		return 0
	}
	return dirSize(path)
}

// SessionFiles returns the actual file list for a sessions-kind item, so
// deletion removes only the old files rather than the whole dir.
func (it *AIItem) SessionFiles() []string {
	if it.Kind != "sessions" || it.Path == "" {
		return nil
	}
	var paths []string
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	_ = filepath.Walk(it.Path, func(p string, fi os.FileInfo, err error) error {
		if err != nil || !fi.Mode().IsRegular() {
			return nil
		}
		ok := strings.HasSuffix(fi.Name(), ".jsonl") ||
			strings.HasSuffix(fi.Name(), ".jsonl.zst")
		if ok && fi.ModTime().Before(cutoff) {
			paths = append(paths, p)
		}
		return nil
	})
	return paths
}

// DeletePaths returns every path this item removes: the aged file list
// for sessions, junk files for oldbin, the item's dir otherwise.
func (it *AIItem) DeletePaths() []string {
	switch it.Kind {
	case "sessions":
		if files := it.SessionFiles(); len(files) > 0 {
			return files
		}
	case "oldbin":
		var paths []string
		if ents, err := os.ReadDir(it.Path); err == nil {
			for _, e := range ents {
				nm := e.Name()
				if strings.HasSuffix(nm, ".lock") || strings.HasSuffix(nm, ".tmp") ||
					strings.Contains(nm, ".tmp.") || strings.HasSuffix(nm, ".tar.gz") {
					paths = append(paths, filepath.Join(it.Path, nm))
				}
			}
		}
		return paths
	}
	if it.Path != "" {
		return []string{it.Path}
	}
	return nil
}

// AIState is the UI state for the AI-tools page.
type AIState struct {
	Report   *AIReport
	Scanning bool
	Cleaning bool
	Done     bool // result overlay after a clean
	Freed    int64
	Err      string
}

// Scan refreshes the report.
func (s *AIState) Scan(onDone func()) {
	if s.Scanning {
		return
	}
	s.Scanning = true
	s.Err = ""
	go func() {
		s.Report = NewAIScanner().Scan()
		s.Scanning = false
		if onDone != nil {
			onDone()
		}
	}()
}

// aiTrash moves path to the user's Trash via Finder (recoverable,
// same as Mole's 移到废纸篓). Kept mygo-free so the engine stays headless.
var aiTrash = func(path string) error {
	q := strings.ReplaceAll(path, `\`, `\\`)
	q = strings.ReplaceAll(q, `"`, `\"`)
	return exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application "Finder" to delete POSIX file "%s"`, q)).Run()
}

// Clean removes every selected item (file-wise for sessions, whole dirs
// for caches/backups) and reports freed bytes.
func (s *AIState) Clean(onDone func()) {
	if s.Cleaning || s.Report == nil {
		return
	}
	s.Cleaning = true
	go func() {
		var freed int64
		var failed []string
		for _, it := range s.Report.Items {
			if !it.Selected {
				continue
			}
			for _, p := range it.DeletePaths() {
				if err := aiTrash(p); err != nil {
					failed = append(failed, p)
					continue
				}
			}
			if it.Kind == "sessions" {
				freed += it.Size
			} else {
				freed += it.Size
			}
		}
		s.Freed = freed
		s.Done = true
		s.Cleaning = false
		if len(failed) > 0 {
			s.Err = fmt.Sprintf("%d 项未能移到废纸篓", len(failed))
		}
		if onDone != nil {
			onDone()
		}
	}()
}
