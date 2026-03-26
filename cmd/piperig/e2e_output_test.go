package main

import (
	"strings"
	"testing"
)

func TestOutput_JSONLog(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/json.sh", `#!/bin/sh
echo "starting"
echo '{"label":"fullhd","file":"photo.jpg","size":"1920x1080"}'
echo "done"
`)
	writeFile(t, dir, "test.pipe.yaml", `log:
  - label
  - file
steps:
  - job: scripts/json.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "fullhd | photo.jpg") {
		t.Errorf("expected formatted JSON fields, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "starting") {
		t.Error("expected plain text passthrough")
	}
	if !strings.Contains(stdout, "done") {
		t.Error("expected plain text passthrough")
	}
}

func TestOutput_LogStepOverride(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/json.sh", `#!/bin/sh
echo '{"label":"hd","file":"a.jpg","size":"1280x720"}'
`)
	writeFile(t, dir, "test.pipe.yaml", `log:
  - label
  - file
steps:
  - job: scripts/json.sh
  - job: scripts/json.sh
    log: [size]
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// First step uses pipe log [label, file]
	if !strings.Contains(stdout, "hd | a.jpg") {
		t.Error("step 1 should use pipe-level log fields")
	}
	// Second step uses own log [size] — should show 1280x720 without label
	if !strings.Contains(stdout, "1280x720") {
		t.Error("step 2 should use step-level log field [size]")
	}
}

func TestOutput_Stderr(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/stderr.sh", "#!/bin/sh\necho \"error message\" >&2\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/stderr.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "! error message") {
		t.Errorf("expected stderr with ! marker, got:\n%s", stdout)
	}
}

func TestOutput_Summary(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")

	t.Run("success", func(t *testing.T) {
		writeFile(t, dir, "ok.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
		stdout, _, code := run(t, dir, "run", "ok.pipe.yaml")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, "✓") || !strings.Contains(stdout, "1 calls") {
			t.Errorf("expected success summary with call count, got:\n%s", stdout)
		}
	})

	t.Run("failure", func(t *testing.T) {
		writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
		writeFile(t, dir, "fail.pipe.yaml", `steps:
  - job: scripts/fail.sh
`)
		stdout, _, _ := run(t, dir, "run", "fail.pipe.yaml")
		if !strings.Contains(stdout, "✗") {
			t.Errorf("expected failure summary marker, got:\n%s", stdout)
		}
	})
}

func TestOutput_PipeHeader(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `description: My test pipe
steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "My test pipe") {
		t.Errorf("expected description in pipe header, got:\n%s", stdout)
	}
}
