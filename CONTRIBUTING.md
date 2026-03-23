# Contributing to piperig

Thank you for your interest in contributing to piperig!

## Getting started

```bash
git clone https://github.com/joarhal/piperig.git
cd piperig
go test ./...
```

Requires Go 1.25+.

## Development workflow

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Run tests: `go test ./...`
5. Run vet: `go vet ./...`
6. Submit a pull request

## Code guidelines

- **English only.** All code, comments, commit messages, and documentation in English.
- **SPEC.md is the source of truth.** If code conflicts with the spec, fix the code.
- **Minimal dependencies.** stdlib + `gopkg.in/yaml.v3` + `bubbletea` + `robfig/cron`. Nothing else without discussion.
- **Test everything.** Every package has `_test.go`. Use table-driven tests for expansion logic.
- **Keep testdata/ in sync.** Tests load `.pipe.yaml` fixtures from `testdata/` directories.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description
```

- **Types:** `feat`, `fix`, `refactor`, `test`, `docs`, `chore`
- **Scope:** package name (`pipe`, `expand`, `runner`, etc.) or `cli` for `cmd/`
- **Mood:** imperative ("add parser", not "added parser")

Examples:

```
feat(expand): support numeric ranges in loop
fix(runner): handle timeout with process group kill
test(pipe): add fixtures for step-level log
docs: update README with scheduling examples
```

## Architecture

See `docs/ARCHITECTURE.md` for the package layout and data flow. Key points:

- `pipe.Call` is the central type — everything either produces or consumes Calls
- Steps are sequential, never parallel
- piperig only calls subprocesses — no Go plugins, no RPC

## Reporting issues

- **Bug reports:** include the `.pipe.yaml`, the command you ran, and the output
- **Feature requests:** describe the use case, not just the solution

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
