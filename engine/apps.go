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

// AppInfo is one installed application with its on-disk footprint.
type AppInfo struct {
	Name      string
	BundleID  string
	Path      string // .app bundle path
	Version   string
	AppSize   int64 // size of the .app bundle
	DataSize  int64 // size of its ~/Library data dirs
	LastUsed  time.Time
	Running   bool
	Remnants  []Remnant // associated files outside the bundle
	Protected bool   // true when in DATA_PROTECTED_BUNDLES / system critical
	Reason    string // why protected
}

// TotalSize is app bundle + data.
func (a *AppInfo) TotalSize() int64 { return a.AppSize + a.DataSize }

// Remnant is one file/dir associated with an app, outside its bundle.
type Remnant struct {
	Path     string
	Size     int64
	IsDir    bool
	Kind     string // caches|logs|preferences|containers|saved-state|launchagents|…
	System   bool   // under /Library (needs sudo to remove)
	Selected bool   // checkbox state
}

// AppScanner finds installed apps and their data.
type AppScanner struct {
	home string
}

func NewAppScanner() *AppScanner {
	home, _ := os.UserHomeDir()
	return &AppScanner{home: home}
}

// ScanInventory returns installed apps from mdfind + /Applications dirs,
// with versions and bundle sizes.
func (s *AppScanner) ScanInventory() ([]*AppInfo, error) {
	paths := map[string]bool{}
	// mdfind catches apps anywhere (Launch Services registry).
	out, err := exec.Command("mdfind", "kMDItemKind == 'Application'").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasSuffix(line, ".app") {
				paths[line] = true
			}
		}
	}
	// Fallback: walk the standard dirs.
	for _, dir := range []string{"/Applications", filepath.Join(s.home, "Applications"), "/System/Applications"} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".app") {
				paths[filepath.Join(dir, e.Name())] = true
			}
		}
	}

	var apps []*AppInfo
	for p := range paths {
		info := readAppInfo(p)
		if info == nil {
			continue
		}
		apps = append(apps, info)
	}
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps, nil
}

// readAppInfo parses Info.plist + measures the bundle.
func readAppInfo(path string) *AppInfo {
	plistPath := filepath.Join(path, "Contents", "Info.plist")
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", plistPath).Output()
	if err != nil {
		return nil
	}
	name := extractJSON(out, "CFBundleDisplayName")
	if name == "" {
		name = extractJSON(out, "CFBundleName")
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".app")
	}
	bundleID := extractJSON(out, "CFBundleIdentifier")
	version := extractJSON(out, "CFBundleShortVersionString")

	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	a := &AppInfo{
		Name:     name,
		BundleID: bundleID,
		Path:     path,
		Version:  version,
		LastUsed: st.ModTime(),
	}
	a.AppSize = dirSize(path)
	return a
}

// dirSize returns bytes for path using `du -sk` (falls back to walk).
func dirSize(path string) int64 {
	out, err := exec.Command("du", "-sk", path).Output()
	if err == nil {
		var kb int64
		fmt.Sscanf(string(out), "%d", &kb)
		return kb * 1024
	}
	var n int64
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() {
			n += fi.Size()
		}
		return nil
	})
	return n
}

// extractJSON pulls a string value for key out of `plutil -convert json` output.
// It is lenient: matches `"key" : "value"` with any whitespace.
func extractJSON(data []byte, key string) string {
	s := string(data)
	needle := fmt.Sprintf(`"%s"`, key)
	i := strings.Index(s, needle)
	if i < 0 {
		return ""
	}
	rest := s[i+len(needle):]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = rest[colon+1:]
	q1 := strings.Index(rest, `"`)
	if q1 < 0 {
		return ""
	}
	rest = rest[q1+1:]
	q2 := strings.Index(rest, `"`)
	if q2 < 0 {
		return ""
	}
	return rest[:q2]
}

// remnantRoots are the user-level dirs scanned for app-associated files.
var remnantRoots = []struct{ Dir, Kind string }{
	{"Library/Application Support", "app-support"},
	{"Library/Caches", "caches"},
	{"Library/Logs", "logs"},
	{"Library/Preferences", "preferences"},
	{"Library/Saved Application State", "saved-state"},
	{"Library/Containers", "containers"},
	{"Library/Group Containers", "group-containers"},
	{"Library/HTTPStorages", "httpstorages"},
	{"Library/WebKit", "webkit"},
	{"Library/Cookies", "cookies"},
	{"Library/LaunchAgents", "launchagents"},
	{"Library/Application Scripts", "scripts"},
	{"Library/Autosave Information", "autosave"},
}

// nameVariants returns the name spellings to match: raw, no-space, underscore,
// hyphen, lowercase of each, plus the bundle id's last component.
func nameVariants(name, bundleID string) []string {
	set := map[string]bool{}
	base := []string{name}
	if bundleID != "" {
		parts := strings.Split(bundleID, ".")
		if len(parts) > 0 {
			base = append(base, parts[len(parts)-1])
		}
		base = append(base, bundleID)
	}
	for _, n := range base {
		for _, v := range []string{
			n,
			strings.ReplaceAll(n, " ", ""),
			strings.ReplaceAll(n, " ", "_"),
			strings.ReplaceAll(n, " ", "-"),
		} {
			for _, vv := range []string{v, strings.ToLower(v)} {
				if vv != "" {
					set[vv] = true
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// FindRemnants lists files under ~/Library associated with the app.
func (s *AppScanner) FindRemnants(a *AppInfo) []Remnant {
	variants := nameVariants(a.Name, a.BundleID)
	var out []Remnant
	seen := map[string]bool{}

	for _, rr := range remnantRoots {
		root := filepath.Join(s.home, rr.Dir)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if matchVariant(e.Name(), variants) {
				p := filepath.Join(root, e.Name())
				if seen[p] {
					continue
				}
				seen[p] = true
				out = append(out, Remnant{
					Path:     p,
					IsDir:    e.IsDir(),
					Kind:     rr.Kind,
					Selected: true,
				})
			}
		}
	}
	// Preferences are .plist files matched by bundle id.
	if a.BundleID != "" {
		prefDir := filepath.Join(s.home, "Library", "Preferences")
		if entries, err := os.ReadDir(prefDir); err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), a.BundleID) {
					p := filepath.Join(prefDir, e.Name())
					if !seen[p] {
						seen[p] = true
						out = append(out, Remnant{Path: p, Kind: "preferences", Selected: true})
					}
				}
			}
		}
	}
	// Measure sizes (dirs via du).
	for i := range out {
		out[i].Size = dirSize(out[i].Path)
	}
	return out
}

// matchVariant reports whether name contains any variant (case-insensitive).
func matchVariant(name string, variants []string) bool {
	lname := strings.ToLower(name)
	for _, v := range variants {
		if v != "" && strings.Contains(lname, strings.ToLower(v)) {
			return true
		}
	}
	return false
}

// AppState is the UI state for the software page.
type AppState struct {
	Scanner   *AppScanner
	Apps      []*AppInfo
	Loading   bool
	Err       string
	Selected  int      // selected app index (-1 none)
	Remnants  []Remnant // remnants of selected app
	Query     string   // search filter
	SubTab    int      // 0=卸载 1=更新 2=启动项
	Updates   *UpdateState
	Startups  *StartupState
}

func NewAppState() *AppState {
	return &AppState{Scanner: NewAppScanner(), Selected: -1, Updates: NewUpdateState()}
}

// Scan loads the app inventory in a goroutine.
func (s *AppState) Scan(onDone func()) {
	if s.Loading {
		return
	}
	s.Loading = true
	s.Err = ""
	go func() {
		apps, err := s.Scanner.ScanInventory()
		if err != nil {
			s.Err = err.Error()
		} else {
			s.Apps = apps
		}
		s.Loading = false
		if onDone != nil {
			onDone()
		}
	}()
}

// SelectApp picks an app and scans its remnants.
func (s *AppState) SelectApp(i int, onDone func()) {
	if i < 0 || i >= len(s.Apps) {
		return
	}
	s.Selected = i
	a := s.Apps[i]
	go func() {
		s.Remnants = s.Scanner.FindRemnants(a)
		a.Running = isRunning(a.BundleID)
		if onDone != nil {
			onDone()
		}
	}()
}

// isRunning checks whether a bundle id has a live process.
func isRunning(bundleID string) bool {
	if bundleID == "" {
		return false
	}
	out, _ := exec.Command("pgrep", "-fl", bundleID).Output()
	return len(strings.TrimSpace(string(out))) > 0
}

// FilteredApps returns apps matching the search query.
func (s *AppState) FilteredApps() []*AppInfo {
	if s.Query == "" {
		return s.Apps
	}
	q := strings.ToLower(s.Query)
	var out []*AppInfo
	for _, a := range s.Apps {
		if strings.Contains(strings.ToLower(a.Name), q) ||
			strings.Contains(strings.ToLower(a.BundleID), q) {
			out = append(out, a)
		}
	}
	return out
}
