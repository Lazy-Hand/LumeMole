package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	mo, err := exec.LookPath("mo")
	if err != nil {
		mo, err = exec.LookPath("mole")
		if err != nil {
			return nil
		}
	}
	real, err := filepath.EvalSymlinks(mo)
	if err != nil {
		real = mo
	}
	cand := filepath.Join(filepath.Dir(real), "..", "libexec", "bin", "analyze-go")
	if _, err := os.Stat(cand); err == nil {
		return &AnalyzeProvider{bin: cand}
	}
	return nil
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
