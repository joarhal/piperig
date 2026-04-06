package main

import (
	"strings"
	"testing"
)

func TestHook_OnFailPipeLevel(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/hook.sh", "#!/bin/sh\necho hook-fired\n")
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/fail.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "hook-fired") {
		t.Errorf("expected on_fail hook output, got:\n%s", stdout)
	}
}

func TestHook_OnSuccessPipeLevel(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/ok.sh", "#!/bin/sh\necho step-ok\n")
	writeScript(t, dir, "scripts/hook.sh", "#!/bin/sh\necho success-hook\n")
	writeFile(t, dir, "test.pipe.yaml", `on_success: scripts/hook.sh

steps:
  - job: scripts/ok.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "success-hook") {
		t.Errorf("expected on_success hook output, got:\n%s", stdout)
	}
}

func TestHook_StepOverridesPipe(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/pipe-hook.sh", "#!/bin/sh\necho pipe-hook\n")
	writeScript(t, dir, "scripts/step-hook.sh", "#!/bin/sh\necho step-hook\n")
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/pipe-hook.sh

steps:
  - job: scripts/fail.sh
    on_fail: scripts/step-hook.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "step-hook") {
		t.Errorf("expected step-level hook, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "pipe-hook") {
		t.Errorf("pipe-level hook should not fire when step overrides, got:\n%s", stdout)
	}
}

func TestHook_FalseDisables(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/hook.sh", "#!/bin/sh\necho hook-fired\n")
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/fail.sh
    on_fail: false
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if strings.Contains(stdout, "hook-fired") {
		t.Errorf("hook should not fire when on_fail: false, got:\n%s", stdout)
	}
}

func TestHook_AfterRetryExhausted(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/hook.sh", "#!/bin/sh\necho hook-fired\n")
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/fail.sh
    retry: 2
    retry_delay: 0s
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	// Hook should fire exactly once (after all retries exhausted)
	count := strings.Count(stdout, "hook-fired")
	if count != 1 {
		t.Errorf("expected hook to fire once, fired %d times, output:\n%s", count, stdout)
	}
}

func TestHook_ErrorStopsPipe(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/hook-fail.sh", "#!/bin/sh\necho hook-error\nexit 1\n")
	writeScript(t, dir, "scripts/ok.sh", "#!/bin/sh\necho should-not-run\n")
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook-fail.sh

steps:
  - job: scripts/fail.sh
  - job: scripts/ok.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if strings.Contains(stdout, "should-not-run") {
		t.Errorf("second step should not run after hook failure, got:\n%s", stdout)
	}
}

func TestHook_AllowFailureStillFires(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/hook.sh", "#!/bin/sh\necho hook-fired\n")
	writeScript(t, dir, "scripts/ok.sh", "#!/bin/sh\necho step2-ok\n")
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/fail.sh
    allow_failure: true
  - job: scripts/ok.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (allow_failure)", code)
	}
	if !strings.Contains(stdout, "hook-fired") {
		t.Errorf("on_fail hook should fire even with allow_failure, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "step2-ok") {
		t.Errorf("second step should run after allow_failure, got:\n%s", stdout)
	}
}

func TestHook_EnvVars(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 42\n")
	writeScript(t, dir, "scripts/hook.sh", `#!/bin/sh
echo "PIPE=$PIPERIG_PIPE"
echo "STEP=$PIPERIG_STEP"
echo "STATUS=$PIPERIG_STATUS"
echo "EXIT=$PIPERIG_EXIT_CODE"
echo "ELAPSED=$PIPERIG_ELAPSED_MS"
`)
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/fail.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "PIPE=test.pipe.yaml") {
		t.Errorf("expected PIPERIG_PIPE, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "STEP=scripts/fail.sh") {
		t.Errorf("expected PIPERIG_STEP, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "STATUS=fail") {
		t.Errorf("expected PIPERIG_STATUS=fail, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "EXIT=42") {
		t.Errorf("expected PIPERIG_EXIT_CODE=42, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "ELAPSED=") {
		t.Errorf("expected PIPERIG_ELAPSED_MS, got:\n%s", stdout)
	}
}

func TestHook_ReceivesStdin(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/job.sh", "#!/bin/sh\necho job-output-line\nexit 1\n")
	writeScript(t, dir, "scripts/hook.sh", `#!/bin/sh
input=$(cat)
echo "GOT:$input"
`)
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/job.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "GOT:job-output-line") {
		t.Errorf("hook should receive step output on stdin, got:\n%s", stdout)
	}
}

func TestHook_ReceivesWithParams(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/hook.sh", `#!/bin/sh
echo "SRC=$SRC"
echo "QUALITY=$QUALITY"
`)
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/fail.sh
    with:
      src: /data/photos
      quality: 80
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "SRC=/data/photos") {
		t.Errorf("hook should receive with params, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "QUALITY=80") {
		t.Errorf("hook should receive with params, got:\n%s", stdout)
	}
}

func TestHook_Timeout(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/slow.sh", "#!/bin/sh\nsleep 30\n")
	writeScript(t, dir, "scripts/hook.sh", `#!/bin/sh
echo "STATUS=$PIPERIG_STATUS"
`)
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/slow.sh
    timeout: 500ms
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "STATUS=timeout") {
		t.Errorf("expected PIPERIG_STATUS=timeout, got:\n%s", stdout)
	}
}

func TestHook_CheckOutput(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/run.sh", "#!/bin/sh\necho ok\n")
	writeScript(t, dir, "scripts/hook.sh", "#!/bin/sh\necho hook\n")
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/hook.sh

steps:
  - job: scripts/run.sh
`)
	stdout, _, code := run(t, dir, "check", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "[on_fail: scripts/hook.sh]") {
		t.Errorf("check should show hooks, got:\n%s", stdout)
	}
}

func TestHook_Validation(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/run.sh", "#!/bin/sh\necho ok\n")
	writeFile(t, dir, "test.pipe.yaml", `on_fail: scripts/nonexistent.sh

steps:
  - job: scripts/run.sh
`)
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (validation error)", code)
	}
}
