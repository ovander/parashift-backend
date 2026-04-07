package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// date is a small helper that builds a UTC midnight time from y-m-d.
func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

// isoWeek is a test helper that returns the ISO week number for a date.
func isoWeek(t time.Time) int {
	_, w := t.ISOWeek()
	return w
}

// TestWeekType_EvenStartWeek verifies the baseline when startDate falls in an
// even ISO week: weeks with even ISO numbers = A, odd = B.
//
// Jan 5, 2026 (Monday) is ISO week 2 (even) → A=even, B=odd.
func TestWeekType_EvenStartWeek(t *testing.T) {
	start := date(2026, 1, 5) // Monday, ISO week 2 (even)

	// ISO week 2 (Jan 5–11) → even → A
	assert.Equal(t, "A", WeekType(date(2026, 1, 5), start), "Mon Jan 5 — ISO week 2 = A")
	assert.Equal(t, "A", WeekType(date(2026, 1, 11), start), "Sun Jan 11 — ISO week 2 = A")

	// ISO week 3 (Jan 12–18) → odd → B
	assert.Equal(t, "B", WeekType(date(2026, 1, 12), start), "Mon Jan 12 — ISO week 3 = B")
	assert.Equal(t, "B", WeekType(date(2026, 1, 18), start), "Sun Jan 18 — ISO week 3 = B")

	// ISO week 4 (Jan 19–25) → even → A
	assert.Equal(t, "A", WeekType(date(2026, 1, 19), start), "Mon Jan 19 — ISO week 4 = A")
	assert.Equal(t, "A", WeekType(date(2026, 1, 25), start), "Sun Jan 25 — ISO week 4 = A")
}

// TestWeekType_OddStartWeek verifies that an odd-week startDate flips the
// parity: weeks with odd ISO numbers = A, even = B.
//
// Jan 12, 2026 (Monday) is ISO week 3 (odd) → A=odd, B=even.
func TestWeekType_OddStartWeek(t *testing.T) {
	start := date(2026, 1, 12) // Monday, ISO week 3 (odd)

	// ISO week 3 (Jan 12–18) → odd → A
	assert.Equal(t, "A", WeekType(date(2026, 1, 12), start), "Mon Jan 12 — ISO week 3 = A")
	assert.Equal(t, "A", WeekType(date(2026, 1, 18), start), "Sun Jan 18 — ISO week 3 = A")

	// ISO week 4 (Jan 19–25) → even → B
	assert.Equal(t, "B", WeekType(date(2026, 1, 19), start), "Mon Jan 19 — ISO week 4 = B")
	assert.Equal(t, "B", WeekType(date(2026, 1, 25), start), "Sun Jan 25 — ISO week 4 = B")

	// ISO week 5 (Jan 26 – Feb 1) → odd → A
	assert.Equal(t, "A", WeekType(date(2026, 1, 26), start), "Mon Jan 26 — ISO week 5 = A")
}

// TestWeekType_NonMondayStart verifies that a mid-week startDate still
// produces consistent results based on the ISO week of the start date.
//
// Jan 7, 2026 (Wednesday) is ISO week 2 (even) → A=even, B=odd.
// All 7 days of ISO week 3 must be "B", all 7 days of week 4 must be "A".
func TestWeekType_NonMondayStart(t *testing.T) {
	start := date(2026, 1, 7) // Wednesday, ISO week 2 (even)

	// Every day in ISO week 3 (Jan 12–18) must be B.
	for _, d := range []struct {
		t    time.Time
		name string
	}{
		{date(2026, 1, 12), "Mon 12 Jan"},
		{date(2026, 1, 13), "Tue 13 Jan"},
		{date(2026, 1, 14), "Wed 14 Jan"},
		{date(2026, 1, 15), "Thu 15 Jan"},
		{date(2026, 1, 16), "Fri 16 Jan"},
		{date(2026, 1, 17), "Sat 17 Jan"},
		{date(2026, 1, 18), "Sun 18 Jan"},
	} {
		assert.Equal(t, "B", WeekType(d.t, start), "%s (ISO week %d) should be B", d.name, isoWeek(d.t))
	}

	// Every day in ISO week 4 (Jan 19–25) must be A.
	for _, d := range []struct {
		t    time.Time
		name string
	}{
		{date(2026, 1, 19), "Mon 19 Jan"},
		{date(2026, 1, 20), "Tue 20 Jan"},
		{date(2026, 1, 21), "Wed 21 Jan"},
		{date(2026, 1, 22), "Thu 22 Jan"},
		{date(2026, 1, 23), "Fri 23 Jan"},
		{date(2026, 1, 24), "Sat 24 Jan"},
		{date(2026, 1, 25), "Sun 25 Jan"},
	} {
		assert.Equal(t, "A", WeekType(d.t, start), "%s (ISO week %d) should be A", d.name, isoWeek(d.t))
	}
}

// TestWeekType_StartDateSameWeek verifies that a date in the same ISO week as
// startDate always returns "A", regardless of whether startDate is Mon–Sun.
func TestWeekType_StartDateSameWeek(t *testing.T) {
	cases := []struct {
		name  string
		start time.Time
	}{
		{"Monday start",    date(2026, 1, 12)}, // ISO week 3
		{"Wednesday start", date(2026, 1, 14)}, // ISO week 3
		{"Friday start",    date(2026, 1, 16)}, // ISO week 3
		{"Sunday start",    date(2026, 1, 18)}, // ISO week 3
	}
	// Any date in ISO week 3 (Jan 12–18) paired with a startDate also in week 3
	// must return A.
	for _, tc := range cases {
		for _, d := range []time.Time{
			date(2026, 1, 12), date(2026, 1, 14), date(2026, 1, 18),
		} {
			assert.Equal(t, "A", WeekType(d, tc.start),
				"%s: date %v (ISO week %d) should be A", tc.name, d, isoWeek(d))
		}
	}
}

// TestWeekType_YearBoundary verifies week 53/1 year-boundary behaviour.
// Dec 28, 2026 is ISO week 53 (odd).  Jan 4, 2027 is ISO week 1 (odd).
// Both should return the same type when start_date is in an odd week.
func TestWeekType_YearBoundary(t *testing.T) {
	start := date(2026, 12, 28) // ISO week 53 (odd) → A=odd, B=even

	// Week 53 of 2026 (Dec 28 – Jan 3 2027) → odd → A
	assert.Equal(t, "A", WeekType(date(2026, 12, 28), start), "Dec 28 2026 — ISO week 53 = A")
	assert.Equal(t, "A", WeekType(date(2027, 1, 3), start),   "Jan 3 2027  — ISO week 53 = A")

	// Week 1 of 2027 (Jan 4–10) → odd → A
	assert.Equal(t, "A", WeekType(date(2027, 1, 4), start), "Jan 4 2027 — ISO week 1 = A")

	// Week 2 of 2027 (Jan 11–17) → even → B
	assert.Equal(t, "B", WeekType(date(2027, 1, 11), start), "Jan 11 2027 — ISO week 2 = B")
}
