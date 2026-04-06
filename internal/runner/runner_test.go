package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joarhal/piperig/internal/config"
	"github.com/joarhal/piperig/internal/output"
	"github.com/joarhal/piperig/internal/pipe"
)

func scriptPath(name string) string {
	return filepath.Join("testdata", "scripts", name)
}

func newTestRunner() (*Runner, *bytes.Buffer) {
	var buf bytes.Buffer
	w := output.New(&buf, false)
	return &Runner{
		Interpreters: config.Default().Interpreters,
		Output:       w,
		Now:          time.Now(),
		Config:       config.Default(),
	}, &buf
}

func TestRunCallSuccess(t *testing.T) {
	r, _ := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job: scriptPath("exit0.sh"), Input: pipe.InputEnv,
	}, 0, nil, nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestRunCallFailure(t *testing.T) {
	r, _ := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job: scriptPath("exit1.sh"), Input: pipe.InputEnv,
	}, 0, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	re, ok := err.(*pipe.RunError)
	if !ok {
		t.Fatalf("expected RunError, got %T", err)
	}
	if re.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", re.ExitCode)
	}
}

func TestRunCallEnvMode(t *testing.T) {
	r, buf := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job:    scriptPath("echo_env.sh"),
		Params: map[string]string{"src": "/data", "quality": "80"},
		Input:  pipe.InputEnv,
	}, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "SRC=/data") {
		t.Errorf("expected SRC=/data in output, got:\n%s", out)
	}
	if !strings.Contains(out, "QUALITY=80") {
		t.Errorf("expected QUALITY=80 in output, got:\n%s", out)
	}
}

func TestRunCallJSONMode(t *testing.T) {
	r, buf := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job:    scriptPath("read_json.sh"),
		Params: map[string]string{"src": "/data"},
		Input:  pipe.InputJSON,
	}, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "src") {
		t.Errorf("expected JSON with 'src' in output, got:\n%s", out)
	}
}

func TestRunCallArgsMode(t *testing.T) {
	r, buf := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job:    scriptPath("echo_args.sh"),
		Params: map[string]string{"key": "value"},
		Input:  pipe.InputArgs,
	}, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "--key") || !strings.Contains(out, "value") {
		t.Errorf("expected --key value in output, got:\n%s", out)
	}
}

func TestRunCallTimeout(t *testing.T) {
	r, _ := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job: scriptPath("slow.sh"), Input: pipe.InputEnv,
	}, 500*time.Millisecond, nil, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRunStepRetrySuccess(t *testing.T) {
	// Setup counter file
	counter := filepath.Join(t.TempDir(), "counter")
	r, _ := newTestRunner()
	step := pipe.StepPlan{
		Job: scriptPath("flaky.sh"),
		Calls: []pipe.Call{{
			Job:    scriptPath("flaky.sh"),
			Params: map[string]string{"fail_count": "2", "counter_file": counter},
			Input:  pipe.InputEnv,
		}},
		Retry:      3,
		RetryDelay: 10 * time.Millisecond,
	}
	err := r.RunStep(context.Background(), step)
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
}

func TestRunStepRetryExhausted(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "counter")
	r, _ := newTestRunner()
	step := pipe.StepPlan{
		Job: scriptPath("flaky.sh"),
		Calls: []pipe.Call{{
			Job:    scriptPath("flaky.sh"),
			Params: map[string]string{"fail_count": "10", "counter_file": counter},
			Input:  pipe.InputEnv,
		}},
		Retry:      2,
		RetryDelay: 10 * time.Millisecond,
	}
	err := r.RunStep(context.Background(), step)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
}

func TestRunStepAllowFailure(t *testing.T) {
	r, _ := newTestRunner()
	step := pipe.StepPlan{
		Job: scriptPath("exit1.sh"),
		Calls: []pipe.Call{{
			Job: scriptPath("exit1.sh"), Input: pipe.InputEnv,
		}},
		AllowFailure: true,
	}
	err := r.RunStep(context.Background(), step)
	if err != nil {
		t.Fatalf("allow_failure should swallow error, got %v", err)
	}
}

func TestRunPlanFailFast(t *testing.T) {
	r, _ := newTestRunner()
	plan := &pipe.Plan{
		Steps: []pipe.StepPlan{
			{
				Job:   scriptPath("exit1.sh"),
				Calls: []pipe.Call{{Job: scriptPath("exit1.sh"), Input: pipe.InputEnv}},
			},
			{
				Job:   scriptPath("exit0.sh"),
				Calls: []pipe.Call{{Job: scriptPath("exit0.sh"), Input: pipe.InputEnv}},
			},
		},
	}
	err := r.RunPlan(context.Background(), plan)
	if err == nil {
		t.Fatal("expected fail fast error")
	}
}

func TestStdoutTextAndJSON(t *testing.T) {
	r, buf := newTestRunner()
	r.Output.SetLog([]string{"label", "file", "size"})
	err := r.RunCall(context.Background(), pipe.Call{
		Job: scriptPath("json_lines.sh"), Input: pipe.InputEnv,
	}, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Should have text lines
	if !strings.Contains(out, "· starting") {
		t.Errorf("expected text 'starting', got:\n%s", out)
	}
	// Should have JSON formatted line
	if !strings.Contains(out, "▸") {
		t.Errorf("expected JSON marker ▸, got:\n%s", out)
	}
	if !strings.Contains(out, "fullhd") {
		t.Errorf("expected 'fullhd' in JSON output, got:\n%s", out)
	}
}

func TestStderr(t *testing.T) {
	r, buf := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job: scriptPath("stderr.sh"), Input: pipe.InputEnv,
	}, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "! error message") {
		t.Errorf("expected stderr output, got:\n%s", out)
	}
}

func TestRunPlanSetsLog(t *testing.T) {
	r, _ := newTestRunner()
	plan := &pipe.Plan{
		Steps: []pipe.StepPlan{
			{
				Job:   scriptPath("exit0.sh"),
				Log:   []string{"label", "file"},
				Calls: []pipe.Call{{Job: scriptPath("exit0.sh"), Input: pipe.InputEnv}},
			},
		},
	}
	_ = r.RunPlan(context.Background(), plan)
	if len(r.Output.Log()) != 2 {
		t.Errorf("expected log fields set, got %v", r.Output.Log())
	}
}

func TestStepLogOverridesPipeLog(t *testing.T) {
	r, buf := newTestRunner()
	plan := &pipe.Plan{
		Steps: []pipe.StepPlan{
			{
				Job:   scriptPath("json_lines.sh"),
				Log:   []string{"label"},
				Calls: []pipe.Call{{Job: scriptPath("json_lines.sh"), Input: pipe.InputEnv}},
			},
		},
	}
	err := r.RunPlan(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Should have JSON marker with only "label" field
	if !strings.Contains(out, "▸") {
		t.Errorf("expected JSON marker ▸, got:\n%s", out)
	}
	if !strings.Contains(out, "fullhd") {
		t.Errorf("expected 'fullhd' in output, got:\n%s", out)
	}
}

func TestRunNestedDepthLimit(t *testing.T) {
	r, _ := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job:   "testdata/pipes/self.pipe.yaml",
		Input: pipe.InputEnv,
	}, 0, nil, nil)
	if err == nil {
		t.Fatal("expected depth limit error")
	}
	if !strings.Contains(err.Error(), "nested pipe depth limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigEnvInjected(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, false)
	cfg := config.Default()
	cfg.Env["PYTHONPATH"] = "/custom/path"
	cfg.Env["CUSTOM_VAR"] = "hello"

	r := &Runner{
		Interpreters: cfg.Interpreters,
		Output:       w,
		Now:          time.Now(),
		Config:       cfg,
	}

	err := r.RunCall(context.Background(), pipe.Call{
		Job: scriptPath("print_env.sh"), Input: pipe.InputEnv,
	}, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "PYTHONPATH=/custom/path") {
		t.Errorf("expected PYTHONPATH=/custom/path in output, got:\n%s", out)
	}
	if !strings.Contains(out, "CUSTOM_VAR=hello") {
		t.Errorf("expected CUSTOM_VAR=hello in output, got:\n%s", out)
	}
}

func TestConfigEnvWithJSONMode(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, false)
	cfg := config.Default()
	cfg.Env["CUSTOM_VAR"] = "from_config"

	r := &Runner{
		Interpreters: cfg.Interpreters,
		Output:       w,
		Now:          time.Now(),
		Config:       cfg,
	}

	err := r.RunCall(context.Background(), pipe.Call{
		Job:    scriptPath("print_env.sh"),
		Params: map[string]string{"src": "/data"},
		Input:  pipe.InputJSON,
	}, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "CUSTOM_VAR=from_config") {
		t.Errorf("expected CUSTOM_VAR=from_config in JSON mode, got:\n%s", out)
	}
}

func TestConfigEnvWithArgsMode(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, false)
	cfg := config.Default()
	cfg.Env["CUSTOM_VAR"] = "from_config"

	r := &Runner{
		Interpreters: cfg.Interpreters,
		Output:       w,
		Now:          time.Now(),
		Config:       cfg,
	}

	err := r.RunCall(context.Background(), pipe.Call{
		Job:    scriptPath("print_env.sh"),
		Params: map[string]string{"key": "value"},
		Input:  pipe.InputArgs,
	}, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "CUSTOM_VAR=from_config") {
		t.Errorf("expected CUSTOM_VAR=from_config in args mode, got:\n%s", out)
	}
}

func TestDirectExecNoExtension(t *testing.T) {
	// Create an executable without extension
	dir := t.TempDir()
	binPath := filepath.Join(dir, "mybin")
	os.WriteFile(binPath, []byte("#!/bin/bash\nexit 0\n"), 0o755)

	r, _ := newTestRunner()
	err := r.RunCall(context.Background(), pipe.Call{
		Job: binPath, Input: pipe.InputEnv,
	}, 0, nil, nil)
	if err != nil {
		t.Fatalf("direct exec should work, got %v", err)
	}
}

func TestTimeoutSendsSIGTERM(t *testing.T) {
	r, buf := newTestRunner()
	start := time.Now()
	err := r.RunCall(context.Background(), pipe.Call{
		Job:   scriptPath("trap_term.sh"),
		Input: pipe.InputEnv,
	}, 500*time.Millisecond, nil, nil)
	dur := time.Since(start)

	// Error expected — context deadline exceeded
	if err == nil {
		t.Fatal("expected error from timeout")
	}
	// Must complete quickly (SIGTERM handled), not wait 30s
	if dur > 5*time.Second {
		t.Errorf("took %v, expected quick exit after SIGTERM", dur)
	}
	// Script should receive SIGTERM, not SIGKILL
	out := buf.String()
	if !strings.Contains(out, "SIGTERM received") {
		t.Errorf("expected script to receive SIGTERM, got:\n%s", out)
	}
}

func TestContextCancelSendsSIGTERM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r, buf := newTestRunner()

	done := make(chan error, 1)
	go func() {
		done <- r.RunCall(ctx, pipe.Call{
			Job:   scriptPath("trap_term.sh"),
			Input: pipe.InputEnv,
		}, 0, nil, nil)
	}()

	// Let the process start, then cancel
	time.Sleep(200 * time.Millisecond)
	cancel()

	start := time.Now()
	err := <-done
	dur := time.Since(start)

	// Error expected — context canceled
	if err == nil {
		t.Fatal("expected error from context cancel")
	}
	// Must complete quickly
	if dur > 5*time.Second {
		t.Errorf("took %v, expected quick exit after SIGTERM", dur)
	}
	out := buf.String()
	if !strings.Contains(out, "SIGTERM received") {
		t.Errorf("expected script to receive SIGTERM, got:\n%s", out)
	}
}
