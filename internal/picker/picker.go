package picker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joarhal/piperig/internal/pipe"
)

// Result is the user's selection from the picker.
type Result struct {
	Target string // path to pipe file or directory
	Mode   string // "run" or "check"
}

type item struct {
	path string
	desc string
	dir  bool
}

type model struct {
	all      []item
	filtered []item
	cursor   int
	filter   string
	mode     int // 0=run, 1=check
	chosen   bool
	quit     bool
}

var modes = [2]string{"run", "check"}

// Pick scans for .pipe.yaml files and shows an interactive TUI.
// Returns the selected target and mode, or error.
func Pick() (*Result, error) {
	items, err := collectItems()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no .pipe.yaml files found")
	}

	m := model{all: items, filtered: items}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := final.(model)
	if fm.quit || !fm.chosen {
		return nil, fmt.Errorf("cancelled")
	}
	selected := fm.filtered[fm.cursor]
	return &Result{
		Target: selected.path,
		Mode:   modes[fm.mode],
	}, nil
}

func collectItems() ([]item, error) {
	paths, err := pipe.Scan(".")
	if err != nil {
		return nil, err
	}

	var files []item
	dirs := make(map[string]bool)

	cwd, _ := os.Getwd()
	for _, p := range paths {
		rel, err := filepath.Rel(cwd, p)
		if err != nil {
			rel = p
		}

		desc := ""
		hidden := false
		if loaded, err := pipe.Load(p); err == nil {
			desc = loaded.Description
			hidden = loaded.Hidden
		}

		if hidden {
			continue
		}

		files = append(files, item{path: rel, desc: desc})

		// Collect parent directories
		dir := filepath.Dir(rel)
		for dir != "." && dir != "" {
			dirs[dir+"/"] = true
			dir = filepath.Dir(dir)
		}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })

	var dirItems []item
	for d := range dirs {
		dirItems = append(dirItems, item{path: d, dir: true})
	}
	sort.Slice(dirItems, func(i, j int) bool { return dirItems[i].path < dirItems[j].path })

	return append(files, dirItems...), nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quit = true
			return m, tea.Quit
		case tea.KeyEsc:
			m.quit = true
			return m, tea.Quit
		case tea.KeyEnter:
			if len(m.filtered) > 0 {
				m.chosen = true
				return m, tea.Quit
			}
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case tea.KeyLeft:
			m.mode = 0
		case tea.KeyRight:
			m.mode = 1
		case tea.KeyBackspace:
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}
		case tea.KeyRunes:
			ch := string(msg.Runes)
			if ch == "q" && m.filter == "" {
				m.quit = true
				return m, tea.Quit
			}
			if ch == "k" && m.filter == "" {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}
			if ch == "j" && m.filter == "" {
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
				return m, nil
			}
			m.filter += ch
			m.applyFilter()
		}
	}
	return m, nil
}

func (m *model) applyFilter() {
	if m.filter == "" {
		m.filtered = m.all
	} else {
		lower := strings.ToLower(m.filter)
		var result []item
		for _, it := range m.all {
			if strings.Contains(strings.ToLower(it.path), lower) {
				result = append(result, it)
			}
		}
		m.filtered = result
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m model) View() string {
	var b strings.Builder

	// Header
	b.WriteString("\033[36m  ╭──────────╮\033[0m\n")
	b.WriteString("\033[36m  │\033[0m \033[1;36mpiperig\033[0m  \033[36m│\033[0m\n")
	b.WriteString("\033[36m  ╰──────────╯\033[0m\n\n")

	// Mode toggle
	var modeStr string
	if m.mode == 0 {
		modeStr = "  \033[7;1m run \033[0m  check   ←/→"
	} else {
		modeStr = "  run  \033[7;1m check \033[0m  ←/→"
	}
	b.WriteString(modeStr + "\n\n")

	// Filter
	if m.filter != "" {
		b.WriteString(fmt.Sprintf("  > %s\n\n", m.filter))
	}

	// Items
	for i, it := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}

		line := it.path
		if it.desc != "" {
			line += " — " + it.desc
		}
		if it.dir {
			line = "\033[36m" + line + "\033[0m"
		}

		if i == m.cursor {
			b.WriteString(fmt.Sprintf("  %s\033[7m%s\033[0m\n", cursor, line))
		} else {
			b.WriteString(fmt.Sprintf("  %s%s\n", cursor, line))
		}
	}

	if len(m.filtered) == 0 {
		b.WriteString("  (no matches)\n")
	}

	// Footer
	action := modes[m.mode]
	b.WriteString(fmt.Sprintf("\n  ↑/↓ move  •  ←/→ mode  •  type to filter  •  Enter %s  •  q quit\n", action))

	return b.String()
}
