# piperig

Declarative pipeline runner. YAML in, exit code out.

piperig doesn't know what your jobs do. It knows how to run them: read YAML, expand loops, call a file, read the result. Single binary, zero dependencies.

## Principle

A pipe is a `.pipe.yaml` file. A job is an executable file. piperig connects one to the other.

```
*.pipe.yaml  →  piperig  →  scripts/resize.py  →  params (env / json / args)
                                                    stdout (job output)
                                                    exit code (success/failure)
```

piperig doesn't touch your business logic. It doesn't know about databases, programming languages, or clouds. It only knows:
- how to read YAML
- how to expand loops
- how to call a file
- how to determine the result by exit code

## Job

A job is a path to an executable file relative to the project root (the directory where piperig is called). All paths in a pipe are from the root. If piperig is called outside the project root — validation will catch it (files not found).

piperig determines how to run a file by its extension:

| Extension | Execution |
|---|---|
| `.py` | `python <path>` |
| `.sh` | `bash <path>` |
| `.js` | `node <path>` |
| `.ts` | `npx tsx <path>` |
| `.rb` | `ruby <path>` |
| none / +x | `./<path>` (direct exec) |

Unknown extension — validation error. For non-standard extensions — add a mapping in `.piperig.yaml`.

Contract: a file receives parameters and returns an exit code (0 = success, != 0 = error).

```yaml
steps:
  - job: scripts/resize.py           # → python scripts/resize.py
  - job: scripts/deploy.sh           # → bash scripts/deploy.sh
  - job: bin/optimize                 # → ./bin/optimize
```

### input — parameter passing method

`input` defines how piperig passes parameters to a job. Three modes:

| Mode | Delivery | Example |
|---|---|---|
| `env` (default) | environment variables (UPPERCASE keys) | `SRC=/data/photos QUALITY=80 python scripts/resize.py` |
| `json` | JSON on stdin | `echo '{"src":"/data/photos","quality":80}' \| python scripts/resize.py` |
| `args` | CLI arguments | `python scripts/resize.py --src /data/photos --quality 80` |

`input` can be specified at pipe level (for all steps) or at step level (overrides):

```yaml
input: json

steps:
  - job: scripts/resize.py                    # json (from pipe)
  - job: scripts/deploy.sh
    input: args                                # overridden for this step
```

piperig doesn't impose project structure. Where files live — the user decides.

### Nested pipes

If `job` points to a `.pipe.yaml` file — piperig executes it as a nested pipe:

```yaml
steps:
  - job: scripts/download.py
  - job: pipes/process_images.pipe.yaml    # .pipe.yaml → nested pipe
    with:
      quality: 90                               # overrides with inside process_images.pipe.yaml
  - job: scripts/cleanup.sh
```

Parent `with` overrides child `with` (caller wins). Child's own `loop`/`each` work as written.

`loop`/`each` on a step with `.pipe.yaml` works normally — the child pipe is invoked once per combination:

```yaml
steps:
  - job: pipes/kpi/dau.pipe.yaml
    each:
      - { project: ds }
      - { project: hn2 }
    loop:
      date: -7d..-1d
```

2 projects × 7 dates = 14 invocations of the child pipe. Each invocation receives the `project` and `date` values as parameter overrides.

## Pipe files

Pipe files have the `.pipe.yaml` extension. This allows piperig to find them among other YAML files in the project (docker-compose, CI, configs) without reading their contents.

```
project/
├── pipes/
│   ├── daily/
│   │   ├── images.pipe.yaml
│   │   └── reports.pipe.yaml
│   └── maintenance/
│       └── backup.pipe.yaml
├── scripts/
│   ├── resize.py
│   ├── watermark.py
│   └── upload.sh
└── schedule.yaml
```

## Pipe YAML

### Minimal example

```yaml
description: Resize today's photos

steps:
  - job: scripts/resize.py
    with:
      src: /data/photos/2026-03-17
      quality: 80
```

One step, one job, concrete parameters. piperig will call the file once.

`description` — optional field. Shown in interactive mode and in `piperig check`.

### hidden — exclude from picker

```yaml
description: Helper pipe for image processing
hidden: true

steps:
  - job: scripts/helper.py
```

`hidden: true` — the pipe won't appear in `piperig run` interactive picker. It can still be:
- Run directly: `piperig run pipes/helper.pipe.yaml`
- Used as a nested pipe: `job: pipes/helper.pipe.yaml`
- Included in a schedule

By default `hidden: false`. Use it for utility pipes that are called from other pipes but shouldn't clutter the picker.

### with — shared parameters

```yaml
with:
  src: /data/photos
  dest: /data/output
  quality: 80

steps:
  - job: scripts/resize.py
    with:
      width: 1920
  - job: scripts/watermark.py
    with:
      logo: /assets/logo.png
```

Top-level `with` is merged with each step's `with`. Step wins on conflict.

All values in `with` — scalars only (strings, numbers, booleans). Nested objects and lists are forbidden — validation error.

### Environment variable interpolation

`$VAR` and `${VAR}` in `with` values are expanded from the process environment before any other processing. This allows keeping secrets and host-specific values out of pipe files:

```yaml
with:
  db_host: $DB_HOST
  db_pass: ${DB_PASSWORD}
  bucket: s3://${S3_BUCKET}/output
```

Expansion happens at the same stage as time expression resolution — before template substitution. If a variable is not set, it is replaced with an empty string (same as shell behavior).

Environment variables work in all `with` sections (pipe-level, step-level) and in `each` items. They do **not** work in `loop` values — `loop` uses time ranges, numeric ranges, and explicit lists.

### Time expressions

piperig recognizes time expressions in values **everywhere** — in `with`, `loop`, `each`. If a value matches the format — it is resolved before passing to the job.

| Expression | Result | Description |
|---|---|---|
| `-1d` | `2026-03-18` | yesterday |
| `0d` | `2026-03-19` | today |
| `1d` | `2026-03-20` | tomorrow |
| `-2h` | `2026-03-19T09:00:00` | 2 hours ago (rounded to hour) |
| `-30m` | `2026-03-19T11:13:00` | 30 minutes ago (rounded to minute) |
| `-10s` | `2026-03-19T11:43:15` | 10 seconds ago (rounded to second) |
| `-1w` | `2026-03-10` | last week's Monday |
| `-4w..-1w` | 4 dates | last 4 Mondays |

Supported suffixes: `w` (weeks/Mondays), `d` (days), `h` (hours), `m` (minutes), `s` (seconds).

Sign: `-` past, `+` or no sign — future. `0d`, `0h` — current moment (rounded).

Rounding: values are always rounded down to the suffix unit. `-2h` at 11:43 → `09:00`, not `09:43`. This guarantees idempotency — repeated runs within the same hour produce the same result.

In `loop`, time expressions work as ranges: `-2d..-1d` expands to a list of dates, `-24h..-1h` — to a list of hours.

### loop — iteration over values

```yaml
loop:
  date: -7d..-1d

steps:
  - job: scripts/generate_report.py
    with:
      output: /reports/{date}.pdf
```

`loop` expands each key into a list of values. The step will execute 7 times — once per day. The loop value is injected into parameters.

loop works with any keys:

```yaml
loop:
  date: -2d..-1d
  region: [eu, us, asia]
```

This is a cartesian product: 2 days × 3 regions = 6 calls.

Loop values:
- Time range (`-2d..-1d`, `-24h..-1h`) → expands to a list (see "Time expressions")
- Absolute date range (`2026-03-01..2026-03-05`) → list of dates
- Numeric range (`1..5`) → list of numbers
- Explicit list (`[eu, us, asia]`) → iterate over elements

loop can be at pipe level (applies to all steps) or at step level (overrides):

```yaml
loop:
  date: -2d..-1d

steps:
  - job: scripts/generate_report.py         # date from top-level loop
  - job: scripts/generate_summary.py        # own loop, top-level ignored
    loop:
      date: -14d..-1d
```

### each — iteration over parameter sets

```yaml
each:
  - { size: 1920x1080, label: fullhd }
  - { size: 1280x720,  label: hd }
  - { size: 640x480,   label: sd }
  - { size: 128x128,   label: thumb }

steps:
  - job: scripts/resize.py
```

`each` — an array of objects. Each object is merged into parameters before the call. If a key is missing — it's not passed.

each can be at pipe level (applies to all steps) or at step level (overrides):

```yaml
each:
  - { size: 1920x1080, label: fullhd }
  - { size: 128x128,   label: thumb }

steps:
  - job: scripts/resize.py                     # each from top-level
  - job: scripts/export.py                     # own each, top-level ignored
    each:
      - { format: png }
      - { format: webp }
      - { format: avif }
```

### each + loop together

```yaml
with:
  src: /data/photos
  dest: /data/output
  quality: 80

loop:
  date: -2d..-1d

each:
  - { size: 1920x1080, label: fullhd }
  - { size: 1280x720,  label: hd }
  - { size: 640x480,   label: sd }
  - { size: 128x128,   label: thumb }

steps:
  - job: scripts/resize.py
```

Expansion order: `each × loop × steps`.

4 sizes × 2 days × 1 step = 8 calls. Each call receives the full parameter set: top-level with + each item + loop value + step with.

### Multi-step pipes with different jobs

```yaml
with:
  src: /data/photos
  dest: /data/output

each:
  - { size: 1920x1080, label: fullhd }
  - { size: 1280x720,  label: hd }
  - { size: 640x480,   label: sd }
  - { size: 128x128,   label: thumb }

loop:
  date: -2d..-1d

steps:
  - job: scripts/download.sh
    each: false
    with:
      bucket: s3://photos-raw

  - job: scripts/resize.py
    with:
      output: /data/output/{label}/{date}.jpg

  - job: scripts/watermark.py
    with:
      output: /data/output/{label}/{date}_wm.jpg
      logo: /assets/logo.png

  - job: scripts/upload.sh
    each: false
    loop: false
    with:
      bucket: s3://photos-processed
```

`each: false` and `loop: false` at step level — disable parent iterations for that step. Download is needed once per date (not per size), upload — once for the entire pipe.

### Templates

`{key}` in `with` values substitutes values from the full parameter pool: `with` + `each` + `loop` + step `with`. Substitution happens before passing to the job.

```yaml
with:
  base_dir: /data/output

each:
  - { label: fullhd, size: 1920x1080 }

loop:
  date: -2d..-1d

steps:
  - job: scripts/resize.py
    with:
      output: {base_dir}/{label}/{date}.jpg   # → /data/output/fullhd/2026-03-18.jpg
```

The job receives all parameters — both original (`base_dir`, `label`, `date`) and substituted (`output`). There are no extras — the job ignores what it doesn't need.

## Job execution and output handling

### Parameter passing

piperig passes parameters to the job depending on `input` (see "input — parameter passing method"):

- **env** (default): keys uppercased — `SRC=/data/photos DEST=/data/output DATE=2026-03-18 SIZE=1920x1080`
- **json**: `{"src": "/data/photos", "dest": "/data/output", "date": "2026-03-18", "size": "1920x1080"}` on stdin
- **args**: `--src /data/photos --dest /data/output --date 2026-03-18 --size 1920x1080`

### stdout

The job writes whatever it wants to stdout — plain text, logs, anything. piperig reads each line and tries to parse it as JSON. If a line is valid JSON, piperig uses it for formatted output (see `log`). If not JSON — passes through as-is.

```python
print("Resizing image...")                                  # text → passed through
print(json.dumps({"file": "photo.jpg", "size": "1920x1080"}))  # JSON → piperig formats
print("Done")                                               # text → passed through
```

JSON in stdout is optional. A job may not output JSON at all. piperig won't break.

### stderr

piperig passes through the job's stderr as-is, without changes.

### exit code

The sole result contract is the process exit code:
- **0** — success, next step
- **!= 0** — error

piperig does not interpret stdout/stderr content for decision-making. Exit code decides everything.

### retry — retry attempts

`retry` defines how many times piperig will retry a failed job. Specified at pipe level (for all steps) or at step level (overrides):

```yaml
retry: 3

steps:
  - job: scripts/resize.py                  # 3 attempts (from pipe)
  - job: scripts/upload.sh
    retry: 5                                 # 5 attempts (overridden)
  - job: scripts/notify.sh
    retry: false                             # no retries
```

By default retry is disabled — the job runs once.

Behavior: job failed → pause → retry → ... → if all attempts exhausted → fail fast, pipe stops.

`retry_delay` — pause between attempts. Default `1s`. Specified the same way — at pipe or step level:

```yaml
retry: 3
retry_delay: 5s
```

### timeout — execution time limit

`timeout` defines the maximum job execution time. If the job hasn't finished within the specified time — piperig kills the process and treats it as an error (subject to retry if configured).

```yaml
timeout: 10m

steps:
  - job: scripts/resize.py                  # 10m (from pipe)
  - job: scripts/upload.sh
    timeout: 30m                             # overridden
  - job: scripts/notify.sh
    timeout: 30s
```

By default timeout is disabled — the job can run indefinitely.

### allow_failure — continue on error

`allow_failure: true` on a step — if the job failed (after all retries), the pipe continues instead of fail fast. Useful for non-critical steps (notifications, logging).

```yaml
steps:
  - job: scripts/resize.py
  - job: scripts/notify.sh
    allow_failure: true                      # failed — no big deal, move on
```

By default `allow_failure: false` — an error stops the pipe.

## CLI

```
piperig run <file.pipe.yaml>         run a pipe
piperig run <directory/>             run all .pipe.yaml in directory
piperig run                          TUI: choose pipe interactively
piperig serve <schedule.yaml>        run scheduler (cron)
piperig check <file.pipe.yaml>       dry-check: show what will be called
piperig check <directory/>           dry-check: all .pipe.yaml in directory
piperig list [directory]             list all .pipe.yaml files
piperig init                         create .piperig.yaml with defaults
piperig new pipe <name>              create .pipe.yaml from template
piperig new schedule <name>          create schedule.yaml from template
piperig version                      version
```

### piperig run

```
piperig run pipes/daily/images.pipe.yaml
piperig run pipes/daily/images.pipe.yaml quality=90 dest=/tmp/output
piperig run pipes/daily/                           # all .pipe.yaml in directory, alphabetical order
```

`key=value` after the file — overrides parameters. No `--`, no conflicts with piperig flags.

Parameter priority (weakest to strongest):

1. top-level `with`
2. `each` item
3. `loop` value
4. step `with`
5. **CLI `key=value`** — wins everything

#### --no-color

`--no-color` disables ANSI colors and timestamps in output. Useful for CI, logging to files, or piping to other tools.

```
piperig run pipes/daily/ --no-color
piperig check pipes/daily/ --no-color
```

By default, colors are auto-detected: enabled when stdout is a terminal, disabled when piped. `--no-color` forces colors off regardless of terminal detection.

Other piperig flags (with `--`): reserved for future options.

### piperig run (interactive)

```
$ piperig run

  pipes/daily/images.pipe.yaml — Resize images for the last 2 days
  pipes/daily/reports.pipe.yaml — Weekly sales report
  pipes/daily/
  pipes/maintenance/backup.pipe.yaml — Database backup
  pipes/maintenance/
  pipes/

  ↑/↓ move  •  type to filter  •  Enter run  •  q quit
```

Without arguments piperig searches for all `.pipe.yaml` recursively from the current directory and builds a flat list of all runnable units:
- **Files** — each `.pipe.yaml` with `description` (optional field)
- **Directories** — each directory containing `.pipe.yaml` (recursively)

Fuzzy search by full path:

```
$ piperig run

  > daily im

  pipes/daily/images.pipe.yaml — Resize images for the last 2 days

  ↑/↓ move  •  ←/→ mode  •  type to filter  •  Enter run  •  q quit
```

In the top-right corner — mode toggle with `←/→` arrows:
- **run** — run pipe
- **check** — show call plan (dry-check)

Enter → execute in selected mode → log to terminal → exit. No return to menu.

Example output: `bash docs/tui_example.sh`

When running a directory, pipes execute in alphabetical order. If one pipe fails — stop, the rest don't run (fail fast). For explicit ordering — number the files (`01_download.pipe.yaml`, `02_process.pipe.yaml`) or use a schedule.

### piperig check

```
$ piperig check pipes/daily/images.pipe.yaml

Pipe: images.pipe.yaml (Resize images for the last 2 days)

  Step 1: scripts/download.sh × 2 dates = 2 calls

    1. src=/data/photos  dest=/data/output  date=2026-03-18  bucket=s3://photos-raw
    2. src=/data/photos  dest=/data/output  date=2026-03-19  bucket=s3://photos-raw

  Step 2: scripts/resize.py × 4 each × 2 dates = 8 calls

    1. src=/data/photos  dest=/data/output  date=2026-03-18  size=1920x1080  label=fullhd  output=/data/output/fullhd/2026-03-18.jpg
    2. src=/data/photos  dest=/data/output  date=2026-03-18  size=1280x720   label=hd      output=/data/output/hd/2026-03-18.jpg
    3. src=/data/photos  dest=/data/output  date=2026-03-18  size=640x480    label=sd      output=/data/output/sd/2026-03-18.jpg
    4. src=/data/photos  dest=/data/output  date=2026-03-18  size=128x128    label=thumb   output=/data/output/thumb/2026-03-18.jpg
    5. src=/data/photos  dest=/data/output  date=2026-03-19  size=1920x1080  label=fullhd  output=/data/output/fullhd/2026-03-19.jpg
    6. src=/data/photos  dest=/data/output  date=2026-03-19  size=1280x720   label=hd      output=/data/output/hd/2026-03-19.jpg
    7. src=/data/photos  dest=/data/output  date=2026-03-19  size=640x480    label=sd      output=/data/output/sd/2026-03-19.jpg
    8. src=/data/photos  dest=/data/output  date=2026-03-19  size=128x128    label=thumb   output=/data/output/thumb/2026-03-19.jpg

  Step 3: scripts/watermark.py × 4 each × 2 dates = 8 calls

    1. src=/data/photos  dest=/data/output  date=2026-03-18  size=1920x1080  label=fullhd  output=/data/output/fullhd/2026-03-18_wm.jpg  logo=/assets/logo.png
    2. src=/data/photos  dest=/data/output  date=2026-03-18  size=1280x720   label=hd      output=/data/output/hd/2026-03-18_wm.jpg      logo=/assets/logo.png
    3. src=/data/photos  dest=/data/output  date=2026-03-18  size=640x480    label=sd      output=/data/output/sd/2026-03-18_wm.jpg      logo=/assets/logo.png
    4. src=/data/photos  dest=/data/output  date=2026-03-18  size=128x128    label=thumb   output=/data/output/thumb/2026-03-18_wm.jpg   logo=/assets/logo.png
    5. src=/data/photos  dest=/data/output  date=2026-03-19  size=1920x1080  label=fullhd  output=/data/output/fullhd/2026-03-19_wm.jpg  logo=/assets/logo.png
    6. src=/data/photos  dest=/data/output  date=2026-03-19  size=1280x720   label=hd      output=/data/output/hd/2026-03-19_wm.jpg      logo=/assets/logo.png
    7. src=/data/photos  dest=/data/output  date=2026-03-19  size=640x480    label=sd      output=/data/output/sd/2026-03-19_wm.jpg      logo=/assets/logo.png
    8. src=/data/photos  dest=/data/output  date=2026-03-19  size=128x128    label=thumb   output=/data/output/thumb/2026-03-19_wm.jpg   logo=/assets/logo.png

  Step 4: scripts/upload.sh × 1 call

    1. src=/data/photos  dest=/data/output  bucket=s3://photos-processed

  Total: 19 calls
```

Does not call jobs. Shows the full list of calls with all parameters for each.

`check` accepts `key=value` just like `run` — for preview with overrides:

```
piperig check pipes/daily/images.pipe.yaml date=-1d quality=90
```

### piperig list

```
$ piperig list

pipes/daily/images.pipe.yaml — Resize images for the last 2 days
pipes/daily/reports.pipe.yaml — Weekly sales report
pipes/maintenance/backup.pipe.yaml — Database backup
```

Lists all `.pipe.yaml` files found recursively from the current directory (or from the given directory). Each line shows the path and description (if present). Pipes with `hidden: true` are excluded.

```
piperig list                         # search from cwd
piperig list pipes/daily/            # search from specific directory
```

No TUI, no interaction — plain text, one line per pipe. Useful for scripting, `grep`, and quick overview.

### piperig new

```
piperig new pipe pipes/daily/images      → pipes/daily/images.pipe.yaml
piperig new schedule schedule                     → schedule.yaml
```

Extension is added automatically (`.pipe.yaml` for pipe, `.yaml` for schedule). Parent directories are created automatically if they don't exist. If the file already exists — error, no overwrite.

Pipe template:

```yaml
description: ""

steps:
  - job: scripts/example.py
```

Schedule template:

```yaml
- name: daily
  cron: "0 5 * * *"
  run:
    - pipes/daily/
```

### piperig serve

```
piperig serve schedule.yaml          # cron-loop, runs permanently
piperig serve schedule.yaml --now    # everything from schedule once and exit
```

Scheduler runs pipes on a cron schedule.

## Schedule YAML

Schedule is **when** to run. **What** to run — is in the pipe YAML.

```yaml
- name: daily-images
  cron: "0 5 * * *"
  run:
    - pipes/daily/
  with:
    quality: 80
    dest: /data/output

- name: healthcheck
  every: 10m
  run:
    - pipes/maintenance/healthcheck.pipe.yaml
```

Each entry — `cron` (cron expression) or `every` (interval). One or the other.

`run` — list of `.pipe.yaml` files or directories (same as CLI).

`with` — overrides pipe parameters (like CLI `key=value`). Wins over everything in pipe YAML. Parameters only — `loop` and `each` are not supported in schedule, that's pipe logic.

Entries run independently. Within `run` — sequentially. If a pipe fails — stop, the remaining in `run` don't execute (fail fast).

## Output

piperig formats output with a pipe header, icons, timestamps, and indentation. The header shows the pipe filename and description. Step start (`→`) and finish (`✓`/`✗`) lines include an `HH:MM:SS` timestamp. Intermediate lines (stdout, stderr, retry) are indented without a timestamp. A summary line at the end shows total calls and duration.

```
images.pipe.yaml — Resize images for the last 2 days

09:15:32 → scripts/resize.py  date=2026-03-18  size=1920x1080  label=fullhd
           · Resizing image...
           ▸ fullhd | photo_001.jpg | 1920x1080
           · Applying sharpening filter...
           ▸ fullhd | photo_001.jpg | sharpened
           ! Warning: EXIF data missing
           · Done
09:15:33 ✓ scripts/resize.py  0.8s

09:15:33 → scripts/resize.py  date=2026-03-18  size=1280x720  label=hd
           · Resizing image...
           ! Connection timeout
           ↻ retry 1/3 (1s)
           · Resizing image...
           ▸ hd | photo_001.jpg | 1280x720
           · Done
09:15:34 ✓ scripts/resize.py  1.4s

09:15:34 → scripts/upload.sh  bucket=s3://photos-processed
           · Uploading 8 files...
           ! S3 throttling
           ↻ retry 1/3 (1s)
           ! S3 throttling
           ↻ retry 2/3 (1s)
           ! S3 throttling
           ↻ retry 3/3 (1s)
           ! S3 throttling
09:15:38 ✗ scripts/upload.sh  exit=1  4.1s

✗ 19 calls  6.2s
```

Icons and colors:
- pipe header — **bold** name, **gray** description
- `HH:MM:SS` timestamp — **gray** (dimmed), on start/finish lines only
- `→` step start (with call parameters) — **white/bold**
- `·` stdout text (plain print) — **gray** (dimmed)
- `▸` stdout JSON (formatted via `log`) — **cyan**
- `!` stderr — **yellow**
- `↻` retry — **yellow**
- `✓` step/pipe finish (success) — **green**
- `✗` step/pipe finish (failure) — **red**

Colors and timestamps are automatically disabled when stdout is not a terminal (piped to file, redirected). `--no-color` forces colors off even when running in a terminal.

Example output: `bash docs/log_example.sh`

### log — field selection for JSON formatting

`log` — output setting (not passed to the job). A list of fields from JSON lines in stdout. When a job outputs a JSON line, piperig extracts the specified fields and formats them with `|`:

```yaml
log:
  - label
  - file
  - size

steps:
  - job: scripts/resize.py
```

`log` can be specified at pipe level (for all steps) or at step level (overrides):

```yaml
log:
  - label
  - file

steps:
  - job: scripts/resize.py                    # log from pipe
  - job: scripts/upload.py
    log: [file, status, url]                   # own log fields
  - job: scripts/notify.sh                     # log from pipe
```

Step-level `log` completely replaces pipe-level `log` for that step.

Without `log` — JSON lines are displayed as plain text (`·`).

## .piperig.yaml — project config

Optional file at the project root. Two sections: `interpreters` and `env`.

### interpreters — custom script runners

```yaml
interpreters:
  .py: python3.11
  .js: node18
  .php: php
  .lua: lua
```

Without the file — defaults (`python`, `bash`, `node`, `npx tsx`, `ruby`). Non-standard extensions (`.php`, `.lua`, `.r`) are added here.

### env — process environment variables

```yaml
env:
  PYTHONPATH: .
  NODE_ENV: production
  API_BASE: https://api.example.com
```

Variables from `env` are added to the subprocess environment for every job. If a variable already exists in the system environment, the config value wins (override).

`env` is for the process environment — it does not mix with `with` parameters. `with` controls what the job receives as input (env vars with uppercase keys, JSON, or args). `env` controls the runtime environment of the process itself.

### piperig init

```
$ piperig init

Created .piperig.yaml:
  .py → python
  .sh → bash
  .js → node
  .ts → npx tsx
  .rb → ruby
```

Creates `.piperig.yaml` with defaults. If you don't need it — don't run it.

## Validation

piperig validates everything **before** execution begins. Not a single job call until validation passes.

When it runs:
- `piperig check` — validation + plan
- `piperig run` — validation → if ok → execution
- `piperig serve` — validates all pipes at startup → if ok → cron-loop

What is checked:
1. **YAML structure** — unknown keys → error
2. **Job files** — exist on disk
3. **Job extension** — supported (built-in or from `.piperig.yaml`), otherwise error
4. **Nested .pipe.yaml** — exist, valid recursively
5. **loop/each on .pipe.yaml step** — supported, produces multiple invocations
6. **input** — only `env`, `json`, `args`
7. **Time expressions** — parse correctly (`-1d`, `-2h..-1h`)
8. **Templates** — `{label}` resolves from available parameters (`each`, `with`, `loop`, overrides)
9. **Schedule** — `cron` or `every` (not both), expressions are valid

10. **`with` values** — scalars only (strings, numbers, booleans), nested objects and lists are forbidden

Any validation error — immediate exit with problem description. Strict policy, no warnings.

## piperig exit codes

- **0** — all pipes and steps completed successfully
- **1** — pipe failed (job returned != 0, timeout, retries exhausted)
- **2** — validation error (invalid YAML, file not found, etc.)

## What piperig does NOT do

- Does not manage dependencies between pipes
- Does not store run history
- Does not have a web UI
- Does not know about databases, clouds, or programming languages
- Does not run jobs in parallel within a pipe (steps are sequential)
