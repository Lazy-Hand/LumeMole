package app

import (
	"os"
	"path/filepath"
)

// Permissions is the TCC probe result.
type Permissions struct {
	FullDiskAccess bool
	// CheckedPaths lists what we probed, for the onboarding copy.
	CheckedPaths []string
}

// fdaProbePaths are directories that require Full Disk Access to read.
// If any of them is unreadable we treat FDA as not granted.
var fdaProbePaths = []string{
	".Trash",
	"Library/Safari",
	"Library/Mail",
	"Library/Messages",
}

// ProbePermissions checks whether the app can read TCC-protected locations.
// Reading ~/Library/Safari or ~/.Trash triggers TCC deny → error, which means
// FDA is not granted. A nil error (or "not exist") means we can read it.
func ProbePermissions() *Permissions {
	home, _ := os.UserHomeDir()
	p := &Permissions{FullDiskAccess: true}
	for _, rel := range fdaProbePaths {
		abs := filepath.Join(home, rel)
		p.CheckedPaths = append(p.CheckedPaths, abs)
		if !canReadDir(abs) {
			p.FullDiskAccess = false
			return p
		}
	}
	return p
}

// canReadDir reports whether dir is readable. Not-exist counts as readable
// (nothing to protect); permission errors mean denied.
func canReadDir(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		return false
	}
	defer f.Close()
	_, err = f.ReadDir(1)
	if err != nil {
		// "no such file" inside is fine; permission denied is not.
		if os.IsNotExist(err) || os.IsPermission(err) {
			return !os.IsPermission(err)
		}
	}
	return true
}
