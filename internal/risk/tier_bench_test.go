package risk

import (
	"testing"

	"github.com/marshfalex/shin-go/pkg/contracts"
)

var (
	sinkOffset int64
	sinkWiden  int64
	sinkActive bool
)

func BenchmarkTierLookup(b *testing.B) {
	pos := &PositionTracker{NetPosition: 7} // tier 1 — exercises all comparisons
	cfg := &contracts.RiskConfigMessage{
		T0: 5, T1: 15, T2: 30,
		Offset1: 1, Offset2: 3,
		Width1: 1, Width2: 3,
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkOffset, sinkWiden, sinkActive = CalculateSkew(pos, cfg)
	}
}
