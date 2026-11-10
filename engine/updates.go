package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AppUpdate is one app with a newer version available.
type AppUpdate struct {
	Source    string   // homebrew-cask | appstore (mas)
	Name      string   // cask token or app name
	Installed []string // installed version(s)
	Latest    string   // available version
	Pinned    bool     // brew pinned — cannot upgrade
}

// UpdateChecker scans for outdated apps across channels.
type UpdateChecker struct{}

// brewPath returns the brew binary, outside PATH too.
func brewPath() string {
	if p, err := exec.LookPath("brew"); err == nil {
		return p
	}
	for _, p := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ScanBrew lists outdated Homebrew casks (greedy: includes auto-updating
// apps, which still respond to brew upgrade). Empty slice when brew or no
// outdated casks.
func (u *UpdateChecker) ScanBrew() ([]AppUpdate, error) {
	brew := brewPath()
	if brew == "" {
		return nil, fmt.Errorf("未安装 Homebrew")
	}
	out, err := exec.Command(brew, "outdated", "--cask", "--greedy", "--json=v2").Output()
	if err != nil {
		return nil, fmt.Errorf("brew outdated: %w", err)
	}
	var res struct {
		Casks []struct {
			Name             string   `json:"name"`
			InstalledVersion []string `json:"installed_versions"`
			Current          string   `json:"current_version"`
			Pinned           bool     `json:"pinned"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, err
	}
	ups := make([]AppUpdate, 0, len(res.Casks))
	for _, c := range res.Casks {
		ups = append(ups, AppUpdate{
			Source:    "homebrew-cask",
			Name:      c.Name,
			Installed: c.InstalledVersion,
			Latest:    c.Current,
			Pinned:    c.Pinned,
		})
	}
	return ups, nil
}

// ScanMAS lists App Store updates via `mas outdated`. Returns nil,nil when
// mas is not installed (not an error — the channel is optional).
func (u *UpdateChecker) ScanMAS() ([]AppUpdate, error) {
	mas, err := exec.LookPath("mas")
	if err != nil {
		return nil, nil
	}
	out, err := exec.Command(mas, "outdated").Output()
	if err != nil {
		return nil, nil // mas exits non-zero when signed out
	}
	var ups []AppUpdate
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// Format: "123456789 AppName (1.2.3 -> 1.3.0)"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.Join(fields[1:len(fields)-1], " ")
		latest := strings.TrimSuffix(strings.TrimPrefix(fields[len(fields)-1], "("), ")")
		if i := strings.LastIndex(latest, "->"); i >= 0 {
			latest = strings.TrimSpace(latest[i+2:])
		}
		ups = append(ups, AppUpdate{Source: "appstore", Name: name, Latest: latest})
	}
	return ups, nil
}

// UpgradeBrew runs `brew upgrade --cask <name>`.
func (u *UpdateChecker) UpgradeBrew(name string) error {
	brew := brewPath()
	if brew == "" {
		return fmt.Errorf("未安装 Homebrew")
	}
	cmd := exec.Command(brew, "upgrade", "--cask", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// UpdateState is the UI state for the 更新 sub-tab.
type UpdateState struct {
	Updates   []AppUpdate
	Loading   bool
	Err       string
	Upgrading map[string]bool // names currently upgrading
	Done      map[string]bool // upgraded this session
}

func NewUpdateState() *UpdateState {
	return &UpdateState{Upgrading: map[string]bool{}, Done: map[string]bool{}}
}

// Scan refreshes the update list across channels.
func (s *UpdateState) Scan(onDone func()) {
	if s.Loading {
		return
	}
	s.Loading = true
	s.Err = ""
	go func() {
		c := &UpdateChecker{}
		var all []AppUpdate
		if ups, err := c.ScanBrew(); err != nil {
			s.Err = err.Error()
		} else {
			all = append(all, ups...)
		}
		if ups, _ := c.ScanMAS(); len(ups) > 0 {
			all = append(all, ups...)
		}
		s.Updates = all
		s.Loading = false
		if onDone != nil {
			onDone()
		}
	}()
}

// Upgrade runs the channel's updater for one item.
func (s *UpdateState) Upgrade(u AppUpdate, onDone func()) {
	if s.Upgrading[u.Name] {
		return
	}
	s.Upgrading[u.Name] = true
	go func() {
		var err error
		switch u.Source {
		case "homebrew-cask":
			err = (&UpdateChecker{}).UpgradeBrew(u.Name)
		default:
			err = fmt.Errorf("暂不支持 %s 渠道", u.Source)
		}
		delete(s.Upgrading, u.Name)
		if err != nil {
			s.Err = err.Error()
		} else {
			s.Done[u.Name] = true
		}
		if onDone != nil {
			onDone()
		}
	}()
}

// StartupItem is one auto-start entry.
type StartupItem struct {
	Name    string // plist label or login item name
	Path    string // plist file path (empty for login items)
	Kind    string // launchagent | loginitem
	Program string // resolved binary/script from the plist
	Enabled bool
	// Login items can't be mapped back to a file reliably; removal uses
	// System Events. LaunchAgents disable by moving the plist aside.
}

// StartupScanner enumerates user-level auto-start entries.
type StartupScanner struct {
	home string
}

func NewStartupScanner() *StartupScanner {
	h, _ := os.UserHomeDir()
	return &StartupScanner{home: h}
}

// Scan returns LaunchAgents (~/Library/LaunchAgents) + System Events login
// items. Disabled agents live in LaunchAgents/disabled/.
func (s *StartupScanner) Scan() []StartupItem {
	var out []StartupItem
	for _, dir := range []struct {
		path    string
		enabled bool
	}{
		{s.home + "/Library/LaunchAgents", true},
		{s.home + "/Library/LaunchAgents/disabled", false},
	} {
		ents, err := os.ReadDir(dir.path)
		if err != nil {
			continue
		}
		var entries []string
		for _, e := range ents {
			entries = append(entries, e.Name())
		}
		for _, e := range entries {
			if !strings.HasSuffix(e, ".plist") {
				continue
			}
			p := dir.path + "/" + e
			out = append(out, StartupItem{
				Name:    launchAgentLabel(p, e),
				Path:    p,
				Kind:    "launchagent",
				Program: plistProgram(p),
				Enabled: dir.enabled,
			})
		}
	}
	// Login items via System Events (works without accessibility rights on
	// most systems; silently skipped if denied).
	if out2, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get the name of every login item`).Output(); err == nil {
		for _, n := range strings.Split(strings.TrimSpace(string(out2)), ", ") {
			if n = strings.TrimSpace(n); n != "" {
				out = append(out, StartupItem{Name: n, Kind: "loginitem", Enabled: true})
			}
		}
	}
	return out
}

// launchAgentLabel reads the plist's Label key, falling back to filename.
func launchAgentLabel(path, filename string) string {
	out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :Label", path).Output()
	if err == nil {
		if l := strings.TrimSpace(string(out)); l != "" {
			return l
		}
	}
	return strings.TrimSuffix(filename, ".plist")
}

// plistProgram reads Program or the first ProgramArguments entry.
func plistProgram(path string) string {
	for _, key := range []string{"Print :Program", "Print :ProgramArguments:0"} {
		out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", key, path).Output()
		if err == nil {
			if p := strings.TrimSpace(string(out)); p != "" {
				return p
			}
		}
	}
	return ""
}

// SetEnabled moves a LaunchAgent plist in/out of the disabled/ dir, or
// toggles a login item through System Events.
func (it *StartupItem) SetEnabled(on bool) error {
	switch it.Kind {
	case "launchagent":
		if it.Path == "" {
			return fmt.Errorf("无文件路径")
		}
		disabledDir := filepath.Join(filepath.Dir(it.Path), "disabled")
		if on {
			// Move back out of disabled/.
			return os.Rename(filepath.Join(disabledDir, filepath.Base(it.Path)), it.Path)
		}
		if err := os.MkdirAll(disabledDir, 0o755); err != nil {
			return err
		}
		return os.Rename(it.Path, filepath.Join(disabledDir, filepath.Base(it.Path)))
	case "loginitem":
		verb := "delete"
		if on {
			return fmt.Errorf("登录项只能删除不能禁用；重新添加请到对应 App 设置")
		}
		return exec.Command("osascript", "-e",
			fmt.Sprintf(`tell application "System Events" to %s login item "%s"`, verb, it.Name)).Run()
	}
	return nil
}

// StartupState is the UI state for the 启动项 sub-tab.
type StartupState struct {
	Items   []StartupItem
	Loading bool
	Err     string
}

func (s *StartupState) Scan(onDone func()) {
	if s.Loading {
		return
	}
	s.Loading = true
	go func() {
		s.Items = NewStartupScanner().Scan()
		s.Loading = false
		if onDone != nil {
			onDone()
		}
	}()
}

// Toggle flips one item's enabled state, then refreshes.
func (s *StartupState) Toggle(i int, onDone func()) {
	if i < 0 || i >= len(s.Items) {
		return
	}
	it := &s.Items[i]
	go func() {
		if err := it.SetEnabled(!it.Enabled); err != nil {
			s.Err = err.Error()
		}
		s.Items = NewStartupScanner().Scan()
		if onDone != nil {
			onDone()
		}
	}()
}
