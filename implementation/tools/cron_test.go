// Package tools implements the tool execution system for the coding agent.
// This file contains tests for the cron expression parser (requirement 048).
package tools

import (
	"testing"
	"time"
)

func TestParseCronValid(t *testing.T) {
	cases := []struct {
		expr string
		want string // normalized expression
	}{
		{"*/5 * * * *", "*/5 * * * *"},
		{"0 9 * * 1-5", "0 9 * * 1-5"},
		{"@hourly", "0 * * * *"},
		{"@daily", "0 0 * * *"},
		{"@weekly", "0 0 * * 0"},
		{"@monthly", "0 0 1 * *"},
		{"@yearly", "0 0 1 1 *"},
		{"  30   6  *  *  * ", "30 6 * * *"},
		{"0 0 1 jan,mar *", "0 0 1 jan,mar *"},
		{"0 0 * * 7", "0 0 * * 7"},
		{"15 10 * * mon,wed,fri", "15 10 * * mon,wed,fri"},
		{"0 0-6/2 * * *", "0 0-6/2 * * *"},
	}

	for _, tc := range cases {
		sched, err := ParseCron(tc.expr)
		if err != nil {
			t.Errorf("ParseCron(%q) returned error: %v", tc.expr, err)
			continue
		}
		if sched.Expr() != tc.want {
			t.Errorf("ParseCron(%q).Expr() = %q, want %q", tc.expr, sched.Expr(), tc.want)
		}
	}
}

func TestParseCronInvalid(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"* * * *",          // too few fields
		"* * * * * *",      // too many fields
		"60 * * * *",       // minute out of range
		"* 24 * * *",       // hour out of range
		"* * 0 * *",        // day-of-month out of range
		"* * * 13 *",       // month out of range
		"* * * * 8",        // day-of-week out of range
		"*/0 * * * *",      // zero step
		"5-1 * * * *",      // reversed range
		"abc * * * *",      // not a number
		"* * * xyz *",      // unknown month name
		"* * * * nonsense", // unknown weekday name
		"@nonsense",        // unknown macro
		"1-5/0 * * * *",    // zero step in range
		"*/5, * * * *",     // empty list element
		"0 0 30 2 *",       // valid syntax, never fires (checked separately)
	}

	for _, expr := range cases {
		// "0 0 30 2 *" is syntactically valid: skip it in the error table.
		if expr == "0 0 30 2 *" {
			continue
		}
		if _, err := ParseCron(expr); err == nil {
			t.Errorf("ParseCron(%q) expected an error, got none", expr)
		}
	}
}

func TestCronMatches(t *testing.T) {
	// 2026-05-13 is a Wednesday.
	sched, err := ParseCron("30 9 * * 1-5")
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}

	weekday := time.Date(2026, 5, 13, 9, 30, 0, 0, time.Local)
	if !sched.Matches(weekday) {
		t.Errorf("expected %s to match %q", weekday, sched.Expr())
	}

	wrongMinute := time.Date(2026, 5, 13, 9, 31, 0, 0, time.Local)
	if sched.Matches(wrongMinute) {
		t.Errorf("expected %s not to match %q", wrongMinute, sched.Expr())
	}

	weekend := time.Date(2026, 5, 16, 9, 30, 0, 0, time.Local) // Saturday
	if sched.Matches(weekend) {
		t.Errorf("expected %s not to match %q", weekend, sched.Expr())
	}
}

func TestCronSundayNormalization(t *testing.T) {
	seven, err := ParseCron("0 0 * * 7")
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}
	zero, err := ParseCron("0 0 * * 0")
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}

	sunday := time.Date(2026, 5, 17, 0, 0, 0, 0, time.Local) // Sunday
	if !seven.Matches(sunday) || !zero.Matches(sunday) {
		t.Errorf("both 0 and 7 should mean Sunday")
	}
	monday := time.Date(2026, 5, 18, 0, 0, 0, 0, time.Local)
	if seven.Matches(monday) || zero.Matches(monday) {
		t.Errorf("Sunday schedules should not match Monday")
	}
}

func TestCronDaySemantics(t *testing.T) {
	// Both day fields restricted: standard cron matches if EITHER matches.
	both, err := ParseCron("0 0 13 * 5") // the 13th or any Friday
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}
	thirteenthNotFriday := time.Date(2026, 6, 13, 0, 0, 0, 0, time.Local) // Saturday
	if !both.Matches(thirteenthNotFriday) {
		t.Errorf("expected the 13th to match %q", both.Expr())
	}
	fridayNotThirteenth := time.Date(2026, 6, 5, 0, 0, 0, 0, time.Local) // Friday
	if !both.Matches(fridayNotThirteenth) {
		t.Errorf("expected Friday to match %q", both.Expr())
	}
	neither := time.Date(2026, 6, 6, 0, 0, 0, 0, time.Local) // Saturday, not the 13th
	if both.Matches(neither) {
		t.Errorf("expected %s not to match %q", neither, both.Expr())
	}

	// Only day-of-month restricted: weekday must stay unrestricted.
	domOnly, err := ParseCron("0 0 15 * *")
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}
	if !domOnly.Matches(time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)) {
		t.Errorf("expected the 15th to match %q", domOnly.Expr())
	}
	if domOnly.Matches(time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)) {
		t.Errorf("expected the 16th not to match %q", domOnly.Expr())
	}
}

func TestCronNextAfter(t *testing.T) {
	cases := []struct {
		expr string
		from string // RFC3339-ish local time
		want string
	}{
		{"*/5 * * * *", "2026-05-13T10:02:00", "2026-05-13T10:05:00"},
		{"*/5 * * * *", "2026-05-13T10:05:00", "2026-05-13T10:10:00"},
		{"0 9 * * *", "2026-05-13T09:00:00", "2026-05-14T09:00:00"},
		{"0 0 1 * *", "2026-05-13T12:00:00", "2026-06-01T00:00:00"},
		{"30 9 * * 1-5", "2026-05-15T10:00:00", "2026-05-18T09:30:00"}, // Fri -> Mon
		{"0 0 * * *", "2026-02-27T00:00:00", "2026-02-28T00:00:00"},
		{"0 12 29 2 *", "2026-05-13T00:00:00", "2028-02-29T12:00:00"}, // leap year
	}

	for _, tc := range cases {
		from, err := time.ParseInLocation("2006-01-02T15:04:05", tc.from, time.Local)
		if err != nil {
			t.Fatalf("bad test input %q: %v", tc.from, err)
		}
		sched, err := ParseCron(tc.expr)
		if err != nil {
			t.Fatalf("ParseCron(%q) failed: %v", tc.expr, err)
		}
		got := sched.NextAfter(from)
		want, _ := time.ParseInLocation("2006-01-02T15:04:05", tc.want, time.Local)
		if !got.Equal(want) {
			t.Errorf("NextAfter(%q, %q) = %s, want %s", tc.expr, tc.from, got, want)
		}
	}
}

func TestCronNextAfterImpossible(t *testing.T) {
	sched, err := ParseCron("0 0 30 2 *") // February 30th never happens
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}
	if got := sched.NextAfter(time.Now()); !got.IsZero() {
		t.Errorf("expected zero time for impossible schedule, got %s", got)
	}
}

func TestCronNextAfterMatchesSchedule(t *testing.T) {
	// Whatever the starting point, the returned time must match the schedule.
	sched, err := ParseCron("*/7 */3 * * *")
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}
	cur := time.Now()
	for i := 0; i < 20; i++ {
		next := sched.NextAfter(cur)
		if next.IsZero() {
			t.Fatalf("unexpected zero next time")
		}
		if !next.After(cur) {
			t.Fatalf("NextAfter must return a time strictly after the reference (got %s after %s)", next, cur)
		}
		if !sched.Matches(next) {
			t.Fatalf("NextAfter returned %s which does not match %q", next, sched.Expr())
		}
		cur = next
	}
}

// TestCronDayStepIsRestricted guards the day-of-month/day-of-week union rule:
// only a bare "*" marks a day field as unrestricted. A stepped field such as
// "*/2" is restricted, so it must not disable the other day field's match.
func TestCronDayStepIsRestricted(t *testing.T) {
	// "0 0 */2 * *": days 1,3,5,... at midnight, every day field otherwise bare.
	sched, err := ParseCron("0 0 */2 * *")
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}
	from := time.Date(2026, 3, 1, 12, 0, 0, 0, time.Local) // March 1 noon
	n1 := sched.NextAfter(from)
	if n1.Day() != 3 {
		t.Errorf("expected next occurrence on March 3 (odd days only), got %s", n1)
	}
	n2 := sched.NextAfter(n1)
	if n2.Day() != 5 {
		t.Errorf("expected following occurrence on March 5, got %s", n2)
	}
	even := time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)
	if sched.Matches(even) {
		t.Errorf("March 2 (even day) must not match %q", sched.Expr())
	}

	// "0 0 */5 * 1": dom restricted (1,6,11,...) AND dow restricted (Mon).
	// Standard cron: a day matches if EITHER field matches.
	sched, err = ParseCron("0 0 */5 * 1")
	if err != nil {
		t.Fatalf("ParseCron failed: %v", err)
	}
	from = time.Date(2026, 3, 2, 12, 0, 0, 0, time.Local) // Tuesday noon
	n := sched.NextAfter(from)
	if n.Day() != 6 || n.Weekday() == time.Monday {
		t.Errorf("expected the dom match (Fri March 6), got %s (%s)", n, n.Weekday())
	}
	monday := time.Date(2026, 3, 9, 0, 0, 0, 0, time.Local)
	if !sched.Matches(monday) {
		t.Errorf("Monday March 9 must match %q via the day-of-week field", sched.Expr())
	}
	tue := time.Date(2026, 3, 10, 0, 0, 0, 0, time.Local)
	if sched.Matches(tue) {
		t.Errorf("Tuesday March 10 matches neither day rule and must not match %q", sched.Expr())
	}
}
