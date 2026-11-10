package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DeleteResult is the outcome of one deletion pass.
type DeleteResult struct {
	Trashed    []string // user-domain paths moved to Trash
	Skipped    []DeleteSkip
	FreedBytes int64
}

// DeleteSkip is one path that was not removed, with why.
type DeleteSkip struct {
	Path   string
	Reason string
}

// criticalRoots are paths whose own deletion would damage the system —
// deny on exact match only. Their children are judged by allowedPrefixes
// (mole's explicit allowlist) or by user-domain heuristics.
var criticalRoots = []string{
	"/", "/bin", "/dev", "/sbin", "/usr", "/System", "/Library",
	"/Library/Apple", "/Library/Application Support", "/Library/Extensions",
	"/Library/Keychains", "/Applications", "/Volumes", "/opt",
	"/Users", "/Users/Shared", "/Users/Guest",
	"/private", "/private/tmp", "/private/var", "/private/var/tmp",
	"/private/var/folders", "/private/var/db", "/private/var/audit",
	"/private/var/root", "/etc", "/private/etc", "/var",
}

// allowedPrefixes are mole's explicit safe-clean zones under critical
// parents. Anything else under a critical root stays critical.
var allowedPrefixes = []string{
	"/usr/local", "/opt/homebrew",
	"/private/tmp", "/private/var/tmp", "/private/var/log",
	"/private/var/folders",
	"/private/var/db/diagnostics", "/private/var/db/DiagnosticPipeline",
	"/private/var/db/powerlog", "/private/var/db/reportmemoryexception",
	"/private/var/db/receipts",
}

// ValidatePath is the lightweight deny-first check before deleting.
// It is NOT a port of validate_path_for_deletion — the dry-run list already
// passed mole's own guards — but it catches hand-edited or stale paths that
// would be catastrophic to rm.
func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("空路径")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("非绝对路径: %s", path)
	}
	// Path traversal: reject /../ /.. ../ as whole components only.
	for _, seg := range strings.Split(path, "/") {
		if seg == ".." {
			return fmt.Errorf("包含 .. 穿越: %s", path)
		}
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("包含控制字符: %s", path)
		}
	}
	// Normalize double slashes and trailing slash.
	norm := normalizePolicyPath(path)

	// Leaf symlink: resolve and re-check the target against critical roots.
	if fi, err := os.Lstat(norm); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if tgt, err := filepath.EvalSymlinks(norm); err == nil {
			if isCritical(tgt) {
				return fmt.Errorf("符号链接指向受保护路径: %s -> %s", path, tgt)
			}
		}
	}
	// Ancestor symlink: canonicalize parent, deny if resolved path is critical.
	parent := filepath.Dir(norm)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil && resolved != parent {
		if isCritical(filepath.Join(resolved, filepath.Base(norm))) {
			return fmt.Errorf("解析后指向受保护路径: %s -> %s", path, resolved)
		}
	}
	if isCritical(norm) {
		return fmt.Errorf("系统关键路径: %s", path)
	}
	return nil
}

// normalizePolicyPath collapses //, /./, trailing /. Mirrors
// _mole_normalize_deletion_policy_path minus the Data-volume check.
func normalizePolicyPath(p string) string {
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	for strings.Contains(p, "/./") {
		p = strings.ReplaceAll(p, "/./", "/")
	}
	p = strings.TrimSuffix(p, "/.")
	if p == "" {
		p = "/"
	}
	return strings.TrimSuffix(p, "/")
}

// isCritical reports whether path is a critical root itself or sits inside
// a critical root's tree without falling under an allowed prefix. A user's
// home root (one component under /Users) is also critical.
func isCritical(p string) bool {
	p = normalizePolicyPath(p)
	// Home root rejection: exactly one component under /Users.
	if strings.HasPrefix(p, "/Users/") && !strings.Contains(p[len("/Users/"):], "/") {
		return true
	}
	// Explicit allowlist wins — these trees under critical parents are
	// designated safe-clean zones by mole.
	for _, allowed := range allowedPrefixes {
		if p == allowed || strings.HasPrefix(p, allowed+"/") {
			return false
		}
	}
	for _, root := range criticalRoots {
		if p == root {
			return true
		}
		// Inside a critical tree: allowed only for /Users/* (home dirs are
		// user domain) and /usr/local /opt/homebrew (already covered by the
		// allowlist above). Everything else under a critical root stays
		// critical.
		if strings.HasPrefix(p, root+"/") {
			if root == "/Users" {
				continue // home dir contents are user domain
			}
			return true
		}
	}
	return false
}

// isWhitelisted reports whether path matches the shared mole whitelist.
// Mirrors the whitelist format: plain prefix match on normalized path.
func isWhitelisted(path, whitelistFile string) bool {
	data, err := os.ReadFile(whitelistFile)
	if err != nil {
		return false
	}
	norm := normalizePolicyPath(path)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = normalizePolicyPath(line)
		if norm == line || strings.HasPrefix(norm, line+"/") {
			return true
		}
	}
	return false
}

// trashViaFinder moves path to the user Trash via Finder.
func trashViaFinder(path string) error {
	q := strings.ReplaceAll(path, `\`, `\\`)
	q = strings.ReplaceAll(q, `"`, `\"`)
	return exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application "Finder" to delete POSIX file "%s"`, q)).Run()
}

// pathIdentity is (dev, inode) for TOCTOU re-checking.
type pathIdentity struct{ dev, ino uint64 }

func statIdentity(path string) (pathIdentity, bool) {
	out, err := exec.Command("stat", "-f", "%d:%i", path).Output()
	if err != nil {
		return pathIdentity{}, false
	}
	var id pathIdentity
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d:%d", &id.dev, &id.ino)
	return id, true
}

// DeletePaths removes each path: validate → whitelist → identity recheck →
// Trash (user domain). NeedsSudo entries are skipped with a reason.
func DeletePaths(paths []string, whitelistFile string, onProgress func(done, total int, cur string)) *DeleteResult {
	res := &DeleteResult{}
	total := len(paths)
	for i, p := range paths {
		if onProgress != nil {
			onProgress(i, total, p)
		}
		if err := ValidatePath(p); err != nil {
			res.Skipped = append(res.Skipped, DeleteSkip{p, err.Error()})
			continue
		}
		if isWhitelisted(p, whitelistFile) {
			res.Skipped = append(res.Skipped, DeleteSkip{p, "白名单保护"})
			continue
		}
		// Identity check before delete — guards against the file changing
		// between scan and removal.
		before, ok1 := statIdentity(p)
		if !ok1 {
			res.Skipped = append(res.Skipped, DeleteSkip{p, "路径已不存在"})
			continue
		}
		// Needs sudo → skip, don't try to escalate here.
		if needsSudo(p) {
			res.Skipped = append(res.Skipped, DeleteSkip{p, "需要管理员权限（在终端运行 mo clean）"})
			continue
		}
		if err := trashViaFinder(p); err != nil {
			res.Skipped = append(res.Skipped, DeleteSkip{p, "移入废纸篓失败: " + err.Error()})
			continue
		}
		// Confirm it's gone and the same inode we intended.
		if after, ok2 := statIdentity(p); ok2 && after == before {
			res.Skipped = append(res.Skipped, DeleteSkip{p, "删除后仍存在"})
			continue
		}
		res.Trashed = append(res.Trashed, p)
	}
	if onProgress != nil {
		onProgress(total, total, "")
	}
	return res
}

// needsSudo is a coarse test: system-domain paths the current user cannot
// trash. Mirrors mole's user-vs-system split.
func needsSudo(p string) bool {
	norm := normalizePolicyPath(p)
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(norm, home) {
		return false
	}
	// Outside home → system domain. /private/var/folders/<u> is the user's
	// own Darwin temp, allowed; the rest of /private/var needs root.
	if strings.HasPrefix(norm, "/private/var/folders/") {
		// per-user subfolder: .../folders/XX/<hash>/...
		return !strings.Contains(norm, "/C/") && !strings.Contains(norm, "/T/")
	}
	return strings.HasPrefix(norm, "/Library") || strings.HasPrefix(norm, "/private") ||
		strings.HasPrefix(norm, "/var") || strings.HasPrefix(norm, "/opt")
}

// WhitelistPath is the shared mole whitelist location.
func WhitelistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "mole", "whitelist")
}
