package main

import (
	"strings"
	"testing"
)

func TestValidation_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `foo: bar
steps:
  - job: scripts/hello.sh
`)
	_, stderr, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code for unknown YAML key")
	}
	if !strings.Contains(stderr, "unknown") || !strings.Contains(stderr, "foo") {
		t.Errorf("expected error about unknown key 'foo', got stderr:\n%s", stderr)
	}
}

func TestValidation_JobNotFound(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/nonexistent.sh
`)
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (validation error)", code)
	}
}

func TestValidation_BadExtension(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "scripts/run.xyz", "content")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/run.xyz
`)
	_, stderr, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (validation error)", code)
	}
	if !strings.Contains(stderr, ".xyz") {
		t.Errorf("expected error mentioning .xyz extension, got stderr:\n%s", stderr)
	}
}

func TestValidation_BadTimeExpr(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello.sh
    loop:
      date: -1x
`)
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (validation error for bad time expression)", code)
	}
}

func TestValidation_UnresolvedTemplate(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello.sh
    with:
      output: "{missing}/file.txt"
`)
	_, stderr, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (validation error for unresolved template)", code)
	}
	if !strings.Contains(stderr, "missing") {
		t.Errorf("expected error about unresolved template 'missing', got stderr:\n%s", stderr)
	}
}

func TestValidation_BadInputMode(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello.sh
    input: xml
`)
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (validation error for bad input mode)", code)
	}
}

func TestValidation_BadTimeout(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello.sh
    timeout: "10 minutes"
`)
	_, stderr, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (validation error for bad timeout)", code)
	}
	if !strings.Contains(stderr, "invalid timeout") {
		t.Errorf("expected error about invalid timeout, got stderr:\n%s", stderr)
	}
}

func TestValidation_BadRetryDelay(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello.sh
    retry_delay: "5 seconds"
`)
	_, stderr, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (validation error for bad retry_delay)", code)
	}
	if !strings.Contains(stderr, "invalid retry_delay") {
		t.Errorf("expected error about invalid retry_delay, got stderr:\n%s", stderr)
	}
}

func TestValidation_NestedObjectInWith(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello.sh
    with:
      nested:
        key: value
`)
	_, stderr, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code for nested object in with")
	}
	if !strings.Contains(stderr, "nested") {
		t.Errorf("expected error about nested objects, got stderr:\n%s", stderr)
	}
}

func TestList_WarnsOnBrokenFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
	writeFile(t, dir, "broken.pipe.yaml", `not: [valid: yaml: {{`)
	stdout, stderr, code := run(t, dir, "list", dir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "broken.pipe.yaml") {
		t.Errorf("expected warning about broken.pipe.yaml in stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stdout, "good.pipe.yaml") {
		t.Errorf("expected good.pipe.yaml in stdout, got:\n%s", stdout)
	}
}
