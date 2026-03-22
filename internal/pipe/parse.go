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
// Nested objects and lists are rejected.
func (m *StringMap) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected map, got %v", node.Kind)
	}
	*m = make(StringMap, len(node.Content)/2)
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch val.Kind {
		case yaml.MappingNode:
			return fmt.Errorf("key %q: nested objects not allowed in with", key)
		case yaml.SequenceNode:
			return fmt.Errorf("key %q: lists not allowed in with", key)
		default:
			if val.Tag == "!!null" {
				(*m)[key] = ""
			} else {
				(*m)[key] = val.Value
			}
		}
	}
	return nil
}

// UnmarshalYAML validates that only known keys appear at pipe level.
func (p *Pipe) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping, got %v", node.Kind)
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		switch key.Value {
		case "description":
			p.Description = val.Value
		case "with":
			if err := val.Decode(&p.With); err != nil {
				return fmt.Errorf("with: %w", err)
			}
		case "loop":
			var m map[string]any
			if err := val.Decode(&m); err != nil {
				return fmt.Errorf("loop: %w", err)
			}
			p.Loop = m
		case "each":
			if err := val.Decode(&p.Each); err != nil {
				return fmt.Errorf("each: %w", err)
			}
		case "input":
			p.Input = InputMode(val.Value)
		case "log":
			if err := val.Decode(&p.Log); err != nil {
				return fmt.Errorf("log: %w", err)
			}
		case "retry":
			var n int
			if err := val.Decode(&n); err != nil {
				return fmt.Errorf("retry: %w", err)
			}
			p.Retry = &n
		case "retry_delay":
			p.RetryDelay = val.Value
		case "timeout":
			p.Timeout = val.Value
		case "steps":
			if err := val.Decode(&p.Steps); err != nil {
				return fmt.Errorf("steps: %w", err)
			}
		default:
			return fmt.Errorf("unknown key %q", key.Value)
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
		case "log":
			if err := val.Decode(&s.Log); err != nil {
				return fmt.Errorf("log: %w", err)
			}
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
		default:
			return fmt.Errorf("unknown key %q", key.Value)
		}
	}
	return nil
}
