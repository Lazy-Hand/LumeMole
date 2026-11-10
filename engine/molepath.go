package engine

import (
	"os"
	"os/exec"
	"path/filepath"
)

// findMole locates the `mo` (or `mole`) CLI. Apps launched from Finder get
// a minimal PATH without Homebrew prefixes, so LookPath alone misses a
// perfectly good install — check the usual prefixes too.
func findMole() string {
	if p, err := exec.LookPath("mo"); err == nil {
		return p
	}
	if p, err := exec.LookPath("mole"); err == nil {
		return p
	}
	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin", "/home/linuxbrew/.linuxbrew/bin"} {
		for _, name := range []string{"mo", "mole"} {
			cand := filepath.Join(dir, name)
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand
			}
		}
	}
	return ""
}

// moleLibexecBin resolves a helper binary (status-go, analyze-go) that
// ships in mole's libexec/bin next to the `mo` symlink target.
func moleLibexecBin(mo, name string) string {
	if mo == "" {
		return ""
	}
	real, err := filepath.EvalSymlinks(mo)
	if err != nil {
		real = mo
	}
	// Homebrew layout: <cellar>/bin/mo → ../libexec/bin/<name>.
	cand := filepath.Join(filepath.Dir(real), "..", "libexec", "bin", name)
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	// Direct sibling (non-standard installs).
	cand = filepath.Join(filepath.Dir(real), name)
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	return ""
}
