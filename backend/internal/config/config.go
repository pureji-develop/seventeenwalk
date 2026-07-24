package config

import (
	"os"
	"strconv"
	"time"
)

// Defaults match design docs.
const (
	DefaultTotalRounds     = 4
	DefaultBuildTimeoutSec = 180
)

// Match holds per-match tunable settings.
type Match struct {
	TotalRounds     int           `json:"totalRounds"`
	BuildTimeoutSec int           `json:"buildTimeoutSec"`
	BuildTimeout    time.Duration `json:"-"`
}

// FromEnv loads defaults overridden by TOTAL_ROUNDS / BUILD_TIMEOUT_SEC.
func FromEnv() Match {
	m := Match{
		TotalRounds:     DefaultTotalRounds,
		BuildTimeoutSec: DefaultBuildTimeoutSec,
	}
	if v := os.Getenv("TOTAL_ROUNDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			m.TotalRounds = n
		}
	}
	if v := os.Getenv("BUILD_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			m.BuildTimeoutSec = n
		}
	}
	m.Normalize()
	return m
}

// Normalize clamps values and fills BuildTimeout.
func (m *Match) Normalize() {
	if m.TotalRounds <= 0 {
		m.TotalRounds = DefaultTotalRounds
	}
	if m.BuildTimeoutSec <= 0 {
		m.BuildTimeoutSec = DefaultBuildTimeoutSec
	}
	m.BuildTimeout = time.Duration(m.BuildTimeoutSec) * time.Second
}

// Merge overlays non-zero fields from o onto m (caller should Normalize after).
func (m Match) Merge(o Match) Match {
	if o.TotalRounds > 0 {
		m.TotalRounds = o.TotalRounds
	}
	if o.BuildTimeoutSec > 0 {
		m.BuildTimeoutSec = o.BuildTimeoutSec
	}
	m.Normalize()
	return m
}
