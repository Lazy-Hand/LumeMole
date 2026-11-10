package engine
import "testing"
func TestKindOf(t *testing.T) {
	cases := []struct{ name string; isDir bool; want FileKind }{
		{"photo.jpg", false, KindImage},
		{"movie.MP4", false, KindVideo},
		{"song.flac", false, KindAudio},
		{"doc.pdf", false, KindDocument},
		{"archive.zip", false, KindArchive},
		{"setup.pkg", false, KindInstaller},
		{"disk.dmg", false, KindDiskImage},
		{"main.go", false, KindDeveloper},
		{"design.sketch", false, KindDesign},
		{"font.ttf", false, KindFont},
		{"data.sqlite", false, KindData},
		{"weird.xyz", false, KindOther},
		{"folder", true, KindFolder},
	}
	for _, c := range cases {
		if got := KindOf(AnalyzeEntry{Name: c.name, IsDir: c.isDir}); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}
