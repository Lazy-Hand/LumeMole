package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OptimizeTask is one maintenance action, translated from
// lib/optimize/catalog.sh.
type OptimizeTask struct {
	ID          string
	Name        string
	Description string
	NeedsSudo   bool
	// Run executes the task; dryRun reports what would happen.
	Run     func() (string, error)
	// Status returns a short state string, or "" if uncheckable.
	Status  func() string
	// DryRun describes the change without applying it.
	DryRun  func() string
}

// OptimizeState is the UI state for the optimize page.
type OptimizeState struct {
	Tasks     []OptimizeTask
	Running   map[string]bool
	Results   map[string]string
	LastErr   string
	RunningAt time.Time
}

// NewOptimizeState builds the task registry.
func NewOptimizeState() *OptimizeState {
	return &OptimizeState{
		Tasks:   optimizeTasks(),
		Running: map[string]bool{},
		Results: map[string]string{},
	}
}

// optimizeTasks is the Go port of the safe subset of mole's optimize catalog.
// Each returns a one-line result or error.
func optimizeTasks() []OptimizeTask {
	return []OptimizeTask{
		{
			ID:          "dns_flush",
			Name:        "DNS 缓存刷新",
			Description: "清空 DNS 解析缓存（dscacheutil + mDNSResponder）",
			Run: func() (string, error) {
				if err := exec.Command("dscacheutil", "-flushcache").Run(); err != nil {
					return "", err
				}
				_ = exec.Command("killall", "-HUP", "mDNSResponder").Run()
				return "DNS 缓存已刷新", nil
			},
			Status: func() string {
				out, _ := exec.Command("scutil", "--dns").Output()
				if len(out) > 0 {
					return "运行中"
				}
				return ""
			},
		},
		{
			ID:          "quicklook_cache",
			Name:        "QuickLook 缩略图缓存",
			Description: "删除 QuickLook 缩略图与图标服务缓存（自动重建）",
			Run: func() (string, error) {
				home, _ := os.UserHomeDir()
				targets := []string{
					filepath.Join(home, "Library/Caches/com.apple.QuickLook.thumbnailcache"),
					filepath.Join(home, "Library/Caches/com.apple.iconservices.store"),
					filepath.Join(home, "Library/Caches/com.apple.iconservices"),
				}
				var removed int
				for _, t := range targets {
					if err := os.RemoveAll(t); err == nil {
						removed++
					}
				}
				_ = exec.Command("qlmanage", "-r", "cache").Run()
				return fmt.Sprintf("已清理 %d 项，缩略图将自动重建", removed), nil
			},
		},
		{
			ID:          "saved_state",
			Name:        "应用状态清理",
			Description: "删除 30 天前未使用的应用保存状态（Saved Application State）",
			Run: func() (string, error) {
				dir := filepath.Join(os.Getenv("HOME"), "Library", "Saved Application State")
				cutoff := time.Now().Add(-30 * 24 * time.Hour)
				var removed int
				entries, _ := os.ReadDir(dir)
				for _, e := range entries {
					if !strings.HasSuffix(e.Name(), ".savedState") {
						continue
					}
					fi, err := e.Info()
					if err != nil || fi.ModTime().After(cutoff) {
						continue
					}
					if os.RemoveAll(filepath.Join(dir, e.Name())) == nil {
						removed++
					}
				}
				return fmt.Sprintf("已清理 %d 个过期状态", removed), nil
			},
		},
		{
			ID:          "dsstore_network",
			Name:        "网络卷禁用 .DS_Store",
			Description: "阻止 Finder 在 SMB/AFP/NFS/USB 卷上创建 .DS_Store",
			Run: func() (string, error) {
				if err := exec.Command("defaults", "write", "com.apple.desktopservices",
					"DSDontWriteNetworkStores", "-bool", "true").Run(); err != nil {
					return "", err
				}
				return "已设置：网络卷不再写入 .DS_Store", nil
			},
			Status: func() string {
				out, _ := exec.Command("defaults", "read", "com.apple.desktopservices",
					"DSDontWriteNetworkStores").Output()
				if strings.TrimSpace(string(out)) == "1" {
					return "已启用"
				}
				return "未启用"
			},
		},
		{
			ID:          "notification_clean",
			Name:        "通知中心清理",
			Description: "清除旧通知记录，减少数据库膨胀",
			Run: func() (string, error) {
				dir := filepath.Join(os.Getenv("HOME"), "Library",
					"Application Support", "NotificationCenter")
				entries, _ := os.ReadDir(dir)
				var removed int
				for _, e := range entries {
					if strings.HasSuffix(e.Name(), ".db") ||
						strings.HasSuffix(e.Name(), ".db-wal") ||
						strings.HasSuffix(e.Name(), ".db-shm") {
						if os.Remove(filepath.Join(dir, e.Name())) == nil {
							removed++
						}
					}
				}
				return fmt.Sprintf("已清理 %d 个通知数据库文件", removed), nil
			},
		},
		{
			ID:          "quarantine_clean",
			Name:        "下载隔离数据库",
			Description: "清空 Gatekeeper 下载追踪历史",
			Run: func() (string, error) {
				dir := filepath.Join(os.Getenv("HOME"), "Library",
					"Preferences", "com.apple.LaunchServices.QuarantineEventsV2")
				if err := os.RemoveAll(dir); err != nil {
					return "", err
				}
				return "已清空下载隔离记录", nil
			},
		},
		{
			ID:          "login_items_audit",
			Name:        "登录项审计",
			Description: "列出失效的登录项与 LaunchAgents（只读）",
			Run: func() (string, error) {
				out, err := exec.Command("osascript", "-e",
					`tell application "System Events" to get the name of every login item`).Output()
				if err != nil {
					return "", err
				}
				items := strings.TrimSpace(string(out))
				if items == "" {
					items = "无"
				}
				return "登录项: " + items, nil
			},
		},
		{
			ID:          "orphan_launchagents",
			Name:        "孤儿 LaunchAgents",
			Description: "检测指向不存在程序的 LaunchAgent plist（只读）",
			Run: func() (string, error) {
				home, _ := os.UserHomeDir()
				dir := filepath.Join(home, "Library", "LaunchAgents")
				entries, _ := os.ReadDir(dir)
				var orphans []string
				for _, e := range entries {
					if !strings.HasSuffix(e.Name(), ".plist") {
						continue
					}
					p := filepath.Join(dir, e.Name())
					out, _ := exec.Command("plutil", "-convert", "json", "-o", "-", p).Output()
					if len(out) == 0 {
						continue
					}
					prog := extractJSON(out, "Program")
					if prog == "" {
						prog = extractJSON(out, "ProgramArguments")
					}
					if prog != "" {
						if _, err := os.Stat(prog); os.IsNotExist(err) {
							orphans = append(orphans, e.Name())
						}
					}
				}
				if len(orphans) == 0 {
					return "未发现孤儿 LaunchAgents", nil
				}
				return fmt.Sprintf("发现 %d 个孤儿项: %s", len(orphans), strings.Join(orphans, ", ")), nil
			},
		},
	}
}

// RunTask executes one task. onDone runs via win.Update.
func (s *OptimizeState) RunTask(id string, onDone func()) {
	if s.Running[id] {
		return
	}
	s.Running[id] = true
	go func() {
		var msg string
		var err error
		for _, t := range s.Tasks {
			if t.ID == id {
				msg, err = t.Run()
				break
			}
		}
		if err != nil {
			s.Results[id] = "失败: " + err.Error()
		} else {
			s.Results[id] = msg
		}
		delete(s.Running, id)
		if onDone != nil {
			onDone()
		}
	}()
}
