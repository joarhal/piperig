package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joarhal/piperig/internal/config"
	"github.com/joarhal/piperig/internal/expand"
	"github.com/joarhal/piperig/internal/output"
	"github.com/joarhal/piperig/internal/pipe"
	"github.com/joarhal/piperig/internal/validate"
)

// maxNestedDepth is the maximum allowed depth for nested pipe execution.
// This prevents infinite recursion when pipes reference each other cyclically.
const maxNestedDepth = 10

// Runner executes expanded plans by invoking subprocesses.
type Runner struct {
	Interpreters map[string]string
	Output       *output.Writer
	Now          time.Time
	Config       *config.Config
	Depth        int
	PipeName     string
}

// RunPlan executes all steps in a plan sequentially.
func (r *Runner) RunPlan(ctx context.Context, plan *pipe.Plan) error {
	r.PipeName = plan.Name
	if plan.Name != "" {
		r.Output.PipeHeader(plan.Name, plan.Description)
	}

	start := time.Now()
	totalCalls := plan.TotalCalls()

	for _, step := range plan.Steps {
		if err := r.RunStep(ctx, step); err != nil {
			r.Output.PipeSummary(totalCalls, time.Since(start), true)
			return err
		}
	}

	r.Output.PipeSummary(totalCalls, time.Since(start), false)
	return nil
}

// RunStep executes all calls in a step with retry and allow_failure handling.
func (r *Runner) RunStep(ctx context.Context, step pipe.StepPlan) error {
	r.Output.SetLog(step.Log)

	hasHooks := step.OnFail != "" || step.OnSuccess != ""

	for _, call := range step.Calls {
		r.Output.Start(call.Job, call.Params)

		var stdoutBuf, stderrBuf *bytes.Buffer
		if hasHooks {
			stdoutBuf = &bytes.Buffer{}
			stderrBuf = &bytes.Buffer{}
		}
		start := time.Now()
		err := r.executeWithRetry(ctx, call, step, stdoutBuf, stderrBuf)
		dur := time.Since(start)

		if err != nil {
			exitCode := 1
			isTimeout := false
			if re, ok := err.(*pipe.RunError); ok {
				exitCode = re.ExitCode
				isTimeout = re.Timeout
			}
			r.Output.Fail(call.Job, exitCode, dur)

			// Fire on_fail hook
			if step.OnFail != "" {
				status := "fail"
				if isTimeout {
					status = "timeout"
				}
				var combined []byte
				if stdoutBuf != nil {
					combined = append(stdoutBuf.Bytes(), stderrBuf.Bytes()...)
				}
				if hookErr := r.runHook(ctx, step.OnFail, call, status, exitCode, dur, combined); hookErr != nil {
					if !step.AllowFailure {
						return hookErr
					}
				}
			}

			if step.AllowFailure {
				continue
			}
			return err
		}

		r.Output.Ok(call.Job, dur)

		// Fire on_success hook
		if step.OnSuccess != "" {
			var combined []byte
			if stdoutBuf != nil {
				combined = append(stdoutBuf.Bytes(), stderrBuf.Bytes()...)
			}
			if hookErr := r.runHook(ctx, step.OnSuccess, call, "success", 0, dur, combined); hookErr != nil {
				return hookErr
			}
		}
	}
	return nil
}

func (r *Runner) executeWithRetry(ctx context.Context, call pipe.Call, step pipe.StepPlan, stdoutBuf, stderrBuf *bytes.Buffer) error {
	var lastErr error
	maxAttempts := step.Retry + 1 // retry=3 means 4 total attempts

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Reset buffers before each attempt — hook gets last attempt's output only
		if stdoutBuf != nil {
			stdoutBuf.Reset()
			stderrBuf.Reset()
		}
		lastErr = r.RunCall(ctx, call, step.Timeout, stdoutBuf, stderrBuf)
		if lastErr == nil {
			return nil
		}
		if attempt < maxAttempts {
			r.Output.Retry(attempt, step.Retry, step.RetryDelay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(step.RetryDelay):
			}
		}
	}
	return lastErr
}

// RunCall invokes a single subprocess or nested pipe.
// stdoutBuf and stderrBuf are optional; when non-nil, output is tee'd into them for hooks.
func (r *Runner) RunCall(ctx context.Context, call pipe.Call, timeout time.Duration, stdoutBuf, stderrBuf *bytes.Buffer) error {
	// Nested pipe
	if strings.HasSuffix(call.Job, ".pipe.yaml") {
		return r.runNestedPipe(ctx, call)
	}

	// Apply timeout
	var timeoutCtx context.Context
	if timeout > 0 {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = timeoutCtx
	}

	// Resolve interpreter
	cmdName, args, err := r.resolveCommand(call.Job)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// Send SIGTERM first to allow graceful cleanup
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second // after SIGTERM, wait 5s then SIGKILL

	// Set parameters by input mode
	switch call.Input {
	case pipe.InputEnv, "":
		cmd.Env = r.buildEnv(call.Params)
	case pipe.InputJSON:
		jsonData, err := json.Marshal(call.Params)
		if err != nil {
			return err
		}
		cmd.Stdin = strings.NewReader(string(jsonData))
		cmd.Env = r.baseEnv()
	case pipe.InputArgs:
		cmd.Args = append(cmd.Args, r.buildArgs(call.Params)...)
		cmd.Env = r.baseEnv()
	}

	// Capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return &pipe.RunError{Job: call.Job, ExitCode: 1, Err: err}
	}

	// Read output concurrently
	done := make(chan struct{})
	go func() {
		r.readStderr(stderr, stderrBuf)
		close(done)
	}()
	r.readStdout(stdout, stdoutBuf)
	<-done

	if err := cmd.Wait(); err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		isTimeout := timeoutCtx != nil && timeoutCtx.Err() == context.DeadlineExceeded
		return &pipe.RunError{Job: call.Job, ExitCode: exitCode, Timeout: isTimeout, Err: err}
	}
	return nil
}

func (r *Runner) runNestedPipe(ctx context.Context, call pipe.Call) error {
	r.Depth++
	defer func() { r.Depth-- }()
	if r.Depth > maxNestedDepth {
		return fmt.Errorf("nested pipe depth limit exceeded (%d)", maxNestedDepth)
	}

	p, err := pipe.Load(call.Job)
	if err != nil {
		return &pipe.RunError{Job: call.Job, ExitCode: 1, Err: err}
	}

	fileExists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	errs := validate.Validate(p, r.Config, fileExists, call.Params)
	if len(errs) > 0 {
		return &pipe.RunError{Job: call.Job, ExitCode: 2, Err: &pipe.ValidationError{Errors: errs}}
	}

	plan, err := expand.Expand(p, call.Params, r.Now)
	if err != nil {
		return &pipe.RunError{Job: call.Job, ExitCode: 1, Err: err}
	}
	plan.Name = filepath.Base(call.Job)

	return r.RunPlan(ctx, plan)
}

func (r *Runner) resolveCommand(job string) (string, []string, error) {
	ext := filepath.Ext(job)
	if ext == "" {
		// Direct exec — use as-is if absolute, prefix ./ if relative
		if filepath.IsAbs(job) {
			return job, nil, nil
		}
		return "./" + job, nil, nil
	}
	interp, ok := r.Interpreters[ext]
	if !ok {
		return "", nil, fmt.Errorf("no interpreter for extension %q", ext)
	}
	// Handle multi-word interpreters like "npx tsx"
	parts := strings.Fields(interp)
	args := append(parts[1:], job)
	return parts[0], args, nil
}

// baseEnv returns os.Environ() with config-level env vars applied (config wins).
func (r *Runner) baseEnv() []string {
	env := os.Environ()
	for k, v := range r.Config.Env {
		env = append(env, k+"="+v)
	}
	return env
}

func (r *Runner) buildEnv(params map[string]string) []string {
	env := r.baseEnv()
	for k, v := range params {
		env = append(env, strings.ToUpper(k)+"="+v)
	}
	return env
}

func (r *Runner) buildArgs(params map[string]string) []string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var args []string
	for _, k := range keys {
		args = append(args, "--"+k, params[k])
	}
	return args
}

func (r *Runner) readStdout(rc io.Reader, buf *bytes.Buffer) {
	scanner := bufio.NewScanner(rc)
	logFields := r.Output.Log()
	for scanner.Scan() {
		line := scanner.Text()
		if buf != nil {
			buf.WriteString(line + "\n")
		}
		if len(logFields) > 0 {
			var obj map[string]any
			if json.Unmarshal([]byte(line), &obj) == nil {
				fields := make(map[string]string)
				for k, v := range obj {
					fields[k] = formatJSONValue(v)
				}
				r.Output.JSON(fields)
				continue
			}
		}
		r.Output.Text(line)
	}
}

// formatJSONValue converts a JSON value to string without scientific notation.
// Go's json.Unmarshal decodes all numbers as float64, so 17144090 becomes
// 1.714409e+07 with fmt.Sprint. This function formats whole numbers as integers.
func formatJSONValue(v any) string {
	if f, ok := v.(float64); ok {
		if f == float64(int64(f)) {
			return fmt.Sprintf("%d", int64(f))
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func (r *Runner) readStderr(rc io.Reader, buf *bytes.Buffer) {
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Text()
		if buf != nil {
			buf.WriteString(line + "\n")
		}
		r.Output.Stderr(line)
	}
}

// runHook executes a hook script with context from the step execution.
func (r *Runner) runHook(ctx context.Context, hookJob string, call pipe.Call, status string, exitCode int, elapsed time.Duration, output []byte) error {
	cmdName, args, err := r.resolveCommand(hookJob)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second

	// Build env: base + PIPERIG_* context + uppercased with params
	env := r.baseEnv()
	env = append(env,
		"PIPERIG_PIPE="+r.PipeName,
		"PIPERIG_STEP="+call.Job,
		"PIPERIG_STATUS="+status,
		fmt.Sprintf("PIPERIG_EXIT_CODE=%d", exitCode),
		fmt.Sprintf("PIPERIG_ELAPSED_MS=%d", elapsed.Milliseconds()),
	)
	for k, v := range call.Params {
		env = append(env, strings.ToUpper(k)+"="+v)
	}
	cmd.Env = env

	// stdin = step's combined stdout+stderr
	cmd.Stdin = bytes.NewReader(output)

	// Capture hook output (no log field formatting)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return &pipe.RunError{Job: hookJob, ExitCode: 1, Err: err}
	}

	done := make(chan struct{})
	go func() {
		r.readStderr(stderr, nil)
		close(done)
	}()
	// Read hook stdout as plain text (no log field formatting)
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		r.Output.Text(scanner.Text())
	}
	<-done

	if err := cmd.Wait(); err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return &pipe.RunError{Job: hookJob, ExitCode: exitCode, Err: err}
	}
	return nil
}
