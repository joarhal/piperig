package main

import (
	"strings"
	"testing"
)

func TestInit_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".piperig.yaml", "interpreters:\n  .sh: bash\n")
	_, stderr, code := run(t, dir, "init")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (file already exists)", code)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("expected 'already exists' error, got stderr:\n%s", stderr)
	}
}

func TestNew_PipeAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.pipe.yaml", "steps: []\n")
	_, stderr, code := run(t, dir, "new", "pipe", "test")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (file already exists)", code)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("expected 'already exists' error, got stderr:\n%s", stderr)
	}
}
