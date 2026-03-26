package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// flakyScript returns a shell script that fails fail_count times using a counter file.
const flakyScript = `#!/bin/sh
count=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
count=$((count + 1))
echo $count > "$COUNTER_FILE"
if [ $count -le $FAIL_COUNT ]; then
  echo "attempt $count: failing"
  exit 1
fi
echo "attempt $count: ok"
`

func TestRetry_SuccessAfterFailures(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/flaky.sh", flakyScript)
	counterFile := filepath.Join(dir, "counter")
	writeFile(t, dir, "test.pipe.yaml", fmt.Sprintf(`steps:
  - job: scripts/flaky.sh
    retry: 3
    retry_delay: 0s
    with:
      fail_count: "2"
      counter_file: %s
`, counterFile))
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (should succeed after retries)", code)
	}
	if !strings.Contains(stdout, "attempt 3: ok") {
		t.Errorf("expected success on 3rd attempt, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "retry 1/3") {
		t.Error("expected retry marker in output")
	}
}

func TestRetry_StepOverride(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/flaky.sh", flakyScript)
	counterFile := filepath.Join(dir, "counter")
	writeFile(t, dir, "test.pipe.yaml", fmt.Sprintf(`retry: 1
steps:
  - job: scripts/flaky.sh
    retry: 3
    retry_delay: 0s
    with:
      fail_count: "2"
      counter_file: %s
`, counterFile))
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (step retry=3 overrides pipe retry=1)", code)
	}
}

func TestRetry_FalseDisables(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeFile(t, dir, "test.pipe.yaml", `retry: 3
steps:
  - job: scripts/fail.sh
    retry: false
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if strings.Contains(stdout, "retry") {
		t.Error("retry: false should disable inherited retry, but saw retry marker")
	}
}

func TestRetry_DelayShown(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/flaky.sh", flakyScript)
	counterFile := filepath.Join(dir, "counter")
	writeFile(t, dir, "test.pipe.yaml", fmt.Sprintf(`steps:
  - job: scripts/flaky.sh
    retry: 1
    retry_delay: 1s
    with:
      fail_count: "1"
      counter_file: %s
`, counterFile))
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "retry 1/1 (1s)") {
		t.Errorf("expected retry delay in output, got:\n%s", stdout)
	}
}

func TestTimeout_KillsJob(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/slow.sh", "#!/bin/sh\nsleep 30\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/slow.sh
    timeout: 1s
`)
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (job should be killed by timeout)", code)
	}
}

func TestTimeout_StepOverride(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/slow.sh", "#!/bin/sh\nsleep 30\n")
	writeFile(t, dir, "test.pipe.yaml", `timeout: 30s
steps:
  - job: scripts/slow.sh
    timeout: 1s
`)
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (step timeout 1s should override pipe 30s)", code)
	}
}

func TestAllowFailure_Continue(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/ok.sh", "#!/bin/sh\necho step1\n")
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/after.sh", "#!/bin/sh\necho step3\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/ok.sh
  - job: scripts/fail.sh
    allow_failure: true
  - job: scripts/after.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (allow_failure should let pipe continue)", code)
	}
	if !strings.Contains(stdout, "step1") || !strings.Contains(stdout, "step3") {
		t.Errorf("expected both step1 and step3 in output, got:\n%s", stdout)
	}
}

func TestAllowFailure_DefaultFalse(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, dir, "scripts/never.sh", "#!/bin/sh\necho should-not-run\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/fail.sh
  - job: scripts/never.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if strings.Contains(stdout, "should-not-run") {
		t.Error("default allow_failure=false should stop pipe on failure")
	}
}
