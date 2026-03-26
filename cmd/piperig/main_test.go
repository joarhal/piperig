package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "piperig-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmpdir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binary = filepath.Join(dir, "piperig")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s\n", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// run executes the piperig binary with args in the given working directory.
// Returns stdout, stderr, and exit code.
func run(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("exec error: %v", err)
		}
	}
	return stdout.String(), stderr.String(), code
}

// writeFile creates a file at dir/name with the given content.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeScript creates an executable script at dir/name.
func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := writeFile(t, dir, name, content)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVersion(t *testing.T) {
	stdout, _, code := run(t, t.TempDir(), "version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "piperig") {
		t.Errorf("stdout = %q, want version string containing 'piperig'", stdout)
	}
}

func TestRunSuccess(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `description: simple test
steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("stdout should contain 'hello', got:\n%s", stdout)
	}
}

func TestRunWithOverride(t *testing.T) {
	dir := t.TempDir()
	// Script that prints the GREETING env var
	writeScript(t, dir, "scripts/greet.sh", "#!/bin/sh\necho \"greeting=$GREETING\"\n")
	writeFile(t, dir, "test.pipe.yaml", `description: override test
steps:
  - job: scripts/greet.sh
    with:
      greeting: default
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml", "greeting=custom")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "greeting=custom") {
		t.Errorf("expected override applied, got:\n%s", stdout)
	}
}

func TestCheckSuccess(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `description: check test
steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Total:") {
		t.Errorf("expected 'Total:' in check output, got:\n%s", stdout)
	}
}

func TestRunDirectory(t *testing.T) {
	dir := t.TempDir()
	pipesDir := filepath.Join(dir, "pipes")
	os.MkdirAll(pipesDir, 0o755)

	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "pipes/a.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
	writeFile(t, dir, "pipes/b.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
	_, _, code := run(t, dir, "run", "pipes/")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestCheckNoTarget(t *testing.T) {
	_, _, code := run(t, t.TempDir(), "check")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestRunValidationError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.pipe.yaml", `steps:
  - job: scripts/nonexistent.py
`)
	_, _, code := run(t, dir, "run", "bad.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := run(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, ".piperig.yaml") {
		t.Errorf("expected '.piperig.yaml' in output, got:\n%s", stdout)
	}
	// Verify the file was created
	if _, err := os.Stat(filepath.Join(dir, ".piperig.yaml")); err != nil {
		t.Errorf("expected .piperig.yaml to be created: %v", err)
	}
}

func TestNewPipe(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := run(t, dir, "new", "pipe", "test")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "test.pipe.yaml") {
		t.Errorf("expected 'test.pipe.yaml' in output, got:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "test.pipe.yaml")); err != nil {
		t.Errorf("expected test.pipe.yaml to be created: %v", err)
	}
}

func TestNewSchedule(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := run(t, dir, "new", "schedule", "test")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "test.yaml") {
		t.Errorf("expected 'test.yaml' in output, got:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "test.yaml")); err != nil {
		t.Errorf("expected test.yaml to be created: %v", err)
	}
}

func TestRunNoColor(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `description: no-color test
steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml", "--no-color")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "\033[") {
		t.Errorf("expected no ANSI codes with --no-color, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("expected output to contain 'hello', got:\n%s", stdout)
	}
}

func TestCheckNoColor(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `description: no-color check
steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml", "--no-color")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "\033[") {
		t.Errorf("expected no ANSI codes with --no-color, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Total:") {
		t.Errorf("expected 'Total:' in check output, got:\n%s", stdout)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"host=$HOST\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    with:
      host: $PIPERIG_E2E_HOST
`)
	// Set a unique env var for the test
	t.Setenv("PIPERIG_E2E_HOST", "db.test.local")
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "host=db.test.local") {
		t.Errorf("expected expanded env var in output, got:\n%s", stdout)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "pipes/a.pipe.yaml", `description: First pipe
steps:
  - job: scripts/hello.sh
`)
	writeFile(t, dir, "pipes/b.pipe.yaml", `description: Second pipe
steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "a.pipe.yaml") {
		t.Errorf("expected a.pipe.yaml in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "First pipe") {
		t.Errorf("expected description 'First pipe' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "b.pipe.yaml") {
		t.Errorf("expected b.pipe.yaml in output, got:\n%s", stdout)
	}
}

func TestListHiddenExcluded(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "visible.pipe.yaml", `description: Visible
steps:
  - job: scripts/hello.sh
`)
	writeFile(t, dir, "hidden.pipe.yaml", `description: Hidden helper
hidden: true
steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "visible.pipe.yaml") {
		t.Errorf("expected visible pipe in output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "hidden.pipe.yaml") {
		t.Errorf("hidden pipe should be excluded, got:\n%s", stdout)
	}
}

func TestListSubdirectory(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "pipes/daily/a.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
	writeFile(t, dir, "pipes/other/b.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
	stdout, _, code := run(t, dir, "list", "pipes/daily")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "a.pipe.yaml") {
		t.Errorf("expected a.pipe.yaml in output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "b.pipe.yaml") {
		t.Errorf("b.pipe.yaml should not be in output for pipes/daily/, got:\n%s", stdout)
	}
}

func TestListEmpty(t *testing.T) {
	dir := t.TempDir()
	_, _, code := run(t, dir, "list")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 for empty directory", code)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, stderr, code := run(t, t.TempDir(), "notacommand")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("expected 'unknown command' in stderr, got:\n%s", stderr)
	}
}
