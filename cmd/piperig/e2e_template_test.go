package main

import (
	"strings"
	"testing"
)

func TestTemplate_FromLoop(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"output=$OUTPUT\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    loop:
      region: [eu, us]
    with:
      output: /data/{region}/report.csv
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "output=/data/eu/report.csv") {
		t.Error("expected template from loop value: eu")
	}
	if !strings.Contains(stdout, "output=/data/us/report.csv") {
		t.Error("expected template from loop value: us")
	}
}

func TestTemplate_FromEach(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"output=$OUTPUT\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    each:
      - { label: fullhd, size: 1920x1080 }
    with:
      output: /data/{label}.jpg
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "output=/data/fullhd.jpg") {
		t.Errorf("expected template from each value, got:\n%s", stdout)
	}
}
