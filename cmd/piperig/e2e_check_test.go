package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheck_Directory(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	os.MkdirAll(filepath.Join(dir, "pipes"), 0o755)
	writeFile(t, dir, "pipes/a.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
	writeFile(t, dir, "pipes/b.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "check", "pipes/")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// Should contain Total for each pipe
	if strings.Count(stdout, "Total:") < 2 {
		t.Errorf("expected Total: for each pipe, got:\n%s", stdout)
	}
}

func TestCheck_WithOverrides(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello.sh
    with:
      color: red
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml", "color=blue")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "color=blue") {
		t.Errorf("expected override in check output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "color=red") {
		t.Error("override should replace original value in check output")
	}
}

func TestCheck_Description(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `description: My described pipe
steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "My described pipe") {
		t.Errorf("expected description in check output, got:\n%s", stdout)
	}
}
