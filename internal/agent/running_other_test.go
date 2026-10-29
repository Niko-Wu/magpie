//go:build !darwin

package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// On a multi-user Linux server another user's Codex must not be listed:
// pgrep is asked for this user's processes only (#4240).
func TestListRunningAsksForThisUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no pgrep on Windows")
	}
	dir := t.TempDir()
	args := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + args + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "pgrep"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	listRunning("codex")

	got, err := os.ReadFile(args)
	if err != nil {
		t.Fatalf("pgrep was not run: %v", err)
	}
	lines := strings.Fields(string(got))
	want := []string{"-u", strconv.Itoa(os.Getuid()), "-f", "codex"}
	if strings.Join(lines, " ") != strings.Join(want, " ") {
		t.Fatalf("pgrep args = %v, want %v", lines, want)
	}
}
