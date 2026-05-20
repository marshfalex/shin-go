package risk

import (
	"testing"

	"github.com/marshfalex/shin-go/pkg/contracts"
)

// defaultCfg returns a RiskConfigMessage matching SPEC.md defaults.
func defaultCfg() *contracts.RiskConfigMessage {
	return &contracts.RiskConfigMessage{
		V: 1, T0: 5, T1: 15, T2: 30,
		Offset1: 1, Offset2: 3,
		Width1: 1, Width2: 3,
	}
}

// --- Tier 0: neutral band ---

func TestCalculateSkewTier0Zero(t *testing.T) {
	pos := &PositionTracker{NetPosition: 0}
	offset, widen, active := CalculateSkew(pos, defaultCfg())
	if offset != 0 || widen != 0 || !active {
		t.Errorf("net=0: got (%d,%d,%v), want (0,0,true)", offset, widen, active)
	}
}

func TestCalculateSkewTier0AtBoundary(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: int64(cfg.T0)} // exactly T0 → still tier 0
	offset, widen, active := CalculateSkew(pos, cfg)
	if offset != 0 || widen != 0 || !active {
		t.Errorf("net=T0: got (%d,%d,%v), want (0,0,true)", offset, widen, active)
	}
}

func TestCalculateSkewTier0NegativeAtBoundary(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: -int64(cfg.T0)} // |net|=T0 → tier 0
	offset, widen, active := CalculateSkew(pos, cfg)
	if offset != 0 || widen != 0 || !active {
		t.Errorf("net=-T0: got (%d,%d,%v), want (0,0,true)", offset, widen, active)
	}
}

// --- Tier 1: light skew ---

func TestCalculateSkewTier1Entry(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: int64(cfg.T0) + 1} // just above T0
	offset, widen, active := CalculateSkew(pos, cfg)
	if offset != cfg.Offset1 || widen != cfg.Width1 || !active {
		t.Errorf("net=T0+1: got (%d,%d,%v), want (%d,%d,true)", offset, widen, active, cfg.Offset1, cfg.Width1)
	}
}

func TestCalculateSkewTier1UpperBoundary(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: int64(cfg.T1)} // exactly T1 → tier 1
	offset, widen, active := CalculateSkew(pos, cfg)
	if offset != cfg.Offset1 || widen != cfg.Width1 || !active {
		t.Errorf("net=T1: got (%d,%d,%v), want (%d,%d,true)", offset, widen, active, cfg.Offset1, cfg.Width1)
	}
}

func TestCalculateSkewTier1NegativeEntry(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: -(int64(cfg.T0) + 1)} // |net| in tier 1
	offset, widen, active := CalculateSkew(pos, cfg)
	if offset != cfg.Offset1 || widen != cfg.Width1 || !active {
		t.Errorf("net=-(T0+1): got (%d,%d,%v), want (%d,%d,true)", offset, widen, active, cfg.Offset1, cfg.Width1)
	}
}

// --- Tier 2: heavy skew ---

func TestCalculateSkewTier2Entry(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: int64(cfg.T1) + 1} // just above T1
	offset, widen, active := CalculateSkew(pos, cfg)
	if offset != cfg.Offset2 || widen != cfg.Width2 || !active {
		t.Errorf("net=T1+1: got (%d,%d,%v), want (%d,%d,true)", offset, widen, active, cfg.Offset2, cfg.Width2)
	}
}

func TestCalculateSkewTier2UpperBoundary(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: int64(cfg.T2)} // exactly T2 → tier 2
	offset, widen, active := CalculateSkew(pos, cfg)
	if offset != cfg.Offset2 || widen != cfg.Width2 || !active {
		t.Errorf("net=T2: got (%d,%d,%v), want (%d,%d,true)", offset, widen, active, cfg.Offset2, cfg.Width2)
	}
}

func TestCalculateSkewTier2NegativeEntry(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: -(int64(cfg.T1) + 1)}
	offset, widen, active := CalculateSkew(pos, cfg)
	if offset != cfg.Offset2 || widen != cfg.Width2 || !active {
		t.Errorf("net=-(T1+1): got (%d,%d,%v), want (%d,%d,true)", offset, widen, active, cfg.Offset2, cfg.Width2)
	}
}

// --- Tier 3: max risk — pull quotes ---

func TestCalculateSkewTier3Entry(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: int64(cfg.T2) + 1}
	offset, widen, active := CalculateSkew(pos, cfg)
	if active {
		t.Errorf("net=T2+1: quotesActive=true, want false (tier 3 must pull quotes)")
	}
	if offset != 0 || widen != 0 {
		t.Errorf("net=T2+1: offset=%d widen=%d, want both 0 on tier 3", offset, widen)
	}
}

func TestCalculateSkewTier3LargePosition(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: 999}
	_, _, active := CalculateSkew(pos, cfg)
	if active {
		t.Fatal("large position must have quotesActive=false")
	}
}

func TestCalculateSkewTier3NegativeEntry(t *testing.T) {
	cfg := defaultCfg()
	pos := &PositionTracker{NetPosition: -(int64(cfg.T2) + 1)}
	_, _, active := CalculateSkew(pos, cfg)
	if active {
		t.Errorf("net=-(T2+1): quotesActive=true, want false")
	}
}

// --- CurrentTier field update ---

func TestCalculateSkewUpdatesTierField(t *testing.T) {
	cfg := defaultCfg()
	cases := []struct {
		net      int64
		wantTier uint8
	}{
		{0, 0},
		{int64(cfg.T0), 0},
		{int64(cfg.T0) + 1, 1},
		{int64(cfg.T1), 1},
		{int64(cfg.T1) + 1, 2},
		{int64(cfg.T2), 2},
		{int64(cfg.T2) + 1, 3},
	}
	for _, tc := range cases {
		pos := &PositionTracker{NetPosition: tc.net}
		CalculateSkew(pos, cfg)
		if pos.CurrentTier != tc.wantTier {
			t.Errorf("net=%d: CurrentTier=%d, want %d", tc.net, pos.CurrentTier, tc.wantTier)
		}
	}
}
