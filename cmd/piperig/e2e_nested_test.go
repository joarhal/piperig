package main

import (
	"strings"
	"testing"
)

func TestNestedPipe_Basic(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/before.sh", "#!/bin/sh\necho before\n")
	writeScript(t, dir, "scripts/child_job.sh", "#!/bin/sh\necho child\n")
	writeScript(t, dir, "scripts/after.sh", "#!/bin/sh\necho after\n")
	writeFile(t, dir, "child.pipe.yaml", `steps:
  - job: scripts/child_job.sh
`)
	writeFile(t, dir, "parent.pipe.yaml", `steps:
  - job: scripts/before.sh
  - job: child.pipe.yaml
  - job: scripts/after.sh
`)
	stdout, _, code := run(t, dir, "run", "parent.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "before") || !strings.Contains(stdout, "child") || !strings.Contains(stdout, "after") {
		t.Errorf("expected all three outputs (before, child, after), got:\n%s", stdout)
	}
	// Verify order
	ib := strings.Index(stdout, "before")
	ic := strings.Index(stdout, "child")
	ia := strings.Index(stdout, "after")
	if ib >= ic || ic >= ia {
		t.Errorf("nested pipe output out of order: before@%d child@%d after@%d", ib, ic, ia)
	}
}

func TestNestedPipe_ParentOverridesChild(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"quality=$QUALITY\"\n")
	writeFile(t, dir, "child.pipe.yaml", `with:
  quality: "80"
steps:
  - job: scripts/echo.sh
`)
	writeFile(t, dir, "parent.pipe.yaml", `steps:
  - job: child.pipe.yaml
    with:
      quality: "90"
`)
	stdout, _, code := run(t, dir, "run", "parent.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "quality=90") {
		t.Errorf("parent with should override child with, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "quality=80") {
		t.Error("child with value should not appear when parent overrides")
	}
}
