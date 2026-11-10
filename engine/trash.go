package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TrashItem is one entry sitting in ~/.Trash.
type TrashItem struct {
	Name    string
	Path    string
	Size    int64
	IsApp   bool // .app bundle — candidate for remnant cleanup
	Remnant []Remnant // filled by ScanRemnants for trashed apps
}

// TrashReport is the trash page state.
type TrashReport struct {
	Items     []TrashItem
	TotalSize int64
	Readable  bool // false when TCC denies ~/.Trash
}

// ScanTrash lists ~/.Trash and sizes each entry.
// Requires Full Disk Access; Readable=false otherwise.
func ScanTrash() *TrashReport {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".Trash")
	r := &TrashReport{}
	ents, err := os.ReadDir(root)
	if err != nil {
		return r // Readable stays false
	}
	r.Readable = true
	for _, e := range ents {
		p := filepath.Join(root, e.Name())
		it := TrashItem{
			Name: e.Name(),
			Path: p,
			Size: dirSizeQuiet(p),
		}
		if strings.HasSuffix(e.Name(), ".app") {
			it.IsApp = true
		}
		r.Items = append(r.Items, it)
		r.TotalSize += it.Size
	}
	return r
}

// ScanTrashedAppRemnants finds Library leftovers of apps sitting in the
// trash, so the user can finish the uninstall.
func (r *TrashReport) ScanTrashedAppRemnants() {
	sc := NewAppScanner()
	for i := range r.Items {
		if !r.Items[i].IsApp {
			continue
		}
		// Read the trashed bundle's Info.plist for name + bundle id.
		fake := readAppInfo(r.Items[i].Path)
		if fake == nil {
			continue
		}
		r.Items[i].Remnant = sc.FindRemnants(fake)
	}
}

// RemnantTotal sums the bytes of all trashed apps' remnants.
func (r *TrashReport) RemnantTotal() int64 {
	var n int64
	for _, it := range r.Items {
		for _, rem := range it.Remnant {
			if rem.Selected {
				n += rem.Size
			}
		}
	}
	return n
}

// EmptyTrash empties ~/.Trash via Finder (handles the permission dance
// for items Finder itself can remove).
func EmptyTrash() error {
	return exec.Command("osascript", "-e",
		`tell application "Finder" to empty trash`).Run()
}

// DeleteTrashItem removes one trashed item permanently (Finder delete).
func DeleteTrashItem(path string) error {
	q := strings.ReplaceAll(path, `\`, `\\`)
	q = strings.ReplaceAll(q, `"`, `\"`)
	return exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application "Finder" to delete POSIX file "%s"`, q)).Run()
}

// CleanRemnants trashes a trashed-app's selected Library leftovers.
func (it *TrashItem) CleanRemnants() (int, int64, error) {
	var done int
	var freed int64
	var lastErr error
	for _, r := range it.Remnant {
		if !r.Selected || r.System {
			continue
		}
		if err := aiTrash(r.Path); err != nil {
			lastErr = err
			continue
		}
		done++
		freed += r.Size
	}
	return done, freed, lastErr
}

// TrashState is the UI state for the 废纸篓 row on the clean page.
type TrashState struct {
	Report    *TrashReport
	Scanning  bool
	Cleaning  bool
	Err       string
	Expanded  bool
}

// Scan loads trash contents.
func (s *TrashState) Scan(onDone func()) {
	if s.Scanning {
		return
	}
	s.Scanning = true
	go func() {
		s.Report = ScanTrash()
		if s.Report != nil && s.Report.Readable {
			s.Report.ScanTrashedAppRemnants()
		}
		s.Scanning = false
		if onDone != nil {
			onDone()
		}
	}()
}

// Empty empties the whole trash and rescans.
func (s *TrashState) Empty(onDone func()) {
	if s.Cleaning {
		return
	}
	s.Cleaning = true
	go func() {
		if err := EmptyTrash(); err != nil {
			s.Err = err.Error()
		}
		s.Report = ScanTrash()
		s.Cleaning = false
		if onDone != nil {
			onDone()
		}
	}()
}

// CleanAppRemnants removes one trashed app's leftovers.
func (s *TrashState) CleanAppRemnants(i int, onDone func()) {
	if s.Report == nil || i < 0 || i >= len(s.Report.Items) {
		return
	}
	it := &s.Report.Items[i]
	go func() {
		done, _, err := it.CleanRemnants()
		if err != nil {
			s.Err = err.Error()
		}
		_ = done
		s.Report = ScanTrash()
		if s.Report != nil && s.Report.Readable {
			s.Report.ScanTrashedAppRemnants()
		}
		if onDone != nil {
			onDone()
		}
	}()
}
