package main

import (
	"strings"
	"testing"
)

func TestDotEnv_Loaded(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "GREETING=hello_from_dotenv\n")
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"GREETING=$GREETING\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "GREETING=hello_from_dotenv") {
		t.Errorf("expected .env var in output, got:\n%s", stdout)
	}
}

func TestDotEnv_SystemEnvWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "PIPERIG_TEST_SYSWIN=from_dotenv\n")
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"VAL=$PIPERIG_TEST_SYSWIN\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
`)
	// Run with system env set — it should win over .env
	stdout, _, code := runWithEnv(t, dir, []string{"PIPERIG_TEST_SYSWIN=from_system"}, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "VAL=from_system") {
		t.Errorf("expected system env to win, got:\n%s", stdout)
	}
}

func TestDotEnv_ConfigEnvWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "MY_VAR=from_dotenv\n")
	writeFile(t, dir, ".piperig.yaml", "env:\n  MY_VAR: from_config\n")
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"MY_VAR=$MY_VAR\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "MY_VAR=from_config") {
		t.Errorf("expected .piperig.yaml env to win over .env, got:\n%s", stdout)
	}
}

func TestDotEnv_InterpolationInWith(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "BUCKET_NAME=my-bucket\n")
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"DEST=$DEST\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
    with:
      dest: s3://${BUCKET_NAME}/output
`)
	stdout, stderr, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "DEST=s3://my-bucket/output") {
		t.Errorf("expected .env var interpolated in with, got:\n%s", stdout)
	}
}

func TestDotEnv_NoDotEnvIsOk(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho ok\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
`)
	_, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (no .env should be fine)", code)
	}
}

func TestDotEnv_CommentsIgnored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "# This is a comment\n\nVALID_KEY=works\n# Another comment\n")
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"VALID_KEY=$VALID_KEY\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "VALID_KEY=works") {
		t.Errorf("expected valid key from .env, got:\n%s", stdout)
	}
}

func TestDotEnv_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "DOUBLE=\"hello world\"\nSINGLE='single quoted'\n")
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"D=$DOUBLE S=$SINGLE\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
`)
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "D=hello world") {
		t.Errorf("expected unquoted double value, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "S=single quoted") {
		t.Errorf("expected unquoted single value, got:\n%s", stdout)
	}
}

func TestDotEnv_WithoutPiperigYaml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env", "FROM_ENV=dotenv_only\n")
	writeScript(t, dir, "scripts/echo.sh", "#!/bin/sh\necho \"FROM_ENV=$FROM_ENV\"\n")
	writeFile(t, dir, "test.pipe.yaml", `steps:
  - job: scripts/echo.sh
`)
	// No .piperig.yaml — .env should still work
	stdout, _, code := run(t, dir, "run", "test.pipe.yaml")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "FROM_ENV=dotenv_only") {
		t.Errorf("expected .env to work without .piperig.yaml, got:\n%s", stdout)
	}
}
