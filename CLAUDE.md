# piperig

Declarative pipeline runner. Go. Single binary.

## Read order
1. `docs/SPEC.md` — format, API, examples. **Start here.**
2. `docs/ARCHITECTURE.md` — packages, data flow, dependency graph.
3. `docs/TESTING.md` — test strategy, cases, fixtures.
4. This file — development rules.

## Architecture

```
embed.go                          go:embed README.md for `piperig llm`
cmd/piperig/main.go               CLI entrypoint
internal/
  pipe/                            types (Pipe, Step, Call, Plan) + YAML parsing
  timeexpr/                        time expressions (-1d, -2h, -1w, ranges)
  expand/                          loop/each/template expansion → Plan
  validate/                        10 validation rules, before execution
  runner/                          subprocess execution, stdout JSON parsing
  output/                          formatted terminal output (icons, colors)
  scheduler/                       cron + every, schedule YAML
  picker/                          TUI interactive mode (bubbletea)
  config/                          .piperig.yaml (interpreter overrides)
```

## Rules

1. **English only.** All code, comments, commit messages, docs, and issues in English. Public project.
2. **SPEC.md is the source of truth.** Implementation follows the spec, not the other way around. If something in code conflicts with the spec — fix the code.
3. **Minimal external dependencies.** stdlib + `gopkg.in/yaml.v3` + `bubbletea` for TUI + cron library for scheduler. Nothing else.
4. **Test with `go test`.** Every package has `_test.go`. Expansion is pure logic — test it thoroughly with table-driven tests.
5. **testdata/ contains example pipes.** Tests load from there. Keep examples in sync with SPEC.md.
6. **Process boundary.** piperig calls a subprocess, passes params via env/json/args. Exit code determines success (0) or failure (!=0). No Go plugins, no shared memory, no RPC.
7. **Steps are sequential.** No parallelism within a pipe. One step finishes, next begins.
8. **Fail fast.** Non-zero exit code stops the pipe (unless `allow_failure: true` on step).
9. **Pipe files use `.pipe.yaml` extension.** This is how piperig identifies pipes among other YAML files.
10. **Job is a file path.** Resolved relative to cwd. Execution method determined by extension (`.py` → python, `.sh` → bash, no extension → direct exec).
11. **`with` not `props`.** Parameters passed to jobs are called `with` in pipe YAML.
12. **Call is the central type.** Everything either produces Calls (expand) or consumes them (runner, check).

## Commits

1. **Atomic.** One logical change per commit. Don't mix refactoring with new features.
2. **Conventional Commits format.** `type(scope): description` — types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`. Scope is the package name (`pipe`, `timeexpr`, `expand`, etc.) or `cli` for cmd/.
3. **Imperative mood.** "add parser" not "added parser".
4. **Body when needed.** If the change isn't obvious from the title, add a blank line and a short body explaining _why_.
5. **Co-author.** Every commit includes `Co-Authored-By` for all contributors.
6. **English only.** Commit messages in English.

## Screenshots

Terminal screenshots for README and docs are generated from HTML templates.

```
assets/
  terminals/
    terminal.css        shared styles (colors, fonts, layout)
    banner.html         each screenshot is a separate HTML file
  generate.sh           converts all HTML → PNG (uses Puppeteer)
```

- Each HTML file links `terminal.css` and contains only the terminal content.
- CSS classes match piperig output: `.cmd`, `.job`, `.timestamp`, `.dim`, `.ok`, `.fail`, `.warn`, `.retry`, `.summary-ok`.
- PNGs are generated with transparent background and saved to `assets/`.
- Run `make screenshots` to regenerate all PNGs.
- To add a new screenshot: create `assets/terminals/<name>.html`, run `make screenshots`.

# ExecPlans

When writing complex features or significant refactors, use an ExecPlan (as described in docs/plans/PLANS.md) from design to implementation.
