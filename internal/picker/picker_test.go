package picker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectItems(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tasks/a.pipe.yaml", "description: Alpha\nsteps:\n  - job: echo\n")
	writeFile(t, dir, "tasks/b.pipe.yaml", "steps:\n  - job: echo\n")
	writeFile(t, dir, "pipes/c.pipe.yaml", "description: Charlie\nsteps:\n  - job: echo\n")

	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	items, err := collectItems()
	if err != nil {
		t.Fatal(err)
	}

	// Should find 3 files + 2 dirs (pipes/, tasks/)
	var files, dirs []string
	for _, it := range items {
		if it.dir {
			dirs = append(dirs, it.path)
		} else {
			files = append(files, it.path)
		}
	}

	if len(files) != 3 {
		t.Fatalf("files: got %d, want 3: %v", len(files), files)
	}
	if len(dirs) != 2 {
		t.Fatalf("dirs: got %d, want 2: %v", len(dirs), dirs)
	}

	// Files should be sorted
	for i := 1; i < len(files); i++ {
		if files[i] < files[i-1] {
			t.Errorf("files not sorted: %v", files)
		}
	}

	// Check descriptions loaded
	for _, it := range items {
		if it.path == filepath.Join("tasks", "a.pipe.yaml") && it.desc != "Alpha" {
			t.Errorf("desc for a.pipe.yaml: got %q, want %q", it.desc, "Alpha")
		}
		if it.path == filepath.Join("tasks", "b.pipe.yaml") && it.desc != "" {
			t.Errorf("desc for b.pipe.yaml: got %q, want empty", it.desc)
		}
		if it.path == filepath.Join("pipes", "c.pipe.yaml") && it.desc != "Charlie" {
			t.Errorf("desc for c.pipe.yaml: got %q, want %q", it.desc, "Charlie")
		}
	}

	// Paths should be relative
	for _, it := range items {
		if filepath.IsAbs(it.path) {
			t.Errorf("expected relative path, got %q", it.path)
		}
	}
}

func TestCollectItemsEmpty(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	items, err := collectItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestApplyFilter(t *testing.T) {
	m := model{
		all: []item{
			{path: "tasks/deploy.pipe.yaml"},
			{path: "tasks/build.pipe.yaml"},
			{path: "pipes/test.pipe.yaml"},
		},
		filtered: nil,
		cursor:   0,
	}
	m.filtered = m.all

	// Filter by "build"
	m.filter = "build"
	m.applyFilter()
	if len(m.filtered) != 1 {
		t.Fatalf("filtered: got %d, want 1", len(m.filtered))
	}
	if m.filtered[0].path != "tasks/build.pipe.yaml" {
		t.Errorf("filtered[0] = %q", m.filtered[0].path)
	}

	// Filter by "pipe" — matches all
	m.filter = "pipe"
	m.applyFilter()
	if len(m.filtered) != 3 {
		t.Fatalf("filtered: got %d, want 3", len(m.filtered))
	}

	// Filter with no match
	m.filter = "zzz"
	m.applyFilter()
	if len(m.filtered) != 0 {
		t.Fatalf("filtered: got %d, want 0", len(m.filtered))
	}
	if m.cursor != 0 {
		t.Errorf("cursor should be 0 on empty, got %d", m.cursor)
	}

	// Clear filter
	m.filter = ""
	m.applyFilter()
	if len(m.filtered) != 3 {
		t.Fatalf("filtered: got %d, want 3", len(m.filtered))
	}
}

func TestApplyFilterCaseInsensitive(t *testing.T) {
	m := model{
		all: []item{
			{path: "Deploy.pipe.yaml"},
		},
	}
	m.filtered = m.all

	m.filter = "deploy"
	m.applyFilter()
	if len(m.filtered) != 1 {
		t.Fatal("case-insensitive filter should match")
	}
}

func TestApplyFilterCursorBounds(t *testing.T) {
	m := &model{
		all: []item{
			{path: "a.pipe.yaml"},
			{path: "b.pipe.yaml"},
			{path: "c.pipe.yaml"},
		},
		cursor: 2,
	}
	m.filtered = m.all

	// Filter reduces list to 1 item, cursor should clamp from 2 to 0
	m.filter = "a.pipe"
	m.applyFilter()
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}

func TestViewContainsItems(t *testing.T) {
	m := model{
		all: []item{
			{path: "tasks/deploy.pipe.yaml", desc: "Deploy to prod"},
			{path: "tasks/build.pipe.yaml"},
			{path: "tasks/", dir: true},
		},
		filtered: []item{
			{path: "tasks/deploy.pipe.yaml", desc: "Deploy to prod"},
			{path: "tasks/build.pipe.yaml"},
			{path: "tasks/", dir: true},
		},
		cursor: 0,
		mode:   0,
	}

	v := m.View()

	// Header
	if !strings.Contains(v, "piperig") {
		t.Error("view should contain piperig header")
	}

	// Items
	if !strings.Contains(v, "tasks/deploy.pipe.yaml") {
		t.Error("view should contain deploy pipe")
	}
	if !strings.Contains(v, "Deploy to prod") {
		t.Error("view should contain description")
	}
	if !strings.Contains(v, "tasks/build.pipe.yaml") {
		t.Error("view should contain build pipe")
	}

	// Cursor on first item
	if !strings.Contains(v, "▸") {
		t.Error("view should contain cursor marker")
	}

	// Footer
	if !strings.Contains(v, "Enter run") {
		t.Error("view should show 'Enter run' in run mode")
	}
}

func TestViewCheckMode(t *testing.T) {
	m := model{
		all:      []item{{path: "a.pipe.yaml"}},
		filtered: []item{{path: "a.pipe.yaml"}},
		mode:     1,
	}

	v := m.View()
	if !strings.Contains(v, "Enter check") {
		t.Error("view should show 'Enter check' in check mode")
	}
}

func TestViewWithFilter(t *testing.T) {
	m := model{
		all:      []item{{path: "a.pipe.yaml"}},
		filtered: []item{{path: "a.pipe.yaml"}},
		filter:   "test",
	}

	v := m.View()
	if !strings.Contains(v, "> test") {
		t.Error("view should show filter input")
	}
}

func TestViewNoMatches(t *testing.T) {
	m := model{
		all:      []item{{path: "a.pipe.yaml"}},
		filtered: []item{},
	}

	v := m.View()
	if !strings.Contains(v, "(no matches)") {
		t.Error("view should show (no matches) when filtered is empty")
	}
}

func TestUpdateNavigation(t *testing.T) {
	m := model{
		all: []item{
			{path: "a.pipe.yaml"},
			{path: "b.pipe.yaml"},
			{path: "c.pipe.yaml"},
		},
		filtered: []item{
			{path: "a.pipe.yaml"},
			{path: "b.pipe.yaml"},
			{path: "c.pipe.yaml"},
		},
		cursor: 0,
	}

	// Down
	result, _ := m.Update(keyMsg(tea.KeyDown))
	rm := result.(model)
	if rm.cursor != 1 {
		t.Errorf("down: cursor = %d, want 1", rm.cursor)
	}

	// Up
	result, _ = rm.Update(keyMsg(tea.KeyUp))
	rm = result.(model)
	if rm.cursor != 0 {
		t.Errorf("up: cursor = %d, want 0", rm.cursor)
	}

	// Up at 0 stays at 0
	result, _ = rm.Update(keyMsg(tea.KeyUp))
	rm = result.(model)
	if rm.cursor != 0 {
		t.Errorf("up at 0: cursor = %d, want 0", rm.cursor)
	}

	// Down to end
	rm.cursor = 2
	result, _ = rm.Update(keyMsg(tea.KeyDown))
	rm = result.(model)
	if rm.cursor != 2 {
		t.Errorf("down at end: cursor = %d, want 2", rm.cursor)
	}
}

func TestUpdateModeToggle(t *testing.T) {
	m := model{mode: 0}

	result, _ := m.Update(keyMsg(tea.KeyRight))
	rm := result.(model)
	if rm.mode != 1 {
		t.Errorf("right: mode = %d, want 1", rm.mode)
	}

	result, _ = rm.Update(keyMsg(tea.KeyLeft))
	rm = result.(model)
	if rm.mode != 0 {
		t.Errorf("left: mode = %d, want 0", rm.mode)
	}
}

func TestUpdateEnter(t *testing.T) {
	m := model{
		filtered: []item{{path: "a.pipe.yaml"}},
		cursor:   0,
	}

	result, cmd := m.Update(keyMsg(tea.KeyEnter))
	rm := result.(model)
	if !rm.chosen {
		t.Error("enter should set chosen")
	}
	if cmd == nil {
		t.Error("enter should return quit cmd")
	}
}

func TestUpdateEnterNoItems(t *testing.T) {
	m := model{filtered: []item{}}

	result, cmd := m.Update(keyMsg(tea.KeyEnter))
	rm := result.(model)
	if rm.chosen {
		t.Error("enter with no items should not set chosen")
	}
	if cmd != nil {
		t.Error("enter with no items should not quit")
	}
}

func TestUpdateQuit(t *testing.T) {
	m := model{}

	// Ctrl+C
	result, cmd := m.Update(keyMsg(tea.KeyCtrlC))
	rm := result.(model)
	if !rm.quit {
		t.Error("ctrl+c should set quit")
	}
	if cmd == nil {
		t.Error("ctrl+c should return cmd")
	}

	// Esc
	m = model{}
	result, cmd = m.Update(keyMsg(tea.KeyEsc))
	rm = result.(model)
	if !rm.quit {
		t.Error("esc should set quit")
	}
}

func keyMsg(keyType tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: keyType}
}
