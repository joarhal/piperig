package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/joarhal/piperig/internal/pipe"
)

// ANSI escape codes.
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	white  = "\033[37m"
)

// Writer formats piperig output with optional ANSI colors.
type Writer struct {
	w     io.Writer
	color bool
	log   []string
}

// New creates a Writer. Set color=true for ANSI terminal output.
func New(w io.Writer, color bool) *Writer {
	return &Writer{w: w, color: color}
}

// SetLog configures which JSON fields to extract for formatted output.
func (w *Writer) SetLog(fields []string) {
	w.log = fields
}

// Log returns the configured log fields.
func (w *Writer) Log() []string {
	return w.log
}

// --- Run-time methods ---

// Start prints the step start line.
func (w *Writer) Start(job string, params map[string]string) {
	paramStr := formatParams(params)
	if paramStr != "" {
		paramStr = "  " + paramStr
	}
	if w.color {
		fmt.Fprintf(w.w, "%s%s→ %s%s%s\n", white, bold, job, paramStr, reset)
	} else {
		fmt.Fprintf(w.w, "→ %s%s\n", job, paramStr)
	}
}

// Text prints a plain stdout line.
func (w *Writer) Text(line string) {
	if w.color {
		fmt.Fprintf(w.w, "  %s· %s%s\n", dim, line, reset)
	} else {
		fmt.Fprintf(w.w, "  · %s\n", line)
	}
}

// JSON prints extracted JSON fields.
func (w *Writer) JSON(fields map[string]string) {
	var vals []string
	for _, k := range w.log {
		if v, ok := fields[k]; ok {
			vals = append(vals, v)
		}
	}
	line := strings.Join(vals, " | ")
	if w.color {
		fmt.Fprintf(w.w, "  %s▸ %s%s\n", cyan, line, reset)
	} else {
		fmt.Fprintf(w.w, "  ▸ %s\n", line)
	}
}

// Stderr prints a stderr line.
func (w *Writer) Stderr(line string) {
	if w.color {
		fmt.Fprintf(w.w, "  %s! %s%s\n", yellow, line, reset)
	} else {
		fmt.Fprintf(w.w, "  ! %s\n", line)
	}
}

// Retry prints a retry attempt marker.
func (w *Writer) Retry(attempt, max int, delay time.Duration) {
	if w.color {
		fmt.Fprintf(w.w, "  %s↻ retry %d/%d (%s)%s\n", yellow, attempt, max, delay, reset)
	} else {
		fmt.Fprintf(w.w, "  ↻ retry %d/%d (%s)\n", attempt, max, delay)
	}
}

// Ok prints a success line.
func (w *Writer) Ok(job string, dur time.Duration) {
	ds := formatDuration(dur)
	if w.color {
		fmt.Fprintf(w.w, "%s✓%s %s%s%s  %s%s%s\n", green, reset, bold, job, reset, dim, ds, reset)
	} else {
		fmt.Fprintf(w.w, "✓ %s  %s\n", job, ds)
	}
}

// Fail prints a failure line.
func (w *Writer) Fail(job string, exitCode int, dur time.Duration) {
	ds := formatDuration(dur)
	if w.color {
		fmt.Fprintf(w.w, "%s✗%s %s%s%s  %sexit=%d%s  %s%s%s\n",
			red, reset, bold, job, reset, red, exitCode, reset, dim, ds, reset)
	} else {
		fmt.Fprintf(w.w, "✗ %s  exit=%d  %s\n", job, exitCode, ds)
	}
}

// --- Check methods ---

// CheckPipe prints the pipe header for check output.
func (w *Writer) CheckPipe(name, description string) {
	if description != "" {
		fmt.Fprintf(w.w, "Pipe: %s (%s)\n", name, description)
	} else {
		fmt.Fprintf(w.w, "Pipe: %s\n", name)
	}
}

// CheckStep prints a step summary for check output.
func (w *Writer) CheckStep(n int, step pipe.StepPlan) {
	callCount := len(step.Calls)
	if step.Dims != "" {
		fmt.Fprintf(w.w, "  Step %d: %s × %s = %d calls\n", n, step.Job, step.Dims, callCount)
	} else {
		fmt.Fprintf(w.w, "  Step %d: %s = %d calls\n", n, step.Job, callCount)
	}
}

// CheckCall prints an individual call for check output.
func (w *Writer) CheckCall(n int, params map[string]string) {
	fmt.Fprintf(w.w, "    %d. %s\n", n, formatParams(params))
}

// CheckTotal prints the total call count for check output.
func (w *Writer) CheckTotal(total int) {
	fmt.Fprintf(w.w, "  Total: %d calls\n", total)
}

// --- Helpers ---

func formatParams(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + params[k]
	}
	return strings.Join(parts, "  ")
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}
