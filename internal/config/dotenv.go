package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

const dotenvFile = ".env"

// loadDotEnv reads a .env file and returns key-value pairs.
// If the file does not exist, it returns nil with no error.
func loadDotEnv() (map[string]string, error) {
	f, err := os.Open(dotenvFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip optional "export " prefix
		line = strings.TrimPrefix(line, "export ")

		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = stripQuotes(v)
		env[k] = v
	}
	return env, scanner.Err()
}

// stripQuotes removes surrounding single or double quotes.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// applyDotEnv sets environment variables from a map, but only if they
// are not already set in the process environment. This ensures .env
// has the lowest priority (system env wins).
func applyDotEnv(env map[string]string) {
	for k, v := range env {
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}
