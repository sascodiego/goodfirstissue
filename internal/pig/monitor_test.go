package pig

import (
	"testing"
	"time"
)

func TestStallMonitorWarnsOnceThenStops(t *testing.T) {
	m := newStallMonitor(20*time.Second, 90*time.Second)

	if warn, kill := m.observe(19 * time.Second); warn || kill {
		t.Fatalf("below the warn threshold: warn=%v kill=%v, want false/false", warn, kill)
	}

	warn, kill := m.observe(21 * time.Second)
	if !warn || kill {
		t.Fatalf("at warn: warn=%v kill=%v, want true/false", warn, kill)
	}

	// A long legitimate turn keeps being silent; it must not warn twice.
	if warn, _ := m.observe(45 * time.Second); warn {
		t.Error("the warning fired twice")
	}

	warn, kill = m.observe(90 * time.Second)
	if warn || !kill {
		t.Fatalf("at kill: warn=%v kill=%v, want false/true", warn, kill)
	}
}

// A kill threshold that is crossed immediately must not report a warning first: the run
// is over, there is nothing for a warning to describe.
func TestStallMonitorKillWinsOverWarn(t *testing.T) {
	m := newStallMonitor(time.Second, 2*time.Second)
	warn, kill := m.observe(10 * time.Second)
	if warn || !kill {
		t.Fatalf("warn=%v kill=%v, want false/true", warn, kill)
	}
}

// Zero disables a threshold; the caller may want no warning at all.
func TestStallMonitorZeroDisables(t *testing.T) {
	m := newStallMonitor(0, 0)
	if warn, kill := m.observe(time.Hour); warn || kill {
		t.Fatalf("warn=%v kill=%v, want false/false with both disabled", warn, kill)
	}
}
