package engine

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CleanEntry is one item from mo clean --dry-run's clean-list.txt.
type CleanEntry struct {
	Path     string
	Size     int64  // parsed bytes; -1 when unknown ("size unknown")
	SizeStr  string // original human string
	Note     string // extra annotation after size, e.g. "counted under X"
	Items    int    // "N items" when reported, else 0
	Section  string
	Selected bool // UI checkbox state; default true
}

// CleanSection groups entries under a mole section.
type CleanSection struct {
	Name    string
	Entries []CleanEntry
	// Totals are computed after parse.
	TotalSize int64
	Known     int // entries with Size >= 0
}

// CleanReport is the parsed dry-run result.
type CleanReport struct {
	GeneratedAt time.Time
	Sections    []CleanSection
	TotalSize   int64
	TotalItems  int
	Partial     bool // true when any size is unknown
	SourceFile  string
	Raw         []byte
}

// CleanProvider runs `mo clean --dry-run` and parses the output.
type CleanProvider struct {
	moPath string
}

// FindCleanProvider locates the `mo` CLI.
func FindCleanProvider() *CleanProvider {
	mo := findMole()
	if mo == "" {
		return nil
	}
	return &CleanProvider{moPath: mo}
}

func (p *CleanProvider) Available() bool { return p != nil && p.moPath != "" }

// CleanListPath is where mo clean --dry-run writes its ledger.
func CleanListPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "mole", "clean-list.txt")
}

// DryRun executes `mo clean --dry-run` (non-interactive; safe, deletes
// nothing) then parses the resulting clean-list.txt.
// progress is called with status strings for the UI.
func (p *CleanProvider) DryRun(progress func(string)) (*CleanReport, error) {
	if progress == nil {
		progress = func(string) {}
	}
	progress("正在扫描可清理项目…")
	cmd := exec.Command(p.moPath, "clean", "--dry-run")
	// Non-TTY: mole runs fully automatically.
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("mo clean --dry-run: %w", err)
	}
	progress("解析预览清单…")
	return ParseCleanList(CleanListPath())
}

// sectionHeader matches "=== Name ===".
var sectionHeader = regexp.MustCompile(`^===\s+(.+?)\s+===\s*$`)

// entryLine matches "/abs/path  # <size>[extra]".
// size can be "123.4MB", "0B", "unknown", and extra may contain ", N items"
// and ", counted under X".
var entryLine = regexp.MustCompile(`^(/\S.*?)\s+#\s*(.*)$`)

var sizeRe = regexp.MustCompile(`^([\d.]+)\s*(B|KB|MB|GB|TB|PB)`)

// ParseCleanList parses clean-list.txt into a report.
func ParseCleanList(path string) (*CleanReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read clean-list: %w", err)
	}
	r := &CleanReport{SourceFile: path, Raw: data, GeneratedAt: time.Now()}
	var cur *CleanSection
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	scan.Buffer(make([]byte, 1<<20), 1<<20)
	for scan.Scan() {
		line := scan.Text()
		if m := sectionHeader.FindStringSubmatch(line); m != nil {
			r.Sections = append(r.Sections, CleanSection{Name: m[1]})
			cur = &r.Sections[len(r.Sections)-1]
			continue
		}
		if cur == nil || strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		m := entryLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		e := CleanEntry{Path: m[1], Section: cur.Name, Selected: true}
		note := m[2]
		// Split "1.5GB, 3 items, counted under X".
		parts := strings.SplitN(note, ",", 2)
		e.SizeStr = strings.TrimSpace(parts[0])
		if sm := sizeRe.FindStringSubmatch(e.SizeStr); sm != nil {
			f, _ := strconv.ParseFloat(sm[1], 64)
			mult := map[string]int64{"B": 1, "KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40, "PB": 1 << 50}[sm[2]]
			e.Size = int64(f * float64(mult))
			cur.TotalSize += e.Size
			cur.Known++
		} else {
			e.Size = -1
			r.Partial = true
		}
		if len(parts) > 1 {
			e.Note = strings.TrimSpace(parts[1])
			fmt.Sscanf(parts[1], "%d items", &e.Items)
		}
		cur.Entries = append(cur.Entries, e)
		r.TotalItems++
	}
	for i := range r.Sections {
		r.TotalSize += r.Sections[i].TotalSize
	}
	return r, scan.Err()
}

// Selected returns the subset of entries the user has chosen to clean.
func (r *CleanReport) Selected() []CleanEntry {
	var out []CleanEntry
	for i := range r.Sections {
		for j := range r.Sections[i].Entries {
			if r.Sections[i].Entries[j].Selected {
				out = append(out, r.Sections[i].Entries[j])
			}
		}
	}
	return out
}

// SelectedSize sums the known sizes of selected entries.
func (r *CleanReport) SelectedSize() int64 {
	var n int64
	for _, e := range r.Selected() {
		if e.Size > 0 {
			n += e.Size
		}
	}
	return n
}

// SelectSection toggles every entry in a section.
func (r *CleanReport) SelectSection(name string, on bool) {
	for i := range r.Sections {
		if r.Sections[i].Name == name {
			for j := range r.Sections[i].Entries {
				r.Sections[i].Entries[j].Selected = on
			}
			return
		}
	}
}

// CleanState is the UI state for the clean page.
type CleanState struct {
	Provider *CleanProvider
	Report   *CleanReport
	Running  bool   // dry-run scan in flight
	Progress string // status line for spinner
	Err      string
	// SelectedSection is which section's entries are shown on the right.
	SelectedSection int
}

// NewCleanState builds state with the provider probed.
func NewCleanState() *CleanState {
	return &CleanState{Provider: FindCleanProvider()}
}

// Run executes the dry-run. onDone runs via win.Update.
func (s *CleanState) Run(onDone func(), onProgress func(string)) {
	if s.Running {
		return
	}
	s.Running = true
	s.Err = ""
	go func() {
		rep, err := s.Provider.DryRun(func(msg string) {
			s.Progress = msg
			if onProgress != nil {
				onProgress(msg)
			}
		})
		if err != nil {
			s.Err = err.Error()
		} else {
			s.Report = rep
		}
		s.Running = false
		if onDone != nil {
			onDone()
		}
	}()
}
