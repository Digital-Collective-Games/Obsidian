package jobexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanAlertLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.txt")
	content := "normal line\n@@JOB-ALERT@@ [PANIC] universe source cold\nmore output\n@@JOB-ALERT@@ [INVARIANT] x must hold\nend\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	lines := scanAlertLines(p)
	if len(lines) != 2 {
		t.Fatalf("want 2 alert lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "@@JOB-ALERT@@ [PANIC] universe source cold" {
		t.Fatalf("unexpected first line: %q", lines[0])
	}
	if lines[1] != "@@JOB-ALERT@@ [INVARIANT] x must hold" {
		t.Fatalf("unexpected second line: %q", lines[1])
	}
}

func TestScanAlertLinesNoneAndMissing(t *testing.T) {
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.txt")
	if err := os.WriteFile(clean, []byte("all good\nnothing notable here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := scanAlertLines(clean); len(got) != 0 {
		t.Fatalf("clean file: want 0 lines, got %v", got)
	}
	if got := scanAlertLines(filepath.Join(dir, "missing.txt")); got != nil {
		t.Fatalf("missing file: want nil, got %v", got)
	}
	if got := scanAlertLines(""); got != nil {
		t.Fatalf("empty path: want nil, got %v", got)
	}
}

func TestTailFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.txt")
	if err := os.WriteFile(p, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tailFile(p, 4); got != "6789" {
		t.Fatalf("tail 4: want 6789, got %q", got)
	}
	if got := tailFile(p, 100); got != "0123456789" {
		t.Fatalf("tail 100: want full, got %q", got)
	}
	if got := tailFile("", 4); got != "" {
		t.Fatalf("empty path: want empty, got %q", got)
	}
}
