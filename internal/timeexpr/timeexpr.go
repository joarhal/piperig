package timeexpr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var exprRe = regexp.MustCompile(`^([+-]?\d+)([dwmhsDWMHS])$`)

const (
	dateFormat     = "2006-01-02"
	datetimeFormat = "2006-01-02T15:04:05"
)

// IsTimeExpr reports whether expr is a valid time expression like "-1d", "0h", "+2w".
func IsTimeExpr(expr string) bool {
	return exprRe.MatchString(expr)
}

// IsRange reports whether expr is a range expression like "-2d..-1d" or "2026-03-01..2026-03-03".
func IsRange(expr string) bool {
	parts := strings.SplitN(expr, "..", 2)
	if len(parts) != 2 {
		return false
	}
	// Time expr range
	if IsTimeExpr(parts[0]) && IsTimeExpr(parts[1]) {
		return true
	}
	// Absolute date range
	_, err1 := time.Parse(dateFormat, parts[0])
	_, err2 := time.Parse(dateFormat, parts[1])
	return err1 == nil && err2 == nil
}

// Resolve resolves a time expression relative to now.
// Returns a date string (2006-01-02) for day/week suffixes,
// or a datetime string (2006-01-02T15:04:05) for hour/minute/second.
func Resolve(expr string, now time.Time) (string, error) {
	m := exprRe.FindStringSubmatch(expr)
	if m == nil {
		return "", fmt.Errorf("invalid time expression: %q", expr)
	}

	n, _ := strconv.Atoi(m[1])
	suffix := strings.ToLower(m[2])

	base := truncate(now, suffix)
	result := add(base, n, suffix)

	switch suffix {
	case "d", "w":
		return result.Format(dateFormat), nil
	default:
		return result.Format(datetimeFormat), nil
	}
}

// ExpandRange expands a range expression into a list of resolved values.
func ExpandRange(expr string, now time.Time) ([]string, error) {
	parts := strings.SplitN(expr, "..", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range: %q", expr)
	}

	// Absolute date range
	if !IsTimeExpr(parts[0]) {
		return expandAbsoluteRange(parts[0], parts[1])
	}

	// Time expr range — both must have same suffix
	m0 := exprRe.FindStringSubmatch(parts[0])
	m1 := exprRe.FindStringSubmatch(parts[1])
	if m0 == nil || m1 == nil {
		return nil, fmt.Errorf("invalid range: %q", expr)
	}

	s0 := strings.ToLower(m0[2])
	s1 := strings.ToLower(m1[2])
	if s0 != s1 {
		return nil, fmt.Errorf("range suffix mismatch: %s vs %s", s0, s1)
	}

	startStr, err := Resolve(parts[0], now)
	if err != nil {
		return nil, err
	}
	endStr, err := Resolve(parts[1], now)
	if err != nil {
		return nil, err
	}

	return generateSequence(startStr, endStr, s0)
}

func expandAbsoluteRange(startStr, endStr string) ([]string, error) {
	start, err := time.Parse(dateFormat, startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date: %q", startStr)
	}
	end, err := time.Parse(dateFormat, endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date: %q", endStr)
	}
	return generateSequence(start.Format(dateFormat), end.Format(dateFormat), "d")
}

func generateSequence(startStr, endStr, suffix string) ([]string, error) {
	var start, end time.Time
	var format string
	var err error

	switch suffix {
	case "d", "w":
		format = dateFormat
	default:
		format = datetimeFormat
	}

	start, err = time.Parse(format, startStr)
	if err != nil {
		return nil, err
	}
	end, err = time.Parse(format, endStr)
	if err != nil {
		return nil, err
	}

	if start.After(end) {
		return nil, fmt.Errorf("inverted range: %s > %s", startStr, endStr)
	}

	var result []string
	for cur := start; !cur.After(end); cur = add(cur, 1, suffix) {
		result = append(result, cur.Format(format))
	}
	return result, nil
}

func truncate(t time.Time, suffix string) time.Time {
	switch suffix {
	case "d":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case "h":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case "m":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
	case "s":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, t.Location())
	case "w":
		// Truncate to Monday 00:00 of the current week
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		weekday := d.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		return d.AddDate(0, 0, -int(weekday-time.Monday))
	default:
		return t
	}
}

func add(t time.Time, n int, suffix string) time.Time {
	switch suffix {
	case "d":
		return t.AddDate(0, 0, n)
	case "w":
		return t.AddDate(0, 0, n*7)
	case "h":
		return t.Add(time.Duration(n) * time.Hour)
	case "m":
		return t.Add(time.Duration(n) * time.Minute)
	case "s":
		return t.Add(time.Duration(n) * time.Second)
	default:
		return t
	}
}
