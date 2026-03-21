package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joarhal/piperig/internal/config"
	"github.com/joarhal/piperig/internal/docs"
	"github.com/joarhal/piperig/internal/expand"
	"github.com/joarhal/piperig/internal/output"
	"github.com/joarhal/piperig/internal/picker"
	"github.com/joarhal/piperig/internal/pipe"
	"github.com/joarhal/piperig/internal/runner"
	"github.com/joarhal/piperig/internal/scheduler"
	"github.com/joarhal/piperig/internal/validate"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "check":
		os.Exit(cmdCheck(os.Args[2:]))
	case "serve":
		os.Exit(cmdServe(os.Args[2:]))
	case "init":
		os.Exit(cmdInit())
	case "new":
		os.Exit(cmdNew(os.Args[2:]))
	case "llm":
		fmt.Print(docs.README)
		os.Exit(0)
	case "version":
		fmt.Println("piperig " + version)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  piperig run <file.pipe.yaml|dir> [key=value ...]
  piperig check <file.pipe.yaml|dir> [key=value ...]
  piperig serve <schedule.yaml> [--now]
  piperig init
  piperig new pipe|schedule <name>
  piperig version`)
}

// parseArgs splits args into target and key=value overrides.
func parseArgs(args []string) (target string, overrides map[string]string) {
	overrides = make(map[string]string)
	if len(args) == 0 {
		return "", overrides
	}
	target = args[0]
	for _, arg := range args[1:] {
		if k, v, ok := strings.Cut(arg, "="); ok {
			overrides[k] = v
		}
	}
	return target, overrides
}

func cmdRun(args []string) int {
	target, overrides := parseArgs(args)
	if target == "" {
		result, err := picker.Pick()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		target = result.Target
		if result.Mode == "check" {
			return cmdCheck([]string{target})
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	w := output.New(os.Stdout, output.StdoutIsTerminal())
	now := time.Now()

	paths, code := resolvePaths(target)
	if code != 0 {
		return code
	}

	for _, path := range paths {
		if code := runSinglePipe(path, cfg, w, overrides, now); code != 0 {
			return code
		}
	}
	return 0
}

func runSinglePipe(path string, cfg *config.Config, w *output.Writer, overrides map[string]string, now time.Time) int {
	p, err := pipe.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		return 1
	}

	fileExists := func(f string) bool {
		_, err := os.Stat(f)
		return err == nil
	}

	errs := validate.Validate(p, cfg, fileExists, overrides)
	if len(errs) > 0 {
		ve := &pipe.ValidationError{Errors: errs}
		fmt.Fprintln(os.Stderr, ve)
		return 2
	}

	plan, err := expand.Expand(p, overrides, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "expand: %v\n", err)
		return 1
	}

	r := &runner.Runner{
		Interpreters: cfg.Interpreters,
		Output:       w,
		Now:          now,
		Config:       cfg,
	}

	if err := r.RunPlan(context.Background(), plan); err != nil {
		if _, ok := err.(*pipe.RunError); ok {
			return 1
		}
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		return 1
	}
	return 0
}

func cmdCheck(args []string) int {
	target, overrides := parseArgs(args)
	if target == "" {
		fmt.Fprintln(os.Stderr, "usage: piperig check <file.pipe.yaml|dir> [key=value ...]")
		return 1
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	w := output.New(os.Stdout, output.StdoutIsTerminal())
	now := time.Now()

	paths, code := resolvePaths(target)
	if code != 0 {
		return code
	}

	for _, path := range paths {
		if code := checkSinglePipe(path, cfg, w, overrides, now); code != 0 {
			return code
		}
	}
	return 0
}

func checkSinglePipe(path string, cfg *config.Config, w *output.Writer, overrides map[string]string, now time.Time) int {
	p, err := pipe.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		return 1
	}

	fileExists := func(f string) bool {
		_, err := os.Stat(f)
		return err == nil
	}

	errs := validate.Validate(p, cfg, fileExists, overrides)
	if len(errs) > 0 {
		ve := &pipe.ValidationError{Errors: errs}
		fmt.Fprintln(os.Stderr, ve)
		return 2
	}

	plan, err := expand.Expand(p, overrides, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "expand: %v\n", err)
		return 1
	}

	name := filepath.Base(path)
	w.CheckPipe(name, plan.Description)
	fmt.Println()
	for i, step := range plan.Steps {
		w.CheckStep(i+1, step)
		for j, call := range step.Calls {
			w.CheckCall(j+1, call.Params)
		}
		fmt.Println()
	}
	w.CheckTotal(plan.TotalCalls())

	return 0
}

func cmdInit() int {
	const filename = ".piperig.yaml"
	if _, err := os.Stat(filename); err == nil {
		fmt.Fprintf(os.Stderr, "%s already exists\n", filename)
		return 1
	}

	cfg := config.Default()
	var lines []string
	lines = append(lines, "interpreters:")
	for ext, cmd := range cfg.Interpreters {
		lines = append(lines, fmt.Sprintf("  %s: %s", ext, cmd))
	}

	if err := os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		return 1
	}

	fmt.Println("Created .piperig.yaml:")
	for ext, cmd := range cfg.Interpreters {
		fmt.Printf("  %s → %s\n", ext, cmd)
	}
	return 0
}

func cmdNew(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: piperig new pipe|schedule <name>")
		return 1
	}

	kind := args[0]
	name := args[1]

	switch kind {
	case "pipe":
		filename := name + ".pipe.yaml"
		if _, err := os.Stat(filename); err == nil {
			fmt.Fprintf(os.Stderr, "%s already exists\n", filename)
			return 1
		}
		if dir := filepath.Dir(filename); dir != "." {
			os.MkdirAll(dir, 0o755)
		}
		content := `description: ""

steps:
  - job: scripts/example.py
`
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
			return 1
		}
		fmt.Printf("Created %s\n", filename)
		return 0

	case "schedule":
		filename := name + ".yaml"
		if _, err := os.Stat(filename); err == nil {
			fmt.Fprintf(os.Stderr, "%s already exists\n", filename)
			return 1
		}
		if dir := filepath.Dir(filename); dir != "." {
			os.MkdirAll(dir, 0o755)
		}
		content := `- name: daily
  cron: "0 5 * * *"
  run:
    - pipes/daily/
`
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
			return 1
		}
		fmt.Printf("Created %s\n", filename)
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown template kind: %s (use pipe or schedule)\n", kind)
		return 1
	}
}

func cmdServe(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: piperig serve <schedule.yaml> [--now]")
		return 1
	}

	schedFile := args[0]
	nowMode := false
	for _, a := range args[1:] {
		if a == "--now" {
			nowMode = true
		}
	}

	entries, err := scheduler.LoadSchedule(schedFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load schedule: %v\n", err)
		return 1
	}

	errs := scheduler.ValidateEntries(entries)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	w := output.New(os.Stdout, output.StdoutIsTerminal())

	if nowMode {
		if err := scheduler.ServeNow(entries, cfg, w); err != nil {
			if _, ok := err.(*pipe.ValidationError); ok {
				fmt.Fprintln(os.Stderr, err)
				return 2
			}
			return 1
		}
		return 0
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("piperig serve: %d schedule entries\n", len(entries))
	if err := scheduler.Serve(ctx, entries, cfg, w); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}

func resolvePaths(target string) ([]string, int) {
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "not found: %s\n", target)
		return nil, 1
	}

	if info.IsDir() {
		paths, err := pipe.Scan(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			return nil, 1
		}
		if len(paths) == 0 {
			fmt.Fprintf(os.Stderr, "no .pipe.yaml files found in %s\n", target)
			return nil, 1
		}
		return paths, 0
	}

	return []string{target}, 0
}
