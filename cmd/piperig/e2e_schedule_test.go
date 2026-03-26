package main

import (
	"strings"
	"testing"
)

func TestServe_Now(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/hello.sh", "#!/bin/sh\necho hello-from-schedule\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/hello.sh
`)
	writeFile(t, dir, "schedule.yaml", `- name: test
  cron: "0 5 * * *"
  run:
    - test.pipe.yaml
`)
	stdout, _, code := run(t, dir, "serve", "schedule.yaml", "--now")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "hello-from-schedule") {
		t.Errorf("expected scheduled pipe output, got:\n%s", stdout)
	}
}

func TestServe_NowWithOverrides(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"greeting=$GREETING\"\n")
	writeFile(t, dir, "test.pipe.yaml", `with:
  greeting: default
steps:
  - job: scripts/echo.sh
`)
	writeFile(t, dir, "schedule.yaml", `- name: test
  cron: "0 5 * * *"
  run:
    - test.pipe.yaml
  with:
    greeting: scheduled
`)
	stdout, _, code := run(t, dir, "serve", "schedule.yaml", "--now")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "greeting=scheduled") {
		t.Errorf("expected schedule override, got:\n%s", stdout)
	}
}
