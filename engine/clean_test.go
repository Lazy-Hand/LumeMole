package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCleanList(t *testing.T) {
	fixture := `# Mole Cleanup Preview - 2026-11-10
# comment line

=== User essentials ===
/Users/x/Library/Caches/go-build  # 9.77GB
/Users/x/Library/Caches/Yarn  # 15.8MB
/Users/x/Library/Caches/Unknown  # size unknown

=== Developer tools ===
/Users/x/.npm/_cacache  # 120MB, 342 items
/Users/x/.cache/type  # 4.4MB, counted under /Users/x/.cache
`
	dir := t.TempDir()
	path := filepath.Join(dir, "clean-list.txt")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := ParseCleanList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sections) != 2 {
		t.Fatalf("sections = %d", len(r.Sections))
	}
	u := r.Sections[0]
	if u.Name != "User essentials" {
		t.Fatalf("section name %q", u.Name)
	}
	if len(u.Entries) != 3 {
		t.Fatalf("entries = %d", len(u.Entries))
	}
	// 9.77GB + 15.8MB known; third is unknown.
	if u.Known != 2 {
		t.Fatalf("known = %d", u.Known)
	}
	if !r.Partial {
		t.Fatal("expected Partial for unknown size")
	}
	d := r.Sections[1]
	if d.Name != "Developer tools" || len(d.Entries) != 2 {
		t.Fatalf("dev section: %+v", d)
	}
	if d.Entries[0].Items != 342 {
		t.Fatalf("items = %d", d.Entries[0].Items)
	}
	if d.Entries[1].Note == "" {
		t.Fatal("expected counted-under note")
	}
}

func TestCleanSelectSection(t *testing.T) {
	r := &CleanReport{
		Sections: []CleanSection{
			{Name: "a", Entries: []CleanEntry{{Path: "/x", Selected: true}, {Path: "/y", Selected: true}}},
			{Name: "b", Entries: []CleanEntry{{Path: "/z", Selected: true}}},
		},
	}
	r.SelectSection("a", false)
	if r.Sections[0].Entries[0].Selected || r.Sections[0].Entries[1].Selected {
		t.Fatal("section a not deselected")
	}
	if !r.Sections[1].Entries[0].Selected {
		t.Fatal("section b wrongly deselected")
	}
}
