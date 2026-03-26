# Architecture Overview

Internal structure of piperig. Read after [SPEC.md](SPEC.md).

## 1. Project Structure

```
piperig/
├── embed.go                     # go:embed README.md for `piperig llm`
├── cmd/
│   └── piperig/
│       └── main.go              # CLI entrypoint: subcommands, wiring, exit codes
├── internal/
│   ├── pipe/                    # Types (Pipe, Step, Call, Plan) + YAML parsing + scan
│   │   ├── pipe.go              # Data model
│   │   ├── parse.go             # Load(path) → *Pipe, YAML normalization → string
│   │   ├── scan.go              # Scan(dir) → []string — glob search for .pipe.yaml
│   │   └── errors.go            # ValidationError, RunError
│   ├── timeexpr/                # Time expressions (-1d, -2h, -1w, ranges)
│   │   └── timeexpr.go          # Resolve(), ExpandRange(), IsTimeExpr()
│   ├── expand/                  # Expansion of loop/each/templates → Plan
│   │   └── expand.go            # Expand(pipe, overrides, now) → *Plan
│   ├── validate/                # 10 validation rules, before execution
│   │   └── validate.go          # Validate(pipe, config, fileExists) → []error
│   ├── runner/                  # Subprocess execution
│   │   └── runner.go            # RunPlan → RunStep → RunCall
│   ├── output/                  # Formatted terminal output
│   │   ├── output.go            # Writer with Start/Text/JSON/Ok/Fail + Check* methods
│   │   └── color.go             # ANSI colors, isatty via inline syscall
│   ├── scheduler/               # Cron + every, schedule YAML
│   │   └── scheduler.go         # Serve(), ServeNow()
│   ├── picker/                  # TUI interactive mode (bubbletea)
│   │   └── picker.go            # Fuzzy search, run/check mode toggle
│   └── config/                  # .piperig.yaml + .env loading
│       ├── config.go            # Load(), Default()
│       └── dotenv.go            # .env file parser
├── test/
│   └── e2e_test.go              # Full cycle: binary → stdout/exit code
├── testdata/                    # Test .pipe.yaml files and scripts
│   ├── *.pipe.yaml              # Example pipes for tests
│   └── scripts/                 # Test job scripts
├── docs/
│   ├── SPEC.md                  # Source of truth: format, API, examples
│   ├── ARCHITECTURE.md          # This file
│   ├── TESTING.md               # Test strategy, cases, fixtures
│   ├── log_example.sh           # Example of formatted log output
│   └── tui_example.sh           # Example of TUI picker
├── go.mod
├── go.sum
└── CLAUDE.md                    # Development rules
```

## 2. High-Level System Diagram

### Data flow

```
                        .pipe.yaml
                            │
                      [pipe/Load]
                            │
                         *Pipe
                            │
                   [validate/Validate] ←── CLI overrides (for template validation)
                            │
                         ok/error
                            │ ok
                   [expand/Expand] ←── CLI overrides + now time
                            │
                         *Plan
                            │
              ┌─────────────┴─────────────┐
              ▼                           ▼
     [output/Check*]               [runner/RunPlan]
                                          │
                                   [output/Writer]
```

`check` and `run` receive the same `*Plan`. The difference is that check prints via `output.Check*`, while run executes via `runner/`.

### Directory and scan

When running a directory or TUI, `pipe.Scan(dir)` finds all `.pipe.yaml` files via glob (`**/*.pipe.yaml`) and returns a flat sorted list of paths. No recursive traversal -- a single glob query.

### Nested pipes

If a step references a `.pipe.yaml` file, `runner/` goes through the full cycle for the child pipe:

```
RunCall(step with .pipe.yaml)
    → pipe.Load(child.pipe.yaml)
    → validate.Validate(childPipe)
    → expand.Expand(childPipe, parentWith, now)
    → runner.RunPlan(childPlan)
```

Parent `with` is passed as overrides -- parent wins (overrides have highest priority).

### Package dependency graph

```
                    cmd/piperig
                         │
        ┌────┬───────┬───┴───┬──────────┬────────┐
        ▼    ▼       ▼       ▼          ▼        ▼
     pipe/ config/ validate/ expand/ runner/ scheduler/
                     │        │       │  │       │
                     ▼        ▼       ▼  ▼       ▼
                  pipe/     pipe/  pipe/ output/ pipe/
                  config/   timeexpr/  config/  validate/
                                               expand/
                                               runner/
```

No cycles. Leaf packages (no in-project dependencies): `pipe/`, `timeexpr/`, `config/`, `output/`.

## 3. Core Components

### 3.1. pipe/ -- data model, parsing, scan

Types, YAML loading, pipe file discovery. No business logic.

```go
type Pipe struct {
    Description string            `yaml:"description"`
    With        map[string]string `yaml:"with"`
    Loop        map[string]any    `yaml:"loop"`
    Each        []map[string]string `yaml:"each"`
    Input       InputMode         `yaml:"input"`
    Log         []string          `yaml:"log"`
    Retry       *int              `yaml:"retry"`
    RetryDelay  string            `yaml:"retry_delay"`
    Timeout     string            `yaml:"timeout"`
    Steps       []Step            `yaml:"steps"`
}

type Step struct {
    Job          string            `yaml:"job"`
    With         map[string]string `yaml:"with"`
    Loop         map[string]any    `yaml:"loop"`
    LoopOff      bool              // true when YAML contains "loop: false"
    Each         []map[string]string `yaml:"each"`
    EachOff      bool              // true when YAML contains "each: false"
    Input        InputMode         `yaml:"input"`
    Retry        *int              `yaml:"retry"`
    RetryOff     bool              // true when YAML contains "retry: false"
    RetryDelay   string            `yaml:"retry_delay"`
    Timeout      string            `yaml:"timeout"`
    AllowFailure bool             `yaml:"allow_failure"`
}
```

**Call** is the central type of the entire system. A fully resolved invocation of a single job:

```go
type Call struct {
    Job    string
    Params map[string]string
    Input  InputMode
}
```

**StepPlan** and **Plan** are the result of expand:

```go
type StepPlan struct {
    Job          string
    Calls        []Call
    Dims         string        // e.g. "4 each × 2 dates" — for check output
    Retry        int
    RetryDelay   time.Duration
    Timeout      time.Duration
    AllowFailure bool
}

type Plan struct {
    Description string
    Log         []string
    Steps       []StepPlan
}

func (p *Plan) TotalCalls() int  // computed, not stored
```

**Normalization during parsing**: YAML values (`quality: 80`, `enabled: true`) are converted to `string` via `fmt.Sprint()`. For `Step.Loop`, `Step.Each`, `Step.Retry` which can be `false` -- custom `UnmarshalYAML`: if `false` then `LoopOff`/`EachOff`/`RetryOff = true`, if a value is present it is parsed normally.

**Scan** -- pipe file discovery:

```go
// Scan finds all .pipe.yaml files in a directory via glob. Returns a flat
// sorted list of paths. Used in: piperig run dir/, check dir/, picker.
func Scan(dir string) ([]string, error)
```

A single glob query `**/*.pipe.yaml`, no manual recursion.

**Errors** -- two types for distinguishing exit codes in `main.go`:

```go
type ValidationError struct { Errors []error }  // exit 2
type RunError struct { Job string; ExitCode int; Err error }  // exit 1
```

### 3.2. timeexpr/ -- time expressions

Pure functions, sole dependency is `time.Time`.

```go
func Resolve(expr string, now time.Time) (string, error)     // "-1d" → "2026-03-18"
func IsTimeExpr(expr string) bool                             // "-1d" → true
func ExpandRange(expr string, now time.Time) ([]string, error) // "-2d..-1d" → ["...", "..."]
func IsRange(expr string) bool                                 // "-2d..-1d" → true
```

Suffixes: `d` (days), `h` (hours), `m` (minutes), `s` (seconds), `w` (weeks/Mondays). Rounds down for idempotency.

### 3.3. expand/ -- system core

One function, pure logic, zero I/O:

```go
func Expand(p *pipe.Pipe, overrides map[string]string, now time.Time) (*pipe.Plan, error)
```

Order of operations:

1. **Time expressions** -- resolves all time expressions in `with`, `each`, `loop`, step `with` via `timeexpr.Resolve()`
2. **Loop expansion** -- expands each loop key into a list of values:
   - Time range (`-2d..-1d`) via `timeexpr.ExpandRange()`
   - Absolute date range (`2026-03-01..2026-03-05`) via `timeexpr.ExpandRange()`
   - Numeric range (`1..5`) generates a list `["1","2","3","4","5"]` (logic inside expand/, not timeexpr/)
   - Explicit list (`[eu, us, asia]`) as-is
3. **Cartesian product** -- `each x loop`
4. **Per-step logic** -- `EachOff`/`LoopOff` disables parent iteration, step-level `loop`/`each` replaces parent
5. **Merge** -- priority: `pipe.With` < `each item` < `loop value` < `step.With` < `overrides`
6. **Templates** -- `{key}` in values is substituted from merged params
7. **Nested pipe steps** -- if job ends with `.pipe.yaml`, skip loop/each expansion, produce 1 Call with merged `with` params (validation already rejected loop/each on such steps)
8. **Call formation** -- job + merged params + input mode (step then pipe then env)
9. **Policies** -- retry (respecting `RetryOff`), timeout, allow_failure: step then pipe then defaults

### 3.4. validate/ -- validation

```go
func Validate(p *pipe.Pipe, cfg *config.Config, fileExists func(string) bool, overrides map[string]string) []error
```

10 rules from SPEC.md. Each rule is a separate internal function. `fileExists` callback: in production uses `os.Stat`, in tests uses `func(string) bool { return true }`. `overrides` are CLI key=value pairs or schedule with, needed for template validation (rule 8). Nested `.pipe.yaml` files trigger recursive `Load()` + `Validate()`.

### 3.5. runner/ -- execution

```go
type Runner struct {
    Interpreters map[string]string  // ".py" → "python3.11"
    Output       *output.Writer
}

func (r *Runner) RunPlan(ctx context.Context, plan *pipe.Plan) error
func (r *Runner) RunStep(ctx context.Context, step *pipe.StepPlan) error
func (r *Runner) RunCall(ctx context.Context, call *pipe.Call) error
```

`RunPlan` iterates steps, `RunStep` iterates calls + retry/timeout/allow_failure, `RunCall` invokes `exec.CommandContext`.

Extension to interpreter mapping. Three input modes: env (UPPERCASE keys), json (stdin), args (--key value). stdout is read line by line -- JSON lines are formatted via `output.Writer`, everything else as-is.

Nested pipes: `RunCall` goes through `Load -> Validate -> Expand -> RunPlan`.

`RunPlan` sets `Output.log` from `plan.Log` before execution -- each pipe can have its own `log` fields.

### 3.6. output/ -- terminal output

```go
type Writer struct {
    w     io.Writer
    color bool      // true if stdout is a terminal
    log   []string  // fields for JSON formatting, set per-pipe
}

// Methods for run-time output:
func (w *Writer) Start(job string, params map[string]string)        // → white/bold
func (w *Writer) Text(line string)                                   // · gray
func (w *Writer) JSON(fields map[string]string)                      // ▸ cyan
func (w *Writer) Stderr(line string)                                 // ! yellow
func (w *Writer) Retry(attempt, max int, delay time.Duration)        // ↻ yellow
func (w *Writer) Ok(job string, duration time.Duration)              // ✓ green
func (w *Writer) Fail(job string, exitCode int, dur time.Duration)   // ✗ red

func (w *Writer) SetLog(fields []string)                             // per-pipe reconfiguration

// Methods for check output:
func (w *Writer) CheckPipe(name, description string)                 // pipe header
func (w *Writer) CheckStep(n int, step pipe.StepPlan)                     // Step N: job × dims = N calls
func (w *Writer) CheckCall(n int, params map[string]string)          // N. key=value key=value
func (w *Writer) CheckTotal(total int)                               // Total: N calls
```

Terminal detection via inline syscall `TIOCGWINSZ` (~10 lines, darwin/linux), no external dependencies.

### 3.7. scheduler/

```go
type Schedule struct {
    Name  string            `yaml:"name"`
    Cron  string            `yaml:"cron"`
    Every string            `yaml:"every"`
    Run   []string          `yaml:"run"`
    With  map[string]string `yaml:"with"`
}
```

Cron parsing via a library. `Serve()` runs an infinite cron loop. `ServeNow()` runs once and exits (`--now`).

When running a pipe from a schedule, `schedule.With` is passed as overrides to `expand.Expand()`. Priority is the same as CLI `key=value` -- it overrides everything in pipe YAML.

### 3.8. picker/ -- TUI

bubbletea. Uses `pipe.Scan()` to get the list of `.pipe.yaml` files and directories. Fuzzy search by path, two modes (run/check) via left/right arrows, Enter to execute.

### 3.9. config/ -- .piperig.yaml

```go
type Config struct {
    Interpreters map[string]string `yaml:"interpreters"`
}

func Load() (*Config, error)  // looks for .piperig.yaml in cwd
func Default() *Config        // default interpreters
```

### 3.10. cmd/piperig/ -- CLI wiring

`main.go` wires packages together and implements subcommands:

| Command | Logic |
|---|---|
| `run <file>` | Load -> Validate -> Expand(overrides) -> RunPlan |
| `run <dir>` | Scan -> for each pipe: Load -> Validate -> Expand -> RunPlan (fail fast) |
| `run` (no args) | -> picker/ |
| `check <file>` | Load -> Validate -> Expand(overrides) -> output.Check* |
| `check <dir>` | Scan -> for each pipe: Load -> Validate -> Expand -> output.Check* |
| `list [dir]` | Scan -> for each pipe: Load -> print path + description (skip hidden) |
| `serve` | -> scheduler/ |
| `init` | config.Default() -> write .piperig.yaml |
| `new pipe` | write a .pipe.yaml template |
| `new schedule` | write a schedule.yaml template |
| `version` | print version |

Exit codes: `ValidationError` -> 2, `RunError` -> 1, nil -> 0.

## 4. External Dependencies

| Dependency | Purpose |
|---|---|
| `gopkg.in/yaml.v3` | YAML parsing |
| `github.com/charmbracelet/bubbletea` | TUI picker |
| cron library (tbd) | Cron expression parsing in scheduler |

Everything else is stdlib. isatty detection via inline syscall, no dependency.

## 5. Development & Testing

Detailed testing strategy, all cases and fixtures: [TESTING.md](TESTING.md).

### Running tests

```bash
go test ./...                              # all tests
go test ./test/                            # e2e only
go test -coverprofile=coverage.out ./...   # with coverage
go tool cover -func=coverage.out           # summary
```

## 6. Implementation Order

Bottom-up following the dependency graph:

1. **pipe/** -- types, parsing, scan. Foundation.
2. **timeexpr/** -- extension of the current `dates/`.
3. **config/** -- simple.
4. **expand/** -- system core. Depends on 1, 2.
5. **validate/** -- depends on 1, 3.
6. **output/** -- independent, can be done in parallel with 4-5.
7. **runner/** -- depends on 1, 3, 6.
8. **cmd/piperig/** -- wiring, subcommands `check`, `run`, `init`, `new`.
9. **scheduler/** -- depends on almost everything, but simple.
10. **picker/** -- TUI, last.

## 7. Glossary

| Term | Meaning |
|---|---|
| **Pipe** | A `.pipe.yaml` file -- declarative description of what to run |
| **Job** | An executable file (`.py`, `.sh`, binary) -- path from project root |
| **Step** | A single step inside a pipe: reference to a job + parameters + policies |
| **Call** | A fully resolved invocation: job + all merged params + input mode |
| **Plan** | The result of expand: a list of StepPlan, each containing a list of Call |
| **with** | Parameters passed to jobs |
| **loop** | Iteration over values (cartesian product when multiple keys) |
| **each** | Iteration over parameter sets (array of objects) |
| **input** | Method of passing parameters to a job: `env`, `json`, `args` |
| **Schedule** | A schedule YAML file: when to run pipes |
| **Scan** | Glob search for `.pipe.yaml` files in a directory |

## 8. Project Identification

| | |
|---|---|
| Project | piperig |
| Description | Declarative pipeline runner. Single binary. |
| Language | Go |
| Last Updated | 2026-03-26 |
