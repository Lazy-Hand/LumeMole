package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// StatusSnapshot is one sample from `status-go --json` (or a --watch line).
// Field names follow status-go's JSON keys exactly.
type StatusSnapshot struct {
	CollectedAt    time.Time `json:"collected_at"`
	Host           string    `json:"host"`
	Platform       string    `json:"platform"`
	Uptime         string    `json:"uptime"`
	UptimeSeconds  int64     `json:"uptime_seconds"`
	Procs          int       `json:"procs"`
	Hardware       Hardware  `json:"hardware"`
	HealthScore    int       `json:"health_score"`
	HealthScoreMsg string    `json:"health_score_msg"`
	CPU            CPUInfo   `json:"cpu"`
	GPU            []GPUInfo `json:"gpu"`
	Memory         MemInfo   `json:"memory"`
	Disks          []DiskInfo `json:"disks"`
	DiskIO         DiskIO    `json:"disk_io"`
	TrashSize      int64     `json:"trash_size"`
	Network        []NetInfo `json:"network"`
	NetworkHistory NetHist   `json:"network_history"`
	Proxy          ProxyInfo `json:"proxy"`
	Batteries      []Battery `json:"batteries"`
	Thermal        Thermal   `json:"thermal"`
	Bluetooth      []BTDev   `json:"bluetooth"`
	TopProcesses   []Process `json:"top_processes"`
	ZombieCount    int       `json:"zombie_count"`
}

type Hardware struct {
	Model       string `json:"model"`
	CPUModel    string `json:"cpu_model"`
	TotalRAM    string `json:"total_ram"`
	DiskSize    string `json:"disk_size"`
	OSVersion   string `json:"os_version"`
	RefreshRate string `json:"refresh_rate"`
}

type CPUInfo struct {
	Usage            float64   `json:"usage"`
	PerCore          []float64 `json:"per_core"`
	PerCoreEstimated bool      `json:"per_core_estimated"`
	Load1            float64   `json:"load1"`
	Load5            float64   `json:"load5"`
	Load15           float64   `json:"load15"`
	CoreCount        int       `json:"core_count"`
	LogicalCPU       int       `json:"logical_cpu"`
	PCoreCount       int       `json:"p_core_count"`
	ECoreCount       int       `json:"e_core_count"`
}

type GPUInfo struct {
	Name        string  `json:"name"`
	Usage       float64 `json:"usage"` // -1 when unavailable
	MemoryUsed  int64   `json:"memory_used"`
	MemoryTotal int64   `json:"memory_total"`
	CoreCount   int     `json:"core_count"`
	Note        string  `json:"note"`
}

type MemInfo struct {
	Used        int64   `json:"used"`
	Total       int64   `json:"total"`
	Available   int64   `json:"available"`
	UsedPercent float64 `json:"used_percent"`
	SwapUsed    int64   `json:"swap_used"`
	SwapTotal   int64   `json:"swap_total"`
	Cached      int64   `json:"cached"`
	Pressure    string  `json:"pressure"`
}

type DiskInfo struct {
	Mount       string  `json:"mount"`
	Device      string  `json:"device"`
	Used        int64   `json:"used"`
	Total       int64   `json:"total"`
	UsedPercent float64 `json:"used_percent"`
	FSType      string  `json:"fstype"`
	External    bool    `json:"external"`
	SMARTStatus string  `json:"smart_status"`
	Purgeable   int64   `json:"purgeable"`
}

type DiskIO struct {
	ReadRate  float64 `json:"read_rate"`
	WriteRate float64 `json:"write_rate"`
}

type NetInfo struct {
	Name      string  `json:"name"`
	RxRateMBs float64 `json:"rx_rate_mbs"`
	TxRateMBs float64 `json:"tx_rate_mbs"`
	IP        string  `json:"ip"`
}

type NetHist struct {
	RxHistory []float64 `json:"rx_history"`
	TxHistory []float64 `json:"tx_history"`
}

type ProxyInfo struct {
	Enabled bool   `json:"enabled"`
	Type    string `json:"type"`
	Host    string `json:"host"`
}

type Battery struct {
	Percent    int    `json:"percent"`
	Status     string `json:"status"`
	TimeLeft   string `json:"time_left"`
	Health     string `json:"health"`
	CycleCount int    `json:"cycle_count"`
	Capacity   int    `json:"capacity"`
}

type Thermal struct {
	CPUTemp      float64 `json:"cpu_temp"`
	GPUTemp      float64 `json:"gpu_temp"`
	BatteryTemp  float64 `json:"battery_temp"`
	FanSpeed     float64 `json:"fan_speed"`
	FanCount     int     `json:"fan_count"`
	SystemPower  float64 `json:"system_power"`
	AdapterPower float64 `json:"adapter_power"`
	BatteryPower float64 `json:"battery_power"`
}

type BTDev struct {
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Battery   int    `json:"battery"`
}

type Process struct {
	PID         int     `json:"pid"`
	PPID        int     `json:"ppid"`
	Name        string  `json:"name"`
	Command     string  `json:"command"`
	CPU         float64 `json:"cpu"`
	Memory      float64 `json:"memory"`
	MemoryBytes int64   `json:"memory_bytes"`
}

// StatusProvider returns status snapshots, either by exec'ing status-go or
// (later) collecting natively.
type StatusProvider interface {
	// Snapshot returns one sample.
	Snapshot() (*StatusSnapshot, error)
	// Watch streams snapshots every interval until stop is closed or cmd exits.
	// Each decoded snapshot is sent on the returned channel; channel closes on error.
	Watch(interval time.Duration, stop <-chan struct{}) (<-chan *StatusSnapshot, <-chan error)
	// Available reports whether the provider can serve data.
	Available() bool
}

// MoleStatusProvider shells out to mole's bundled status-go.
type MoleStatusProvider struct {
	bin string // path to status-go binary
}

// FindStatusProvider locates status-go: PATH `mo` → its libexec dir → status-go.
// Returns nil when mole is not installed.
func FindStatusProvider() *MoleStatusProvider {
	mo, err := exec.LookPath("mo")
	if err != nil {
		mo, err = exec.LookPath("mole")
		if err != nil {
			return nil
		}
	}
	// mo may be a symlink into Cellar; resolve to find libexec next to it.
	real, err := filepath.EvalSymlinks(mo)
	if err != nil {
		real = mo
	}
	// Homebrew layout: <cellar>/bin/mo → libexec/bin/status-go.
	cand := filepath.Join(filepath.Dir(real), "..", "libexec", "bin", "status-go")
	if _, err := os.Stat(cand); err == nil {
		return &MoleStatusProvider{bin: cand}
	}
	// Direct sibling (non-standard installs).
	cand = filepath.Join(filepath.Dir(real), "status-go")
	if _, err := os.Stat(cand); err == nil {
		return &MoleStatusProvider{bin: cand}
	}
	return nil
}

func (p *MoleStatusProvider) Available() bool { return p != nil && p.bin != "" }

func (p *MoleStatusProvider) Snapshot() (*StatusSnapshot, error) {
	out, err := exec.Command(p.bin, "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("status-go --json: %w", err)
	}
	var s StatusSnapshot
	if err := json.Unmarshal(out, &s); err != nil {
		return nil, fmt.Errorf("decode status-go: %w", err)
	}
	return &s, nil
}

func (p *MoleStatusProvider) Watch(interval time.Duration, stop <-chan struct{}) (<-chan *StatusSnapshot, <-chan error) {
	out := make(chan *StatusSnapshot, 4)
	errs := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errs)

		args := []string{"--watch", "--interval", fmt.Sprintf("%dms", interval.Milliseconds())}
		cmd := exec.Command(p.bin, args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errs <- err
			return
		}
		if err := cmd.Start(); err != nil {
			errs <- err
			return
		}
		defer func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}()

		// Stop watcher when stop closes.
		go func() {
			<-stop
			_ = cmd.Process.Kill()
		}()

		dec := json.NewDecoder(stdout)
		for {
			var s StatusSnapshot
			if err := dec.Decode(&s); err != nil {
				// EOF or killed — not an error to surface.
				return
			}
			select {
			case out <- &s:
			case <-stop:
				return
			}
		}
	}()
	return out, errs
}

// StatusState is the UI-facing state for the status page.
type StatusState struct {
	Snapshot    *StatusSnapshot
	Provider    StatusProvider
	Available   bool
	LastErr     string
	StopWatch   chan struct{}
	watching    bool

	// Histories for sparklines, kept across frames.
	MemHist  []float64
	CPUHist  []float64
	RxHist   []float64
	TxHist   []float64
}

const histCap = 60

// NewStatusState builds the state, probing for status-go.
func NewStatusState() *StatusState {
	p := FindStatusProvider()
	return &StatusState{Provider: p, Available: p != nil}
}

// StartWatch begins streaming snapshots into state. Call on main thread.
func (s *StatusState) StartWatch(update func()) {
	if s.watching || !s.Available {
		return
	}
	s.watching = true
	s.StopWatch = make(chan struct{})
	snaps, errs := s.Provider.Watch(2*time.Second, s.StopWatch)

	go func() {
		for {
			select {
			case snap, ok := <-snaps:
				if !ok {
					return
				}
				if update != nil {
					// update() is expected to run on the main thread via win.Update.
					s.Snapshot = snap
					s.pushHist(snap)
					update()
				}
			case err, ok := <-errs:
				if !ok {
					return
				}
				if err != nil {
					s.LastErr = err.Error()
				}
			}
		}
	}()
}

// Stop ends the watch goroutine.
func (s *StatusState) Stop() {
	if s.StopWatch != nil {
		close(s.StopWatch)
		s.StopWatch = nil
	}
	s.watching = false
}

func (s *StatusState) pushHist(snap *StatusSnapshot) {
	s.MemHist = appendCapped(s.MemHist, snap.Memory.UsedPercent, histCap)
	s.CPUHist = appendCapped(s.CPUHist, snap.CPU.Usage, histCap)
	var rx, tx float64
	for _, n := range snap.Network {
		rx += n.RxRateMBs
		tx += n.TxRateMBs
	}
	s.RxHist = appendCapped(s.RxHist, rx, histCap)
	s.TxHist = appendCapped(s.TxHist, tx, histCap)
}

func appendCapped(s []float64, v float64, cap int) []float64 {
	s = append(s, v)
	if len(s) > cap {
		copy(s, s[len(s)-cap:])
		s = s[:cap]
	}
	return s
}

// ---- Formatting helpers ----

// FmtBytes formats a byte count as human-readable (e.g. "13.2 GB").
func FmtBytes(b int64) string {
	const u = 1024
	if b < u {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(u), 0
	for n := b / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// FmtMBs formats a MB/s rate adaptively (KB/s below 1 MB/s).
func FmtMBs(m float64) string {
	if m < 0.001 {
		return "0 KB/s"
	}
	if m < 1 {
		return fmt.Sprintf("%.0f KB/s", m*1024)
	}
	return fmt.Sprintf("%.1f MB/s", m)
}

// FmtPct formats a 0-100 percent value.
func FmtPct(p float64) string { return fmt.Sprintf("%.0f%%", p) }

// HealthLabel maps score → Chinese label, matching Mole's tiers.
func HealthLabel(score int) string {
	switch {
	case score >= 90:
		return "很好"
	case score >= 75:
		return "良好"
	case score >= 60:
		return "一般"
	default:
		return "需关注"
	}
}

// HealthColorHex maps score → hex color.
func HealthColorHex(score int) string {
	switch {
	case score >= 90:
		return "#3ecfa8"
	case score >= 75:
		return "#8bd06a"
	case score >= 60:
		return "#e0c53c"
	default:
		return "#e06c3c"
	}
}

// PrimaryDisk returns the boot volume.
func (s *StatusSnapshot) PrimaryDisk() *DiskInfo {
	for i := range s.Disks {
		if s.Disks[i].Mount == "/" {
			return &s.Disks[i]
		}
	}
	if len(s.Disks) > 0 {
		return &s.Disks[0]
	}
	return nil
}

// TotalNetRate sums all interfaces.
func (s *StatusSnapshot) TotalNetRate() (rx, tx float64) {
	for _, n := range s.Network {
		rx += n.RxRateMBs
		tx += n.TxRateMBs
	}
	return
}

// FirstBattery returns the main battery or nil.
func (s *StatusSnapshot) FirstBattery() *Battery {
	if len(s.Batteries) > 0 {
		return &s.Batteries[0]
	}
	return nil
}

// PrimaryGPU returns the first GPU or nil.
func (s *StatusSnapshot) PrimaryGPU() *GPUInfo {
	if len(s.GPU) > 0 {
		return &s.GPU[0]
	}
	return nil
}

// NetRateString formats total up/down.
func (s *StatusSnapshot) NetRateString() string {
	rx, tx := s.TotalNetRate()
	return fmt.Sprintf("↓%s ↑%s", FmtMBs(rx), FmtMBs(tx))
}
