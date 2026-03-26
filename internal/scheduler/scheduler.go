package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"

	"github.com/joarhal/piperig/internal/config"
	"github.com/joarhal/piperig/internal/expand"
	"github.com/joarhal/piperig/internal/output"
	"github.com/joarhal/piperig/internal/pipe"
	"github.com/joarhal/piperig/internal/runner"
	"github.com/joarhal/piperig/internal/validate"
)

// Entry is a single schedule entry.
type Entry struct {
	Name  string            `yaml:"name"`
	Cron  string            `yaml:"cron"`
	Every string            `yaml:"every"`
	Run   []string          `yaml:"run"`
	With  map[string]string `yaml:"with"`
}

// LoadSchedule parses a schedule YAML file.
func LoadSchedule(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ValidateEntries checks schedule entries for basic validity.
func ValidateEntries(entries []Entry) []error {
	var errs []error
	for i, e := range entries {
		if e.Cron != "" && e.Every != "" {
			errs = append(errs, fmt.Errorf("entry %d (%s): specify cron or every, not both", i+1, e.Name))
		}
		if e.Cron == "" && e.Every == "" {
			errs = append(errs, fmt.Errorf("entry %d (%s): specify cron or every", i+1, e.Name))
		}
		if len(e.Run) == 0 {
			errs = append(errs, fmt.Errorf("entry %d (%s): run list is empty", i+1, e.Name))
		}
		if e.Cron != "" {
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			if _, err := parser.Parse(e.Cron); err != nil {
				errs = append(errs, fmt.Errorf("entry %d (%s): invalid cron: %v", i+1, e.Name, err))
			}
		}
		if e.Every != "" {
			if _, err := time.ParseDuration(e.Every); err != nil {
				errs = append(errs, fmt.Errorf("entry %d (%s): invalid every: %v", i+1, e.Name, err))
			}
		}
	}
	return errs
}

// ServeNow runs all schedule entries once and exits.
func ServeNow(entries []Entry, cfg *config.Config, w *output.Writer) error {
	ctx := context.Background()
	now := time.Now()
	var firstErr error
	for _, e := range entries {
		if err := runEntry(ctx, e, cfg, w, now); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Serve starts a cron loop that runs entries on schedule.
func Serve(ctx context.Context, entries []Entry, cfg *config.Config, w *output.Writer) error {
	c := cron.New()

	for _, e := range entries {
		entry := e // capture for closure
		spec := entry.Cron
		if spec == "" {
			spec = "@every " + entry.Every
		}
		_, err := c.AddFunc(spec, func() {
			now := time.Now()
			if err := runEntry(ctx, entry, cfg, w, now); err != nil {
				fmt.Fprintf(os.Stderr, "schedule %s: %v\n", entry.Name, err)
			}
		})
		if err != nil {
			return fmt.Errorf("entry %s: %v", entry.Name, err)
		}
	}

	c.Start()
	<-ctx.Done()
	c.Stop()
	return nil
}

func runEntry(ctx context.Context, e Entry, cfg *config.Config, w *output.Writer, now time.Time) error {
	for _, target := range e.Run {
		paths, err := resolvePaths(target)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err := runPipe(ctx, path, e.With, cfg, w, now); err != nil {
				return err // fail fast within run list
			}
		}
	}
	return nil
}

func runPipe(ctx context.Context, path string, overrides map[string]string, cfg *config.Config, w *output.Writer, now time.Time) error {
	p, err := pipe.Load(path)
	if err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}

	fileExists := func(f string) bool {
		_, err := os.Stat(f)
		return err == nil
	}

	errs := validate.Validate(p, cfg, fileExists, overrides)
	if len(errs) > 0 {
		return &pipe.ValidationError{Errors: errs}
	}

	plan, err := expand.Expand(p, overrides, now)
	if err != nil {
		return fmt.Errorf("expand %s: %w", path, err)
	}
	plan.Name = filepath.Base(path)

	r := &runner.Runner{
		Interpreters: cfg.Interpreters,
		Output:       w,
		Now:          now,
		Config:       cfg,
	}
	return r.RunPlan(ctx, plan)
}

func resolvePaths(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("not found: %s", target)
	}
	if info.IsDir() {
		return pipe.Scan(target)
	}
	return []string{target}, nil
}
