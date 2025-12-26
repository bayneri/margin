package analyze

import (
	"fmt"
	"time"
)

func ResolveWindow(start, end string, last time.Duration, now time.Time) (time.Time, time.Time, error) {
	if last > 0 {
		return now.Add(-last), now, nil
	}
	if start == "" || end == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("--start and --end are required unless --last is set")
	}
	parsedStart, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --start: %w", err)
	}
	parsedEnd, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --end: %w", err)
	}
	if parsedEnd.Before(parsedStart) {
		return time.Time{}, time.Time{}, fmt.Errorf("--end must be after --start")
	}
	return parsedStart, parsedEnd, nil
}

func ParseLast(input string) (time.Duration, error) {
	if input == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(input)
	if err != nil {
		return 0, fmt.Errorf("invalid --last: %w", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("--last must be positive")
	}
	return duration, nil
}
