package risk

import "github.com/marshfalex/shin-go/pkg/contracts"

// PositionTracker tracks net contract inventory per exchange per game.
// Must be exactly 64 bytes to occupy one cache line and prevent false sharing.
type PositionTracker struct {
	NetPosition      int64
	CurrentTier      uint8
	NovigActive      bool
	SporttradeActive bool
	_pad             [48]byte
}

// CalculateSkew evaluates the current exposure tier and returns the quote
// adjustment parameters. All comparisons are integer-only; no float arithmetic.
//
// Tier 0 (|net| <= T0): symmetric quotes, no adjustment.
// Tier 1 (T0 < |net| <= T1): apply Offset1 / Width1.
// Tier 2 (T1 < |net| <= T2): apply Offset2 / Width2.
// Tier 3 (|net| > T2): pull all quotes (quotesActive = false).
//
// Updates pos.CurrentTier as a side effect.
func CalculateSkew(pos *PositionTracker, cfg *contracts.RiskConfigMessage) (oddsOffset, spreadWiden int64, quotesActive bool) {
	net := pos.NetPosition
	if net < 0 {
		net = -net
	}

	switch {
	case net <= cfg.T0:
		pos.CurrentTier = 0
		return 0, 0, true
	case net <= cfg.T1:
		pos.CurrentTier = 1
		return cfg.Offset1, cfg.Width1, true
	case net <= cfg.T2:
		pos.CurrentTier = 2
		return cfg.Offset2, cfg.Width2, true
	default:
		pos.CurrentTier = 3
		return 0, 0, false
	}
}
