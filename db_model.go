package main

import (
	"time"
)

/// Nullable types
/// --------------

// NullableTime represents time values that can be empty.
type NullableTime struct {
	*time.Time
}

// UnmarshalCSV wraps time.Parse to accept empty strings.
func (t *NullableTime) UnmarshalCSV(s string) error {
	t.Time = nil
	if len(s) == 0 {
		return nil
	}
	ret, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	t.Time = &ret
	return err
}

func (t NullableTime) String() string {
	if t.Time == nil {
		return ""
	}
	return t.Time.String()
}

/// --------------

/// Schemas
/// -------

// Customer represents everyone who bought something.
type Customer struct {
	Name string
}

// Push represents a single unit of work.
type Push struct {
	ID                string
	Purchased         time.Time
	Customer          int
	Goal              string
	Delivered         NullableTime
	Diff              string
	IncludeInEstimate bool
}

/// -------
