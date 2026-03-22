package runner

import (
	"bufio"
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
}

// RunPlan executes all steps in a plan sequentially.
func (r *Runner) RunPlan(ctx context.Context, plan *pipe.Plan) error {
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

	for _, call := range step.Calls {
		r.Output.Start(call.Job, call.Params)

		start := time.Now()
		err := r.executeWithRetry(ctx, call, step)
		dur := time.Since(start)

		if err != nil {
			exitCode := 1
			if re, ok := err.(*pipe.RunError); ok {
				exitCode = re.ExitCode
			}
			r.Output.Fail(call.Job, exitCode, dur)

			if step.AllowFailure {
				continue
			}
			return err
		}
		r.Output.Ok(call.Job, dur)
	}
	return nil
}

func (r *Runner) executeWithRetry(ctx context.Context, call pipe.Call, step pipe.StepPlan) error {
	var lastErr error
	maxAttempts := step.Retry + 1 // retry=3 means 4 total attempts

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = r.RunCall(ctx, call, step.Timeout)
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
func (r *Runner) RunCall(ctx context.Context, call pipe.Call, timeout time.Duration) error {
	// Nested pipe
	if strings.HasSuffix(call.Job, ".pipe.yaml") {
		return r.runNestedPipe(ctx, call)
	}

	// Apply timeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Resolve interpreter
	cmdName, args, err := r.resolveCommand(call.Job)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

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
		r.readStderr(stderr)
		close(done)
	}()
	r.readStdout(stdout)
	<-done

	if err := cmd.Wait(); err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return &pipe.RunError{Job: call.Job, ExitCode: exitCode, Err: err}
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

func (r *Runner) readStdout(rc io.Reader) {
	scanner := bufio.NewScanner(rc)
	logFields := r.Output.Log()
	for scanner.Scan() {
		line := scanner.Text()
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

func (r *Runner) readStderr(rc io.Reader) {
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		r.Output.Stderr(scanner.Text())
	}
}
