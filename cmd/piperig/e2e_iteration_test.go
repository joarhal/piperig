package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestLoop_NumericRange(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"n=$N\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    loop:
      n: 1..3
`)
	t.Run("check", func(t *testing.T) {
		stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, "Total: 3 calls") {
			t.Errorf("expected 3 calls, got:\n%s", stdout)
		}
	})
	t.Run("run", func(t *testing.T) {
		stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		for _, n := range []string{"n=1", "n=2", "n=3"} {
			if !strings.Contains(stdout, n) {
				t.Errorf("expected %q in output, got:\n%s", n, stdout)
			}
		}
	})
}

func TestLoop_ExplicitList(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"region=$REGION\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    loop:
      region: [eu, us, asia]
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, r := range []string{"region=eu", "region=us", "region=asia"} {
		if !strings.Contains(stdout, r) {
			t.Errorf("expected %q in output, got:\n%s", r, stdout)
		}
	}
}

func TestLoop_TimeRange(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"date=$DATE\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    loop:
      date: -2d..-1d
`)
	t.Run("check", func(t *testing.T) {
		stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, "Total: 2 calls") {
			t.Errorf("expected 2 calls, got:\n%s", stdout)
		}
	})
	t.Run("run", func(t *testing.T) {
		stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		// Match only the echo output lines (prefixed with ·)
		datePattern := regexp.MustCompile(`· date=\d{4}-\d{2}-\d{2}`)
		matches := datePattern.FindAllString(stdout, -1)
		if len(matches) != 2 {
			t.Errorf("expected 2 date outputs, got %d: %v\nfull output:\n%s", len(matches), matches, stdout)
		}
	})
}

func TestLoop_CartesianProduct(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"n=$N region=$REGION\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    loop:
      n: 1..3
      region: [eu, us]
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Total: 6 calls") {
		t.Errorf("expected 6 calls (3 x 2), got:\n%s", stdout)
	}
}

func TestLoop_PipeLevel(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/a.sh", "#!/bin/sh\necho \"a n=$N\"\n")
	writeScript(t, dir, "scripts/b.sh", "#!/bin/sh\necho \"b n=$N\"\n")
	writeFile(t, dir, "test.pipe.yaml", `loop:
  n: 1..2
steps:
  - job: scripts/a.sh
  - job: scripts/b.sh
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Total: 4 calls") {
		t.Errorf("expected 4 calls (2 steps x 2 iterations), got:\n%s", stdout)
	}
}

func TestLoop_FalseDisables(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho ok\n")
	writeFile(t, dir, "test.pipe.yaml", `loop:
  n: 1..3
steps:
  - job: scripts/echo.sh
  - job: scripts/echo.sh
    loop: false
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Total: 4 calls") {
		t.Errorf("expected 4 calls (3 + 1), got:\n%s", stdout)
	}
}

// --- Each ---

func TestEach_Basic(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"size=$SIZE label=$LABEL\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    each:
      - { size: 1920x1080, label: fullhd }
      - { size: 640x480, label: sd }
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "size=1920x1080") {
		t.Error("expected fullhd params")
	}
	if !strings.Contains(stdout, "size=640x480") {
		t.Error("expected sd params")
	}
}

func TestEach_PipeLevel(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho ok\n")
	writeFile(t, dir, "test.pipe.yaml", `each:
  - { env: staging }
  - { env: production }
steps:
  - job: scripts/echo.sh
  - job: scripts/echo.sh
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Total: 4 calls") {
		t.Errorf("expected 4 calls (2 each x 2 steps), got:\n%s", stdout)
	}
}

func TestEach_FalseDisables(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho ok\n")
	writeFile(t, dir, "test.pipe.yaml", `each:
  - { env: staging }
  - { env: production }
steps:
  - job: scripts/echo.sh
  - job: scripts/echo.sh
    each: false
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Total: 3 calls") {
		t.Errorf("expected 3 calls (2 + 1), got:\n%s", stdout)
	}
}

func TestEachLoop_Cartesian(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho ok\n")
	writeFile(t, dir, "test.pipe.yaml", `each:
  - { label: fullhd }
  - { label: sd }
loop:
  n: 1..3
steps:
  - job: scripts/echo.sh
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Total: 6 calls") {
		t.Errorf("expected 6 calls (2 each x 3 loop), got:\n%s", stdout)
	}
}
