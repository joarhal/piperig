# Testing

Testing strategy for piperig. Read after [ARCHITECTURE.md](ARCHITECTURE.md).

## Structure

```
piperig/
├── internal/
│   ├── pipe/
│   │   ├── parse_test.go        # YAML parsing, normalization, UnmarshalYAML
│   │   └── scan_test.go         # glob search for .pipe.yaml
│   ├── timeexpr/
│   │   └── timeexpr_test.go     # all suffixes, ranges, edge cases
│   ├── expand/
│   │   └── expand_test.go       # core: loop, each, templates, merge, policies
│   ├── validate/
│   │   └── validate_test.go     # each rule separately
│   ├── runner/
│   │   └── runner_test.go       # integration tests with real scripts
│   ├── output/
│   │   └── output_test.go       # snapshot: run-time + check output
│   └── scheduler/
│       └── scheduler_test.go    # cron parsing, --now
├── test/
│   └── e2e_test.go              # full cycle: binary → stdout/exit code
└── testdata/
    ├── pipes/                   # test .pipe.yaml files
    │   ├── minimal.pipe.yaml
    │   ├── with_only.pipe.yaml
    │   ├── loop_dates.pipe.yaml
    │   ├── loop_numeric.pipe.yaml
    │   ├── loop_list.pipe.yaml
    │   ├── each.pipe.yaml
    │   ├── each_loop.pipe.yaml
    │   ├── each_false.pipe.yaml
    │   ├── loop_false.pipe.yaml
    │   ├── templates.pipe.yaml
    │   ├── multi_step.pipe.yaml
    │   ├── nested_parent.pipe.yaml
    │   ├── nested_child.pipe.yaml
    │   ├── input_json.pipe.yaml
    │   ├── input_args.pipe.yaml
    │   ├── retry.pipe.yaml
    │   ├── timeout.pipe.yaml
    │   ├── allow_failure.pipe.yaml
    │   ├── invalid/              # invalid pipes for validation tests
    │   │   ├── unknown_key.pipe.yaml
    │   │   ├── bad_extension.pipe.yaml
    │   │   ├── nested_with.pipe.yaml
    │   │   ├── loop_on_nested.pipe.yaml
    │   │   └── bad_template.pipe.yaml
    │   └── schedule.yaml         # test schedule
    └── scripts/                  # fixtures for runner tests
        ├── exit0.sh              # exit 0
        ├── exit1.sh              # exit 1
        ├── echo_env.py           # prints env vars as JSON
        ├── read_json.py          # reads stdin JSON, prints it
        ├── echo_args.sh          # prints "$@"
        ├── slow.sh               # sleep 10s (for timeout)
        ├── flaky.sh              # fails N times, then OK (for retry)
        ├── json_lines.py         # mix of text + JSON lines
        └── stderr.sh             # writes to stderr
```

## Running

```bash
go test ./...                              # all unit + integration
go test ./test/                            # e2e only
go test -coverprofile=coverage.out ./...   # with coverage
go tool cover -func=coverage.out           # per-function summary
go tool cover -html=coverage.out           # visual report
```

## Target coverage

| Package | Target | Why |
|---|---|---|
| `timeexpr/` | 100% | pure functions, easy to cover |
| `expand/` | 95%+ | core logic, all combinations |
| `validate/` | 95%+ | each rule individually |
| `pipe/` | 90%+ | parsing + scan |
| `output/` | 90%+ | snapshots |
| `runner/` | 80%+ | processes are harder to mock |
| `scheduler/` | 80%+ | cron + timing |

## By package

### timeexpr/

Table-driven. Each test receives a fixed `now` — deterministic.

```go
func TestResolve(t *testing.T) {
    now := time.Date(2026, 3, 19, 11, 43, 25, 0, time.UTC)
    tests := []struct{ expr, want string }{...}
}
```

**Resolve cases:**

| Expression | now | Result | What is tested |
|---|---|---|---|
| `-1d` | 2026-03-19 | `2026-03-18` | basic day |
| `0d` | 2026-03-19 | `2026-03-19` | today |
| `1d` | 2026-03-19 | `2026-03-20` | tomorrow |
| `+1d` | 2026-03-19 | `2026-03-20` | explicit plus |
| `-2h` | ..T11:43:25 | `2026-03-19T09:00:00` | round down to hour |
| `0h` | ..T11:43:25 | `2026-03-19T11:00:00` | current hour |
| `-30m` | ..T11:43:25 | `2026-03-19T11:13:00` | 30 minutes ago, rounded |
| `-10s` | ..T11:43:25 | `2026-03-19T11:43:15` | seconds |
| `-1w` | 2026-03-19 (Wed) | `2026-03-09` | previous Monday |
| `0w` | 2026-03-19 (Wed) | `2026-03-16` | current Monday |
| `-1d` | 2026-03-01 | `2026-02-28` | month boundary |
| `-1d` | 2026-01-01 | `2025-12-31` | year boundary |
| `-1d` | 2024-03-01 | `2024-02-29` | leap year |

**ExpandRange cases:**

| Expression | Result | What is tested |
|---|---|---|
| `-2d..-1d` | 2 days | basic range |
| `-1d..-1d` | 1 day | identical boundaries |
| `-24h..-1h` | 24 hours | hourly range |
| `-4w..-1w` | 4 Mondays | weekly range |
| `2026-03-01..2026-03-03` | 3 days | absolute range |
| `-1d..-3d` | error | inverted range |

**IsTimeExpr cases:**

| Input | Result | What is tested |
|---|---|---|
| `-1d` | true | |
| `0h` | true | |
| `+2w` | true | |
| `hello` | false | regular string |
| `1920x1080` | false | must not be confused with time expr |
| `123` | false | bare number |
| `` | false | empty string |
| `-1x` | false | unknown suffix |

### pipe/ parse

Loads YAML from `testdata/pipes/`. Verifies struct fields after parsing.

**Cases:**

| File | What is tested |
|---|---|
| `minimal.pipe.yaml` | steps only, everything else nil/zero |
| `with_only.pipe.yaml` | With is populated, `quality: 80` → `"80"` (int→string) |
| `each.pipe.yaml` | Each is parsed as `[]map[string]string` |
| `each_false.pipe.yaml` | Step.EachOff = true, Step.Each = nil |
| `loop_false.pipe.yaml` | Step.LoopOff = true, Step.Loop = nil |
| `retry.pipe.yaml` | Step.Retry = &3, other Step.RetryOff = true |
| `input_json.pipe.yaml` | Pipe.Input = "json", Step.Input = "args" |
| `with_only.pipe.yaml` | description field parsed correctly |
| `with_only.pipe.yaml` | log: ["label", "file"] parsed as []string |

**Scalar normalization:**

| YAML | Go type yaml.v3 | Expected in map[string]string |
|---|---|---|
| `quality: 80` | int | `"80"` |
| `enabled: true` | bool | `"true"` |
| `ratio: 0.5` | float64 | `"0.5"` |
| `name: hello` | string | `"hello"` |
| `empty:` | null | `""` (or absent — to be decided) |

### pipe/ scan

Uses `t.TempDir()` to create directory structures in tests.

**Cases:**

| Scenario | Expected |
|---|---|
| `a.pipe.yaml`, `b.pipe.yaml` | `["a.pipe.yaml", "b.pipe.yaml"]` — alphabetical |
| `sub/c.pipe.yaml` | found via glob |
| `deep/sub/d.pipe.yaml` | found at any depth |
| `config.yaml`, `docker-compose.yaml` | ignored |
| empty directory | `[]`, not an error |
| file instead of directory | error |

### expand/

Table-driven. Each test: `Pipe struct` + `overrides` + `now` → expected `*Plan` (check Calls).

**Basic:**

| Case | Pipe | Expected |
|---|---|---|
| with only | `with: {a: 1}`, 1 step | 1 call, params `{a: 1}` |
| step with merge | `with: {a: 1}`, step `with: {b: 2}` | params `{a: 1, b: 2}` |
| step with override | `with: {a: 1}`, step `with: {a: 2}` | params `{a: 2}` |

**Loop:**

| Case | Loop | Expected |
|---|---|---|
| time range | `date: -2d..-1d` | 2 calls, date=2026-03-18, date=2026-03-19 |
| numeric range | `n: 1..3` | 3 calls, n=1, n=2, n=3 |
| explicit list | `region: [eu, us]` | 2 calls |
| absolute dates | `date: 2026-03-01..2026-03-03` | 3 calls |
| multi-key | `date: -2d..-1d, region: [eu, us]` | 4 calls (cartesian) |

**Each:**

| Case | Each | Expected |
|---|---|---|
| basic | `[{a: 1}, {a: 2}]` | 2 calls |
| sparse keys | `[{a: 1, b: 2}, {a: 3}]` | call 1: a=1,b=2; call 2: a=3 (no b) |

**Combinations:**

| Case | Expected |
|---|---|
| each(2) x loop(3) | 6 calls |
| each(2) x loop(3) x 2 steps | 12 calls |
| step each: false | step without each, loop works |
| step loop: false | step without loop, each works |
| step each: false + loop: false | 1 call |
| step-level loop override | own loop, parent ignored |
| step-level each override | own each, parent ignored |

**Nested pipe step:**

| Case | Expected |
|---|---|
| step job is .pipe.yaml | 1 Call with job=path.pipe.yaml, no loop/each expansion |
| nested + parent with | parent with passed through as Call params |

**Templates:**

| Case | Template | Params | Expected |
|---|---|---|---|
| from with | `{base}/out` | base=/data | `/data/out` |
| from loop | `{date}.csv` | loop date=-1d | `2026-03-18.csv` |
| from each | `{label}/img` | each label=hd | `hd/img` |
| combined | `{base}/{label}/{date}` | all sources | full path |
| no template | `literal` | — | `literal` unchanged |

**Overrides:**

| Case | What is tested |
|---|---|
| override > step with | CLI key wins over step with |
| override > loop value | CLI key wins over loop |
| override > each value | CLI key wins over each |
| override > pipe with | CLI key wins over pipe with |

**Time expressions in values:**

| Case | What is tested |
|---|---|
| `with: {date: -1d}` | date resolves to a concrete date |
| `each: [{date: -1d}]` | date resolves |
| `1920x1080` | NOT a time expr, left unchanged |

**Input mode:**

| Case | Pipe.Input | Step.Input | Expected Call.Input |
|---|---|---|---|
| default | "" | "" | env |
| pipe-level | json | "" | json |
| step override | json | args | args |

**Policies:**

| Case | Pipe | Step | Expected StepPlan |
|---|---|---|---|
| retry inherit | retry: 3 | — | Retry: 3 |
| retry override | retry: 3 | retry: 5 | Retry: 5 |
| retry off | retry: 3 | RetryOff | Retry: 0 |
| timeout inherit | timeout: 10m | — | Timeout: 10m |
| timeout override | timeout: 10m | timeout: 30m | Timeout: 30m |
| allow_failure | — | allow_failure: true | AllowFailure: true |
| retry_delay default | retry: 3 | — | RetryDelay: 1s |
| retry_delay override | retry_delay: 5s | retry_delay: 10s | RetryDelay: 10s |

### validate/

Each rule is a separate test. Mock `fileExists`. Overrides are passed for template checks.

```go
func Validate(p *pipe.Pipe, cfg *config.Config, fileExists func(string) bool, overrides map[string]string) []error
```

| # | Rule | Invalid pipe | Expected error |
|---|---|---|---|
| 1 | Unknown YAML key | `steps` + `foo: bar` | unknown key "foo" |
| 2 | Job file not found | `job: scripts/missing.py` | file not found |
| 3 | Unknown extension | `job: scripts/run.xyz` | unsupported extension ".xyz" |
| 4 | Nested pipe not found | `job: missing.pipe.yaml` | file not found |
| 5 | loop/each on nested | `job: child.pipe.yaml` + `loop:` | loop not allowed on nested pipe |
| 6 | Bad input | `input: xml` | invalid input mode "xml" |
| 7 | Bad time expr | `loop: {date: -1x}` | cannot parse time expression |
| 8 | Unresolved template | `with: {out: {missing}/f}` | template {missing} unresolved |
| 8b | Template resolved via override | override `missing=val` | no error |
| 9 | Schedule cron+every | cron + every both set | specify cron or every, not both |
| 10 | Nested object in with | `with: {a: {b: c}}` | with values must be scalars |

**Additional:**

| Case | What is tested |
|---|---|
| valid pipe | `[]error` is empty |
| multiple errors | 3 issues → `[]error` of length 3 |
| nested pipe valid recursively | child pipe also passes validate |

### runner/

Integration tests with real scripts from `testdata/scripts/`.

**Script fixtures:**

| Script | Behavior |
|---|---|
| `exit0.sh` | `exit 0` |
| `exit1.sh` | `exit 1` |
| `echo_env.py` | prints required `os.environ` keys as JSON |
| `read_json.py` | `json.load(sys.stdin)`, prints it |
| `echo_args.sh` | `echo "$@"` |
| `slow.sh` | `sleep 10` |
| `flaky.sh` | accepts env `FAIL_COUNT`, fails N times, then exit 0 |
| `json_lines.py` | `print("text")`, `print(json.dumps({...}))`, `print("text")` |
| `stderr.sh` | `echo "error" >&2` |

**Tests:**

| Case | Setup | Expected |
|---|---|---|
| success | exit0.sh | nil error |
| failure | exit1.sh | RunError, ExitCode=1 |
| env mode | echo_env.py, params {src: /data} | stdout contains SRC=/data |
| json mode | read_json.py, input=json | stdout contains params |
| args mode | echo_args.sh, params {a: 1} | stdout contains --a 1 |
| timeout | slow.sh, timeout=1s | RunError (killed) |
| retry success | flaky.sh FAIL_COUNT=2, retry=3 | nil (succeeds on 3rd attempt) |
| retry exhausted | flaky.sh FAIL_COUNT=5, retry=3 | RunError |
| retry off | pipe retry=3, step RetryOff | 1 attempt, RunError |
| allow_failure | exit1.sh, allow_failure=true | pipe continues (RunPlan nil) |
| stdout text | json_lines.py | output.Text called for text lines |
| stdout json | json_lines.py + log fields | output.JSON called for JSON lines |
| stderr | stderr.sh | output.Stderr called |
| nested pipe | parent → child.pipe.yaml | child executed, parent with as overrides |

### output/

Snapshot tests. Call Writer methods with `color=false` (deterministic), compare against golden strings.

**Run-time methods:**

| Method | Call | Expected output |
|---|---|---|
| Start | `Start("scripts/resize.py", {"date": "2026-03-18"})` | `→ scripts/resize.py  date=2026-03-18\n` |
| Text | `Text("Resizing...")` | `  · Resizing...\n` |
| JSON | `JSON({"label": "hd", "file": "photo.jpg"})` | `  ▸ hd \| photo.jpg\n` |
| Stderr | `Stderr("Warning: low memory")` | `  ! Warning: low memory\n` |
| Retry | `Retry(1, 3, 1*time.Second)` | `  ↻ retry 1/3 (1s)\n` |
| Ok | `Ok("scripts/resize.py", 800*time.Millisecond)` | `✓ scripts/resize.py  0.8s\n` |
| Fail | `Fail("scripts/resize.py", 1, 2*time.Second)` | `✗ scripts/resize.py  exit=1  2.0s\n` |

**Check methods:**

| Method | Expected output |
|---|---|
| CheckPipe | `Pipe: images.pipe.yaml (Resize images)\n` |
| CheckStep | `  Step 1: scripts/resize.py × 4 each × 2 dates = 8 calls\n` |
| CheckCall | `    1. src=/data  date=2026-03-18  size=1920x1080\n` |
| CheckTotal | `  Total: 19 calls\n` |

**Color on/off:**

One test with `color=true` — verifies presence of ANSI escape codes.
One test with `color=false` — clean text without escapes.

**SetLog:**

| Case | log | JSON input | Expected |
|---|---|---|---|
| with fields | `["label", "file"]` | `{"label":"hd","file":"a.jpg","extra":"x"}` | `▸ hd \| a.jpg` (extra not shown) |
| empty log | `[]` | `{"label":"hd"}` | `· {"label":"hd"}` (as text) |
| missing field | `["label", "missing"]` | `{"label":"hd"}` | `▸ hd` (missing skipped) |

### scheduler/

**Unit tests:**

| Case | What is tested |
|---|---|
| parse cron | `"0 5 * * *"` parses without errors |
| parse every | `"10m"` parses as 10 minutes |
| cron + every | error |
| neither cron nor every | error |
| schedule with | With is passed as overrides |

**Integration:**

| Case | What is tested |
|---|---|
| ServeNow | all pipes from schedule run once, then exit |
| ServeNow fail fast | first pipe fails → second is not started |

### test/ — E2E

Full cycle. `TestMain` builds the binary via `go build`, tests run it via `exec.Command`.

```go
func TestMain(m *testing.M) {
    // go build -o binary
    os.Exit(m.Run())
    // cleanup
}

func run(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
    // exec.Command(binary, args...)
}
```

**Cases:**

| Case | Command | Expected |
|---|---|---|
| run success | `piperig run minimal.pipe.yaml` | exit 0, stdout contains ✓ |
| run failure | `piperig run fail.pipe.yaml` | exit 1, stdout contains ✗ |
| run with override | `piperig run pipe.yaml key=val` | exit 0, override applied |
| run directory | `piperig run testdata/pipes/` | alphabetical order, fail fast |
| check | `piperig check multi_step.pipe.yaml` | exit 0, stdout contains "Total: N calls" |
| check with override | `piperig check pipe.yaml key=val` | override reflected in plan |
| validation error | `piperig run invalid/unknown_key.pipe.yaml` | exit 2 |
| bad yaml | `piperig run broken.yaml` | exit 2 |
| serve --now | `piperig serve schedule.yaml --now` | exit 0, all pipes executed |
| version | `piperig version` | exit 0, stdout contains version |
| init | `piperig init` (in tmpdir) | .piperig.yaml created |
| new pipe | `piperig new pipe test` (in tmpdir) | test.pipe.yaml created |
| new schedule | `piperig new schedule test` (in tmpdir) | test.yaml created |
| no args | `piperig` | exit 1 (usage) |
| unknown command | `piperig foo` | exit 1 |
