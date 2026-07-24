package config

import (
	"testing"
	"time"
)

func TestFromEnvAndMerge(t *testing.T) {
	t.Setenv("TOTAL_ROUNDS", "6")
	t.Setenv("BUILD_TIMEOUT_SEC", "45")
	m := FromEnv()
	if m.TotalRounds != 6 || m.BuildTimeoutSec != 45 || m.BuildTimeout != 45*time.Second {
		t.Fatalf("%+v", m)
	}
	merged := m.Merge(Match{TotalRounds: 2})
	if merged.TotalRounds != 2 || merged.BuildTimeoutSec != 45 {
		t.Fatalf("%+v", merged)
	}
}
