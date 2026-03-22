package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/joarhal/piperig/internal/pipe"
)

func newTestWriter() (*Writer, *bytes.Buffer) {
	var buf bytes.Buffer
	return New(&buf, false), &buf
}

func TestStart(t *testing.T) {
	w, buf := newTestWriter()
	w.Start("scripts/resize.py", map[string]string{"date": "2026-03-18", "size": "1920x1080"})
	out := buf.String()
	if !strings.Contains(out, "→ scripts/resize.py  date=2026-03-18  size=1920x1080") {
		t.Errorf("got %q, want timestamp + → scripts/resize.py ...", out)
	}
	// Should have HH:MM:SS timestamp
	if len(out) < 8 || out[2] != ':' || out[5] != ':' {
		t.Errorf("expected HH:MM:SS timestamp prefix, got %q", out)
	}
}

func TestStartNoParams(t *testing.T) {
	w, buf := newTestWriter()
	w.Start("scripts/run.sh", nil)
	out := buf.String()
	if !strings.Contains(out, "→ scripts/run.sh") {
		t.Errorf("got %q, want timestamp + → scripts/run.sh", out)
	}
}

func TestText(t *testing.T) {
	w, buf := newTestWriter()
	w.Text("Resizing image...")
	out := buf.String()
	if !strings.Contains(out, "· Resizing image...") {
		t.Errorf("got %q, want indent + · Resizing image...", out)
	}
}

func TestJSON(t *testing.T) {
	w, buf := newTestWriter()
	w.SetLog([]string{"label", "file", "size"})
	w.JSON(map[string]string{"label": "fullhd", "file": "photo_001.jpg", "size": "1920x1080"})
	out := buf.String()
	if !strings.Contains(out, "▸ fullhd | photo_001.jpg | 1920x1080") {
		t.Errorf("got %q, want indent + ▸ fullhd | ...", out)
	}
}

func TestJSONMissingField(t *testing.T) {
	w, buf := newTestWriter()
	w.SetLog([]string{"label", "missing", "size"})
	w.JSON(map[string]string{"label": "fullhd", "size": "1920x1080"})
	out := buf.String()
	if !strings.Contains(out, "▸ fullhd | 1920x1080") {
		t.Errorf("got %q, want indent + ▸ fullhd | 1920x1080", out)
	}
}

func TestStderr(t *testing.T) {
	w, buf := newTestWriter()
	w.Stderr("Warning: EXIF data missing")
	out := buf.String()
	if !strings.Contains(out, "! Warning: EXIF data missing") {
		t.Errorf("got %q, want indent + ! Warning: EXIF data missing", out)
	}
}

func TestRetry(t *testing.T) {
	w, buf := newTestWriter()
	w.Retry(1, 3, time.Second)
	out := buf.String()
	if !strings.Contains(out, "↻ retry 1/3 (1s)") {
		t.Errorf("got %q, want indent + ↻ retry 1/3 (1s)", out)
	}
}

func TestOk(t *testing.T) {
	w, buf := newTestWriter()
	w.Ok("scripts/resize.py", 800*time.Millisecond)
	out := buf.String()
	if !strings.Contains(out, "✓ scripts/resize.py  0.8s") {
		t.Errorf("got %q, want timestamp + ✓ scripts/resize.py  0.8s", out)
	}
	if len(out) < 8 || out[2] != ':' || out[5] != ':' {
		t.Errorf("expected HH:MM:SS timestamp prefix, got %q", out)
	}
}

func TestFail(t *testing.T) {
	w, buf := newTestWriter()
	w.Fail("scripts/upload.sh", 1, 4100*time.Millisecond)
	out := buf.String()
	if !strings.Contains(out, "✗ scripts/upload.sh  exit=1  4.1s") {
		t.Errorf("got %q, want timestamp + ✗ scripts/upload.sh  exit=1  4.1s", out)
	}
	if len(out) < 8 || out[2] != ':' || out[5] != ':' {
		t.Errorf("expected HH:MM:SS timestamp prefix, got %q", out)
	}
}

func TestCheckPipe(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckPipe("images.pipe.yaml", "Resize images for the last 2 days")
	want := "Pipe: images.pipe.yaml (Resize images for the last 2 days)\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckPipeNoDesc(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckPipe("images.pipe.yaml", "")
	want := "Pipe: images.pipe.yaml\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckStep(t *testing.T) {
	w, buf := newTestWriter()
	step := pipe.StepPlan{
		Job:  "scripts/resize.py",
		Calls: make([]pipe.Call, 8),
		Dims: "4 each × 2 dates",
	}
	w.CheckStep(2, step)
	want := "  Step 2: scripts/resize.py × 4 each × 2 dates = 8 calls\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckStepNoDims(t *testing.T) {
	w, buf := newTestWriter()
	step := pipe.StepPlan{
		Job:   "scripts/upload.sh",
		Calls: make([]pipe.Call, 1),
	}
	w.CheckStep(4, step)
	want := "  Step 4: scripts/upload.sh = 1 calls\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckCall(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckCall(1, map[string]string{"src": "/data", "quality": "80"})
	want := "    1. quality=80  src=/data\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestCheckTotal(t *testing.T) {
	w, buf := newTestWriter()
	w.CheckTotal(19)
	want := "  Total: 19 calls\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestColorOutput(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf, true)
	w.Text("hello")
	out := buf.String()
	if out == "  · hello\n" {
		t.Error("expected ANSI codes in color mode")
	}
	if !bytes.Contains(buf.Bytes(), []byte("hello")) {
		t.Error("output should contain the text")
	}
}
