package pipe

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a .pipe.yaml file. No validation is performed.
func Load(path string) (*Pipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Pipe
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// UnmarshalYAML normalizes YAML scalar values to strings.
// int 80 → "80", bool true → "true", float 0.5 → "0.5", null → "".
func (m *StringMap) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected map, got %v", node.Kind)
	}
	raw := make(map[string]any)
	if err := node.Decode(&raw); err != nil {
		return err
	}
	*m = make(StringMap, len(raw))
	for k, v := range raw {
		if v == nil {
			(*m)[k] = ""
		} else {
			(*m)[k] = fmt.Sprint(v)
		}
	}
	return nil
}

// UnmarshalYAML handles polymorphic loop/each/retry fields on Step.
// It iterates the mapping node manually to inspect each value before decoding.
func (s *Step) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping, got %v", node.Kind)
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]

		switch key.Value {
		case "job":
			s.Job = val.Value
		case "with":
			if err := val.Decode(&s.With); err != nil {
				return fmt.Errorf("with: %w", err)
			}
		case "input":
			s.Input = InputMode(val.Value)
		case "retry_delay":
			s.RetryDelay = val.Value
		case "timeout":
			s.Timeout = val.Value
		case "allow_failure":
			var b bool
			if err := val.Decode(&b); err != nil {
				return fmt.Errorf("allow_failure: %w", err)
			}
			s.AllowFailure = b
		case "loop":
			if val.Kind == yaml.ScalarNode && val.Tag == "!!bool" {
				s.LoopOff = true
			} else {
				var m map[string]any
				if err := val.Decode(&m); err != nil {
					return fmt.Errorf("loop: %w", err)
				}
				s.Loop = m
			}
		case "each":
			if val.Kind == yaml.ScalarNode && val.Tag == "!!bool" {
				s.EachOff = true
			} else {
				var items []StringMap
				if err := val.Decode(&items); err != nil {
					return fmt.Errorf("each: %w", err)
				}
				s.Each = items
			}
		case "retry":
			if val.Kind == yaml.ScalarNode && val.Tag == "!!bool" {
				s.RetryOff = true
			} else {
				var n int
				if err := val.Decode(&n); err != nil {
					return fmt.Errorf("retry: %w", err)
				}
				s.Retry = &n
			}
		}
	}
	return nil
}
