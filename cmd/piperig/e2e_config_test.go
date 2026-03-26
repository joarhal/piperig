package main

import (
	"strings"
	"testing"
)

func TestConfig_CustomInterpreter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".piperig.yaml", `interpreters:
  .custom: bash
`)
	writeScript(t, dir, "scripts/test.custom", "#!/bin/sh\necho custom-interp\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/test.custom
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "custom-interp") {
		t.Errorf("expected custom interpreter output, got:\n%s", stdout)
	}
}

func TestConfig_ProcessEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".piperig.yaml", `interpreters:
  .sh: bash
env:
  CUSTOM_VAR: from_config
`)
	writeScript(t, dir, "scripts/print_env.sh", "#!/bin/sh\necho \"CUSTOM_VAR=$CUSTOM_VAR\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/print_env.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "CUSTOM_VAR=from_config") {
		t.Errorf("expected config env var in output, got:\n%s", stdout)
	}
}
