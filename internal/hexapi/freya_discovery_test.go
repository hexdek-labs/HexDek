package hexapi

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// freya_discovery_test.go — pins findFreyaBinary's cross-platform
// discovery. The motivating bug (found during the 2026-06 DARKSTAR
// Windows rebuild): the old discovery looked for a bare "hexdek-freya"
// via os.Stat("./hexdek-freya") + a cwd LookPath, neither of which
// matches hexdek-freya.exe — so Freya deck analysis was silently dead
// on every Windows deploy and /api/health reported freya: fail despite
// the binary being present.

// The binary name carries the platform extension so a Windows deploy's
// hexdek-freya.exe is actually discoverable.
func TestFreyaBinaryName_PlatformExtension(t *testing.T) {
	got := freyaBinaryName()
	want := "hexdek-freya"
	if runtime.GOOS == "windows" {
		want = "hexdek-freya.exe"
	}
	if got != want {
		t.Errorf("freyaBinaryName()=%q, want %q on %s", got, want, runtime.GOOS)
	}
}

// Branch 1 — the canonical deploy shape and the core of the Windows
// fix: freya built alongside the server binary is found via the
// executable's own directory, regardless of cwd or PATH.
func TestFindFreyaBinary_NextToExecutable(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable unavailable: %v", err)
	}
	planted := filepath.Join(filepath.Dir(exe), freyaBinaryName())
	if _, err := os.Stat(planted); err == nil {
		t.Skip("a real freya binary already sits next to the test binary; skipping plant")
	}
	if err := os.WriteFile(planted, []byte("stub"), 0o755); err != nil {
		t.Fatalf("plant binary next to exe: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(planted) })

	// Neutralize the other two branches so we know branch 1 fired.
	t.Setenv("PATH", "")
	origWD, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	got, ok := findFreyaBinary()
	if !ok {
		t.Fatal("findFreyaBinary did not find the binary next to the executable")
	}
	if got != planted {
		t.Errorf("path=%q, want the exe-adjacent candidate %q", got, planted)
	}
}

// Branch 3 — cwd fallback must return a separator-prefixed path so
// exec.Command runs the local file instead of re-searching PATH
// (Go 1.19+ refuses a cwd match found via a bare name).
func TestFindFreyaBinary_CwdReturnsRelativePath(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, freyaBinaryName()), []byte("stub"), 0o755); err != nil {
		t.Fatalf("plant cwd binary: %v", err)
	}
	t.Setenv("PATH", "")
	origWD, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	got, ok := findFreyaBinary()
	if !ok {
		t.Fatal("findFreyaBinary missed a binary in cwd")
	}
	if !strings.HasPrefix(got, "."+string(os.PathSeparator)) {
		t.Errorf("cwd path=%q, want a %q-prefixed relative path", got, "."+string(os.PathSeparator))
	}
}

// Nothing on PATH, nothing in cwd, nothing next to the exe → not found.
func TestFindFreyaBinary_NotFound(t *testing.T) {
	if exe, err := os.Executable(); err == nil {
		if _, err := os.Stat(filepath.Join(filepath.Dir(exe), freyaBinaryName())); err == nil {
			t.Skip("a real freya binary sits next to the test binary; not-found case unreachable here")
		}
	}
	t.Setenv("PATH", "")
	origWD, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	if _, ok := findFreyaBinary(); ok {
		t.Error("findFreyaBinary reported found with no binary anywhere")
	}
}
