package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath(t *testing.T) {
	cases := []struct {
		path    string
		wantErr string // substring, "" = ok
	}{
		{"", "空"},
		{"relative/path", "非绝对"},
		{"/tmp/../etc/passwd", ".."},
		{"/tmp/foo\nbar", "控制字符"},
		{"/System/Library", "系统关键"},
		{"/usr/bin", "系统关键"},
		{"/Users", "系统关键"},
		{"/Users/foo", "系统关键"}, // home root itself
		{"/Users/foo/Documents/x", ""}, // inside home ok
		{"/usr/local/bin/go", ""}, // homebrew exception
		{"/opt/homebrew/bin/mo", ""}, // homebrew exception
		{"/tmp/lumemole-test-file", ""},
		{"/private/var/folders/xx/yy/C/com.test", ""},
	}
	for _, c := range cases {
		err := ValidatePath(c.path)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", c.path, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: want err containing %q, got %v", c.path, c.wantErr, err)
		}
	}
}

func TestValidateSymlinkToCritical(t *testing.T) {
	tmp := t.TempDir()
	link := filepath.Join(tmp, "evil-link")
	if err := os.Symlink("/System", link); err != nil {
		t.Skip("symlink unsupported")
	}
	if err := ValidatePath(link); err == nil || !strings.Contains(err.Error(), "符号链接") {
		t.Fatalf("symlink to /System not rejected: %v", err)
	}
}

func TestWhitelist(t *testing.T) {
	tmp := t.TempDir()
	wl := filepath.Join(tmp, "whitelist")
	os.WriteFile(wl, []byte("/tmp/protected\n# comment\n/tmp/dir/*\n"), 0644)
	if !isWhitelisted("/tmp/protected", wl) {
		t.Error("exact match missed")
	}
	if !isWhitelisted("/tmp/protected/sub", wl) {
		t.Error("prefix match missed")
	}
	if isWhitelisted("/tmp/other", wl) {
		t.Error("non-match wrongly whitelisted")
	}
}

func TestNeedsSudo(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(home, "Library/Caches/x"), false},
		{"/Library/Caches/x", true},
		{"/private/var/log/x", true},
		{"/private/var/folders/ab/cd/C/x", false},
		{"/tmp/x", false},
	}
	for _, c := range cases {
		if got := needsSudo(c.path); got != c.want {
			t.Errorf("needsSudo(%s) = %v, want %v", c.path, got, c.want)
		}
	}
}
