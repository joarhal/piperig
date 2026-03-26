package main

import (
	"strings"
	"testing"
)

func TestInput_JSON(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/read_json.sh", "#!/bin/sh\ncat\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/read_json.sh
    input: json
    with:
      src: /data
      quality: "80"
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// JSON on stdin should contain the keys
	if !strings.Contains(stdout, "src") || !strings.Contains(stdout, "/data") {
		t.Errorf("expected JSON with src=/data, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "quality") || !strings.Contains(stdout, "80") {
		t.Errorf("expected JSON with quality=80, got:\n%s", stdout)
	}
}

func TestInput_Args(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo_args.sh", "#!/bin/sh\necho \"$@\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo_args.sh
    input: args
    with:
      quality: "80"
      src: /data
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "--quality") || !strings.Contains(stdout, "80") {
		t.Errorf("expected --quality 80 in args, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--src") || !strings.Contains(stdout, "/data") {
		t.Errorf("expected --src /data in args, got:\n%s", stdout)
	}
}

func TestRun_DirectExec(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello", "#!/bin/sh\necho direct-exec\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "direct-exec") {
		t.Errorf("expected direct-exec output, got:\n%s", stdout)
	}
}
