// Package tools implements the tool execution system for the coding agent.
// This file contains a minimal, dependency-free cron expression parser and
// next-occurrence calculator used by the schedule tool (requirement 048).
package tools

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cron macros accepted in place of a 5-field expression.
var cronMacros = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

// Month and weekday names usable in cron fields (case-insensitive).
var cronMonthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var cronWeekdayNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// CronSchedule is a parsed 5-field cron expression:
//
//	minute hour day-of-month month day-of-week
//
// Each field supports "*", lists ("1,2,3"), ranges ("1-5"), steps ("*/5",
// "1-30/2") and, for the month and day-of-week fields, three-letter names.
type CronSchedule struct {
	expr string

	// Bitsets indexed by (value - fieldMin).
	minute []bool
	hour   []bool
	dom    []bool // index = day-1
	month  []bool // index = month-1
	dow    []bool // index = weekday (0-6)

	// domStar/dowStar record whether a day field was written as a bare "*".
	// Standard cron semantics: when both day fields are restricted, a day
	// matches if EITHER field matches; when one is "*", only the other applies.
	domStar bool
	dowStar bool
}

// ParseCron parses a 5-field cron expression or one of the supported macros.
func ParseCron(expr string) (*CronSchedule, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil, fmt.Errorf("empty cron expression")
	}

	if strings.HasPrefix(trimmed, "@") {
		expanded, ok := cronMacros[strings.ToLower(trimmed)]
		if !ok {
			return nil, fmt.Errorf("unknown cron macro %q (supported: @yearly, @monthly, @weekly, @daily, @hourly)", trimmed)
		}
		trimmed = expanded
	}

	fields := strings.Fields(trimmed)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields (minute hour day month weekday), got %d in %q", len(fields), expr)
	}

	sched := &CronSchedule{expr: strings.Join(fields, " ")}

	var err error
	if sched.minute, _, err = parseCronField(fields[0], 0, 59, nil); err != nil {
		return nil, fmt.Errorf("minute field %q: %v", fields[0], err)
	}
	if sched.hour, _, err = parseCronField(fields[1], 0, 23, nil); err != nil {
		return nil, fmt.Errorf("hour field %q: %v", fields[1], err)
	}
	if sched.dom, sched.domStar, err = parseCronField(fields[2], 1, 31, nil); err != nil {
		return nil, fmt.Errorf("day-of-month field %q: %v", fields[2], err)
	}
	if sched.month, _, err = parseCronField(fields[3], 1, 12, cronMonthNames); err != nil {
		return nil, fmt.Errorf("month field %q: %v", fields[3], err)
	}
	// Day-of-week accepts 0-7 where both 0 and 7 mean Sunday; the parsed set is
	// normalized to 0-6.
	var dow7 []bool
	if dow7, sched.dowStar, err = parseCronField(fields[4], 0, 7, cronWeekdayNames); err != nil {
		return nil, fmt.Errorf("day-of-week field %q: %v", fields[4], err)
	}
	sched.dow = make([]bool, 7)
	for v := 0; v <= 7; v++ {
		if dow7[v] {
			sched.dow[v%7] = true
		}
	}

	return sched, nil
}

// Expr returns the normalized (whitespace-collapsed) expression.
func (c *CronSchedule) Expr() string {
	if c == nil {
		return ""
	}
	return c.expr
}

// String implements fmt.Stringer.
func (c *CronSchedule) String() string { return c.Expr() }

// parseCronField parses one cron field into a bitset over [min, max].
// It also reports whether the field was a bare "*". Only the exact spec "*"
// counts: stepped forms such as "*/5" restrict the field (they are not the
// "unrestricted day" marker used by the day-of-month/day-of-week union rule).
func parseCronField(spec string, min, max int, names map[string]int) ([]bool, bool, error) {
	set := make([]bool, max-min+1)
	star := spec == "*"

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false, fmt.Errorf("empty list element")
		}

		body := part
		step := 1
		if idx := strings.Index(part, "/"); idx >= 0 {
			body = strings.TrimSpace(part[:idx])
			stepText := strings.TrimSpace(part[idx+1:])
			s, err := strconv.Atoi(stepText)
			if err != nil || s <= 0 {
				return nil, false, fmt.Errorf("invalid step %q", stepText)
			}
			step = s
		}

		lo, hi := min, max
		switch {
		case body == "*":
			// "*" (optionally with a step) covers the whole range; the
			// field-level star flag was computed above and stays false
			// for stepped forms like "*/5".
		case strings.Contains(body, "-"):
			bounds := strings.SplitN(body, "-", 2)
			a, err := parseCronValue(bounds[0], min, max, names)
			if err != nil {
				return nil, false, err
			}
			b, err := parseCronValue(bounds[1], min, max, names)
			if err != nil {
				return nil, false, err
			}
			if b < a {
				return nil, false, fmt.Errorf("range end %d is before range start %d", b, a)
			}
			lo, hi = a, b
		default:
			v, err := parseCronValue(body, min, max, names)
			if err != nil {
				return nil, false, err
			}
			// "5/10" means "5-max/10" in standard cron; a plain value is a
			// single point.
			if step > 1 {
				hi = max
			} else {
				hi = v
			}
			lo = v
		}

		for v := lo; v <= hi; v += step {
			set[v-min] = true
		}
	}

	for _, on := range set {
		if on {
			return set, star, nil
		}
	}
	return nil, star, fmt.Errorf("field %q matches no values", spec)
}

// parseCronValue parses a single cron value, accepting names when provided.
func parseCronValue(text string, min, max int, names map[string]int) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("empty value")
	}
	if names != nil {
		if v, ok := names[strings.ToLower(text)]; ok {
			return v, nil
		}
		// Accept three-letter prefixes of names (e.g. "monday" -> "mon").
		if len(text) >= 3 {
			if v, ok := names[strings.ToLower(text[:3])]; ok {
				return v, nil
			}
		}
	}
	v, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", text)
	}
	if v < min || v > max {
		return 0, fmt.Errorf("value %d out of range %d-%d", v, min, max)
	}
	return v, nil
}

// Matches reports whether the schedule matches the given time (minute
// granularity).
func (c *CronSchedule) Matches(t time.Time) bool {
	if c == nil {
		return false
	}
	if !c.minute[t.Minute()] || !c.hour[t.Hour()] {
		return false
	}
	if !c.month[int(t.Month())-1] {
		return false
	}
	return c.dayMatches(t)
}

// dayMatches applies the standard cron day-of-month / day-of-week semantics.
func (c *CronSchedule) dayMatches(t time.Time) bool {
	domOK := c.dom[t.Day()-1]
	dowOK := c.dow[int(t.Weekday())]
	switch {
	case c.domStar && c.dowStar:
		return true
	case c.domStar:
		return dowOK
	case c.dowStar:
		return domOK
	default:
		return domOK || dowOK
	}
}

// NextAfter returns the first time strictly after t that matches the schedule,
// evaluated in t's location. It returns the zero time if no occurrence exists
// within the search horizon (five years), which can only happen for impossible
// combinations such as "0 0 30 2 *".
//
// DST note: candidates are built as local wall-clock times. A local time that
// does not exist on a spring-forward day is normalized by time.Date (e.g.
// 02:30 becomes 03:30), and on a fall-back day the first occurrence of the
// repeated wall-clock time is used.
func (c *CronSchedule) NextAfter(t time.Time) time.Time {
	if c == nil {
		return time.Time{}
	}
	loc := t.Location()

	// Start at the next whole minute after t.
	start := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc).Add(time.Minute)
	limit := start.AddDate(5, 0, 0)

	for day := start; day.Before(limit); day = day.AddDate(0, 0, 1) {
		if !c.month[int(day.Month())-1] || !c.dayMatches(day) {
			continue
		}
		for h := 0; h < 24; h++ {
			if !c.hour[h] {
				continue
			}
			for m := 0; m < 60; m++ {
				if !c.minute[m] {
					continue
				}
				cand := time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, loc)
				if cand.Before(start) {
					continue
				}
				return cand
			}
		}
	}

	return time.Time{}
}
