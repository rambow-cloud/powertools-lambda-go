package metrics

import (
	"math"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

var (
	minimumDate = time.UnixMilli(-8640000000000000)
	maximumDate = time.UnixMilli(8640000000000000)
)

// SetTimestamp maps a Go instant to the reference Date input at millisecond precision.
// Values outside the JavaScript Date range warn and serialize as null.
func (m *Metrics) SetTimestamp(timestamp time.Time) error {
	return m.setTimestamp(dateMilliseconds(timestamp), true)
}

// SetTimestampMillis maps the reference numeric timestamp input. Integer values are
// retained even outside CloudWatch's time window. Fractional/non-finite values warn
// and become zero, matching the reference numeric conversion rather than Date input.
func (m *Metrics) SetTimestampMillis(milliseconds float64) error {
	valid := commons.IsIntegerNumber(milliseconds)
	if !valid {
		milliseconds = 0
	}
	return m.setTimestamp(milliseconds, valid)
}

func (m *Metrics) setTimestamp(value float64, valid bool) error {
	if valid {
		now := dateMilliseconds(m.cfg.clock())
		const pastMillis = 14 * 24 * 60 * 60 * 1000
		const futureMillis = 2 * 60 * 60 * 1000
		valid = value >= now-pastMillis && value <= now+futureMillis
	}
	s := m.current
	s.mu.Lock()
	defer m.unlock(s)
	if s.closed.Load() {
		return ErrInvocationClosed
	}
	if !valid {
		s.warn(timestampWarning)
	}
	s.timestamp = &value
	return nil
}

func dateMilliseconds(value time.Time) float64 {
	value = value.Truncate(time.Millisecond)
	if value.Before(minimumDate) || value.After(maximumDate) {
		return math.NaN()
	}
	return float64(value.UnixMilli())
}
