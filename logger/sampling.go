package logger

const samplingMessage = "Setting log level to DEBUG due to sampling rate"

func (c config) sampleLevel(level Level) (Level, bool) {
	if c.rate <= 0 || c.alc != 0 || level <= TraceLevel {
		return level, false
	}
	random := c.random()
	if random >= 0 && random < 100 && float64(random)/100 <= c.rate {
		return DebugLevel, true
	}
	return level, false
}

// Construction owns the first sampling decision; the first invocation reuses it.
func (l *Logger) sampleInitialLevel() {
	level, sampled := l.cfg.sampleLevel(l.base.level)
	l.base.level = level
	if sampled {
		l.report(l.Debug(samplingMessage))
	}
}
