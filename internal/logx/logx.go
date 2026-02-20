package logx

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

var debugEnabled atomic.Bool

// SetDebug toggles debug log emission globally.
func SetDebug(enabled bool) {
	debugEnabled.Store(enabled)
}

// DebugEnabled reports whether debug logs are enabled.
func DebugEnabled() bool {
	return debugEnabled.Load()
}

// Debugf logs a formatted debug line when debug mode is enabled.
func Debugf(format string, args ...interface{}) {
	if !DebugEnabled() {
		return
	}
	log.Printf("DEBUG "+format, args...)
}

// Step measures and logs a named operation for debug mode.
type Step struct {
	name  string
	start time.Time
}

// StepStart marks the beginning of a named step and logs it when debug is enabled.
func StepStart(name string, kv ...interface{}) *Step {
	if DebugEnabled() {
		Debugf("step_start name=%s %s", name, formatKV(kv...))
	}
	return &Step{name: name, start: time.Now()}
}

// Done marks step completion and logs duration when debug is enabled.
func (s *Step) Done(kv ...interface{}) {
	if s == nil || !DebugEnabled() {
		return
	}
	Debugf("step_done name=%s duration=%s %s", s.name, time.Since(s.start), formatKV(kv...))
}

// DoneWithError marks step completion with an error state.
func (s *Step) DoneWithError(err error, kv ...interface{}) {
	if s == nil || !DebugEnabled() {
		return
	}
	fields := append([]interface{}{"status", "error"}, kv...)
	if err != nil {
		fields = append(fields, "error", RedactSensitive(err.Error()))
	}
	Debugf("step_done name=%s duration=%s %s", s.name, time.Since(s.start), formatKV(fields...))
}

// StartWatchdog emits periodic progress logs while waiting for long operations.
func StartWatchdog(name string, interval time.Duration) func() {
	if !DebugEnabled() {
		return func() {}
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}

	stop := make(chan struct{})
	start := time.Now()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				Debugf("step_waiting name=%s elapsed=%s", name, time.Since(start))
			case <-stop:
				return
			}
		}
	}()

	return func() {
		close(stop)
	}
}

func formatKV(kv ...interface{}) string {
	if len(kv) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(kv)/2+1)
	for i := 0; i < len(kv); i += 2 {
		key := fmt.Sprintf("k%d", i)
		if strKey, ok := kv[i].(string); ok && strKey != "" {
			key = strKey
		}
		value := "<missing>"
		if i+1 < len(kv) {
			value = fmt.Sprintf("%v", kv[i+1])
		}
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, RedactSensitive(value)))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, " ")
}
