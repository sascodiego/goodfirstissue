package pig

import "time"

// stallMonitor decides when a silent agent has stopped producing output.
//
// The JSONL stream carries no usable timestamps, so idleness is measured as the silence
// of the pipe: PiG writes one line per event, so no line means nothing is happening.
type stallMonitor struct {
	warn time.Duration
	kill time.Duration

	warned bool
}

func newStallMonitor(warn, kill time.Duration) *stallMonitor {
	return &stallMonitor{warn: warn, kill: kill}
}

// observe reports whether this silence should raise the warning or stop the run.
// The warning fires once per run; the kill fires as soon as the harder limit is passed.
func (m *stallMonitor) observe(silence time.Duration) (warn, kill bool) {
	if m.kill > 0 && silence >= m.kill {
		return false, true
	}
	if m.warn > 0 && !m.warned && silence >= m.warn {
		m.warned = true
		return true, false
	}
	return false, false
}
