package execution

// QuoteState holds the current resting quote for one exchange for one game.
// Must be exactly 64 bytes to occupy one cache line and prevent false sharing.
type QuoteState struct {
	BidPrice      float64
	AskPrice      float64
	BidSize       int64
	AskSize       int64
	ExchangeID    uint8
	LastUpdatedNS int64
	_pad          [16]byte
}
