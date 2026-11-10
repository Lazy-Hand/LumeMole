package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DoctorIssue is one thing worth the user's attention.
type DoctorIssue struct {
	Severity string // warn | info
	Title    string
	Detail   string
	Action   string // page hint: "清理页" "状态页" "设置" etc.
}

// DoctorSection is a named group of key-value rows.
type DoctorSection struct {
	Title string
	Rows  []DoctorKV
}

// DoctorKV is one label/value line.
type DoctorKV struct{ Key, Value string }

// DoctorReport is the full diagnostic output.
type DoctorReport struct {
	Issues   []DoctorIssue
	Sections []DoctorSection
	RunAt    time.Time
}

// RunDoctor collects health checks + system info. fda is the app's
// probed Full Disk Access state.
func RunDoctor(fda bool) *DoctorReport {
	r := &DoctorReport{RunAt: time.Now()}
	r.checkIssues(fda)
	r.collectSections(fda)
	return r
}

// --- issues ---

func (r *DoctorReport) checkIssues(fda bool) {
	if !fda {
		r.Issues = append(r.Issues, DoctorIssue{
			Severity: "warn", Title: "未开启完全磁盘访问",
			Detail: "部分项目（~/Library、废纸篓）无法扫描或清理", Action: "设置",
		})
	}

	// Disk pressure on /.
	if out, err := exec.Command("df", "-k", "/").Output(); err == nil {
		var pct int
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "/dev/") {
				f := strings.Fields(line)
				if len(f) >= 5 {
					fmt.Sscanf(strings.TrimSuffix(f[4], "%"), "%d", &pct)
				}
			}
		}
		if pct >= 90 {
			r.Issues = append(r.Issues, DoctorIssue{
				Severity: "warn", Title: fmt.Sprintf("启动磁盘已用 %d%%", pct),
				Detail: "建议清理释放空间", Action: "清理页",
			})
		}
	}

	// Memory pressure via memory_pressure status.
	if out, err := exec.Command("memory_pressure").Output(); err == nil {
		if strings.Contains(string(out), "warn") || strings.Contains(string(out), "critical") {
			r.Issues = append(r.Issues, DoctorIssue{
				Severity: "warn", Title: "内存压力偏高",
				Detail: "关闭不用的 App 会缓解", Action: "状态页",
			})
		}
	}

	// Battery health via ioreg (laptops only).
	if out, err := exec.Command("ioreg", "-rn", "AppleSmartBattery").Output(); err == nil {
		var maxCap, designCap, cycles int
		for _, line := range strings.Split(string(out), "\n") {
			switch {
			case strings.Contains(line, `"MaxCapacity"`):
				fmt.Sscanf(line, `"MaxCapacity" = %d`, &maxCap)
			case strings.Contains(line, `"DesignCapacity"`):
				fmt.Sscanf(line, `"DesignCapacity" = %d`, &designCap)
			case strings.Contains(line, `"CycleCount"`):
				fmt.Sscanf(line, `"CycleCount" = %d`, &cycles)
			}
		}
		if designCap > 0 {
			health := maxCap * 100 / designCap
			if health < 80 {
				r.Issues = append(r.Issues, DoctorIssue{
					Severity: "warn", Title: fmt.Sprintf("电池健康度 %d%%", health),
					Detail: fmt.Sprintf("循环 %d 次，可去 Apple Store 检测", cycles), Action: "状态页",
				})
			} else if cycles > 800 {
				r.Issues = append(r.Issues, DoctorIssue{
					Severity: "info", Title: fmt.Sprintf("电池已循环 %d 次", cycles),
					Detail: fmt.Sprintf("健康度 %d%%", health), Action: "状态页",
				})
			}
		}
	}

	// Uptime: suggest a reboot past 14d.
	if out, err := exec.Command("sysctl", "-n", "kern.boottime").Output(); err == nil {
		var sec int64
		fmt.Sscanf(string(out), "sec = %d", &sec)
		if sec > 0 {
			days := int(time.Since(time.Unix(sec, 0)).Hours() / 24)
			if days >= 14 {
				r.Issues = append(r.Issues, DoctorIssue{
					Severity: "info", Title: fmt.Sprintf("已开机 %d 天", days),
					Detail: "重启通常能释放泄漏的资源", Action: "状态页",
				})
			}
		}
	}

	// Trash over 5 GB (skip when unreadable — FDA covers this).
	home, _ := os.UserHomeDir()
	if ents, err := os.ReadDir(filepath.Join(home, ".Trash")); err == nil {
		var n int64
		for _, e := range ents {
			n += dirSizeQuiet(filepath.Join(home, ".Trash", e.Name()))
		}
		if n > 5<<30 {
			r.Issues = append(r.Issues, DoctorIssue{
				Severity: "warn", Title: fmt.Sprintf("废纸篓已超过 %s", FmtBytes(n)),
				Detail: "可以在清理页清空并释放空间", Action: "清理页",
			})
		}
	}

	// Orphan LaunchAgents (same check as the optimize task, surfaced here).
	if orphans := orphanLaunchAgents(); len(orphans) > 0 {
		r.Issues = append(r.Issues, DoctorIssue{
			Severity: "info", Title: fmt.Sprintf("%d 个失效后台服务", len(orphans)),
			Detail: "LaunchAgent 指向已删除的程序: " + strings.Join(orphans[:minI(3, len(orphans))], ", "),
			Action: "优化页",
		})
	}
}

// orphanLaunchAgents lists plists whose Program no longer exists.
func orphanLaunchAgents() []string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Library", "LaunchAgents")
	ents, _ := os.ReadDir(dir)
	var out []string
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, _ := exec.Command("plutil", "-convert", "json", "-o", "-", p).Output()
		if len(data) == 0 {
			continue
		}
		prog := extractJSON(data, "Program")
		if prog == "" {
			if pa := extractJSON(data, "ProgramArguments"); strings.HasPrefix(pa, "[") {
				// extractJSON only handles strings; grab the first array
				// element crudely.
				if i := strings.Index(pa, `"`); i >= 0 {
					rest := pa[i+1:]
					if j := strings.Index(rest, `"`); j >= 0 {
						prog = rest[:j]
					}
				}
			}
		}
		if prog != "" {
			if _, err := os.Stat(prog); os.IsNotExist(err) {
				out = append(out, strings.TrimSuffix(e.Name(), ".plist"))
			}
		}
	}
	return out
}

// --- sections ---

func (r *DoctorReport) collectSections(fda bool) {
	home, _ := os.UserHomeDir()

	// 权限
	var permRows []DoctorKV
	fdaStr := "未授权"
	if fda {
		fdaStr = "已授权"
	}
	permRows = append(permRows, DoctorKV{"完全磁盘访问", fdaStr})
	if _, err := os.ReadDir(filepath.Join(home, ".Trash")); err == nil {
		permRows = append(permRows, DoctorKV{"废纸篓", "可读"})
	} else {
		permRows = append(permRows, DoctorKV{"废纸篓", "不可读"})
	}
	r.Sections = append(r.Sections, DoctorSection{"权限", permRows})

	// 系统
	var sysRows []DoctorKV
	if out, err := exec.Command("sw_vers").Output(); err == nil {
		var prod, ver, build string
		for _, l := range strings.Split(string(out), "\n") {
			l = strings.TrimSpace(l)
			switch {
			case strings.HasPrefix(l, "ProductName:"):
				prod = strings.TrimSpace(strings.TrimPrefix(l, "ProductName:"))
			case strings.HasPrefix(l, "ProductVersion:"):
				ver = strings.TrimSpace(strings.TrimPrefix(l, "ProductVersion:"))
			case strings.HasPrefix(l, "BuildVersion:"):
				build = strings.TrimSpace(strings.TrimPrefix(l, "BuildVersion:"))
			}
		}
		sysRows = append(sysRows, DoctorKV{"系统", fmt.Sprintf("%s %s (%s)", prod, ver, build)})
	}
	if out, err := exec.Command("sysctl", "-n", "hw.model").Output(); err == nil {
		sysRows = append(sysRows, DoctorKV{"机型", strings.TrimSpace(string(out))})
	}
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		var bytes int64
		fmt.Sscanf(string(out), "%d", &bytes)
		sysRows = append(sysRows, DoctorKV{"内存", FmtBytes(bytes)})
	}
	if out, err := exec.Command("uptime").Output(); err == nil {
		sysRows = append(sysRows, DoctorKV{"运行时长", strings.TrimSpace(string(out))})
	}
	r.Sections = append(r.Sections, DoctorSection{"系统", sysRows})

	// 配置
	var cfgRows []DoctorKV
	cfgRows = append(cfgRows,
		DoctorKV{"白名单", configFileState(filepath.Join(home, ".config", "mole", "whitelist"))},
		DoctorKV{"清理清单", configFileState(filepath.Join(home, ".config", "mole", "clean-list.txt"))},
		DoctorKV{"操作日志", configFileState(filepath.Join(home, "Library", "Logs", "mole", "operations.log"))},
	)
	r.Sections = append(r.Sections, DoctorSection{"配置与日志", cfgRows})

	// Mole 运行状态
	var runRows []DoctorKV
	runRows = append(runRows, DoctorKV{"LumeMole", "v0.1.1"})
	if FindStatusProvider() != nil {
		runRows = append(runRows, DoctorKV{"status-go", "可用"})
	} else {
		runRows = append(runRows, DoctorKV{"status-go", "未找到"})
	}
	if FindCleanProvider() != nil {
		runRows = append(runRows, DoctorKV{"mole CLI", "可用"})
	} else {
		runRows = append(runRows, DoctorKV{"mole CLI", "未找到"})
	}
	if FindAnalyzeProvider() != nil {
		runRows = append(runRows, DoctorKV{"analyze-go", "可用"})
	} else {
		runRows = append(runRows, DoctorKV{"analyze-go", "未找到"})
	}
	r.Sections = append(r.Sections, DoctorSection{"LumeMole 运行状态", runRows})
}

func configFileState(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "不存在"
	}
	return fmt.Sprintf("%s · 更新于 %s", FmtBytes(st.Size()), st.ModTime().Format("01-02 15:04"))
}

// Text renders the report as plain text for clipboard copy.
func (r *DoctorReport) Text() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("LumeMole 诊断报告 · %s\n\n", r.RunAt.Format("2006-01-02 15:04")))
	if len(r.Issues) > 0 {
		b.WriteString("== 待处理项 ==\n")
		for _, i := range r.Issues {
			b.WriteString(fmt.Sprintf("[%s] %s — %s\n", i.Severity, i.Title, i.Detail))
		}
		b.WriteString("\n")
	}
	for _, s := range r.Sections {
		b.WriteString("== " + s.Title + " ==\n")
		for _, kv := range s.Rows {
			b.WriteString(fmt.Sprintf("  %s: %s\n", kv.Key, kv.Value))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// DoctorState is the UI state for the doctor page.
type DoctorState struct {
	Report  *DoctorReport
	Running bool
	FDA     bool // copied from app.Permissions at page open
}

// Run refreshes the report.
func (s *DoctorState) Run(onDone func()) {
	if s.Running {
		return
	}
	s.Running = true
	go func() {
		s.Report = RunDoctor(s.FDA)
		s.Running = false
		if onDone != nil {
			onDone()
		}
	}()
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}
