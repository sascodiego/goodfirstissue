package config

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a time.Duration that reads from JSON as a human string ("20s", "1m30s")
// or as a number of seconds. Durations are written as strings by Example.
type Duration time.Duration

// Get returns the value as a time.Duration.
func (d Duration) Get() time.Duration { return time.Duration(d) }

// String renders the duration the way it is written in the file.
func (d Duration) String() string { return time.Duration(d).String() }

// UnmarshalJSON accepts "20s" and 20 (seconds).
func (d *Duration) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid duration %q: use a form like \"20s\" or \"1m30s\"", s)
		}
		*d = Duration(parsed)
		return nil
	}

	var seconds float64
	if err := json.Unmarshal(b, &seconds); err != nil {
		return fmt.Errorf("invalid duration %s: use a string like \"20s\" or a number of seconds", b)
	}
	*d = Duration(time.Duration(seconds * float64(time.Second)))
	return nil
}

// MarshalJSON writes the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	// json.Marshal already quotes a string; adding strconv.Quote here would encode the
	// value twice and produce "\"1m0s\"" on the way back in.
	return json.Marshal(d.String())
}
