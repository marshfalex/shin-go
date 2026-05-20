package risk

// PositionTracker tracks net contract inventory per exchange per game.
// Must be exactly 64 bytes to occupy one cache line and prevent false sharing.
type PositionTracker struct {
	NetPosition      int64
	CurrentTier      uint8
	NovigActive      bool
	SporttradeActive bool
	_pad             [48]byte
}
