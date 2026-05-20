package worker

// GameWorkerState holds all hot-path mutable state for one game worker goroutine.
// Must be exactly 64 bytes to occupy one cache line and prevent false sharing.
type GameWorkerState struct {
	ReadCursor  uint64
	WriteCursor uint64
	FairValue   float64
	Alpha       float64
	QuotesActive bool
	CurrentTier  uint8
	_pad         [24]byte
}
