package main

import (
	"strings"
	"testing"
)

func TestRun_MultiStep_Sequential(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/step1.sh", "#!/bin/sh\necho one\n")
	writeScript(t, dir, "scripts/step2.sh", "#!/bin/sh\necho two\n")
	writeScript(t, dir, "scripts/step3.sh", "#!/bin/sh\necho three\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/step1.sh
  - job: scripts/step2.sh
  - job: scripts/step3.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "one") || !strings.Contains(stdout, "two") || !strings.Contains(stdout, "three") {
		t.Errorf("expected all three step outputs, got:\n%s", stdout)
	}
	// Verify order: one before two before three
	i1 := strings.Index(stdout, "one")
	i2 := strings.Index(stdout, "two")
	i3 := strings.Index(stdout, "three")
	if i1 >= i2 || i2 >= i3 {
		t.Errorf("steps ran out of order: one@%d two@%d three@%d", i1, i2, i3)
	}
}

func TestRun_MultiStep_FailFast(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/ok.sh", "#!/bin/sh\necho step-1-ran\n")
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/never.sh", "#!/bin/sh\necho should-not-run\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/ok.sh
  - job: scripts/fail.sh
  - job: scripts/never.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout, "step-1-ran") {
		t.Error("step 1 should have run")
	}
	if strings.Contains(stdout, "should-not-run") {
		t.Error("step 3 should NOT have run after step 2 failure")
	}
}

func TestRun_ExitCode1OnFailure(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/fail.sh
`)
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (runtime failure, not validation)", code)
	}
}

func TestRun_With_StepOverridesPipe(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"greeting=$GREETING target=$TARGET\"\n")
	writeFile(t, dir, "test.pipe.yaml", `with:
  greeting: hello
  target: world
steps:
  - job: scripts/echo.sh
    with:
      greeting: bonjour
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "greeting=bonjour") {
		t.Error("step with should override pipe with")
	}
	if !strings.Contains(stdout, "target=world") {
		t.Error("pipe with should be inherited when not overridden")
	}
}
