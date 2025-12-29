package triggers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var timeFormatRegex = regexp.MustCompile(`^([0-1]?[0-9]|2[0-3]):([0-5][0-9])$`)

// ParseEverySchedule converts human-friendly schedule syntax to cron expression.
// Supports: hour, day, week, month with optional --at time.
func ParseEverySchedule(every, at, timezone string) (cron, tz string, err error) {
	if every == "" {
		return "", "", fmt.Errorf("--every is required when not using --cron")
	}

	var hour, minute int
	if at != "" {
		matches := timeFormatRegex.FindStringSubmatch(at)
		if matches == nil {
			return "", "", fmt.Errorf("invalid time format for --at, use HH:MM (24-hour)")
		}
		hour, _ = strconv.Atoi(matches[1])
		minute, _ = strconv.Atoi(matches[2])
	}

	switch strings.ToLower(every) {
	case "hour":
		if at != "" {
			return "", "", fmt.Errorf("--at not supported with --every=hour")
		}
		cron = "0 * * * *" // Every hour at minute 0
	case "day":
		cron = fmt.Sprintf("%d %d * * *", minute, hour) // Daily at specified time
	case "week":
		cron = fmt.Sprintf("%d %d * * 1", minute, hour) // Weekly on Monday
	case "month":
		cron = fmt.Sprintf("%d %d 1 * *", minute, hour) // Monthly on the 1st
	default:
		return "", "", fmt.Errorf("invalid --every value, must be: hour, day, week, or month")
	}

	if timezone == "" {
		timezone = "UTC"
	}

	return cron, timezone, nil
}
