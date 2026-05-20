package contracts

// TickType identifies the kind of event carried by a ring buffer slot.
type TickType uint8

const (
	TickSharpFeed   TickType = iota + 1
	TickBook
	TickFill
	TickConfigUpdate
)

// MarketType distinguishes two-way from three-way markets.
type MarketType uint8

const (
	TwoWay   MarketType = iota + 1
	ThreeWay
)

// SharpFeedTick carries a Pinnacle line update for one market.
type SharpFeedTick struct {
	GameUUID   [36]byte
	MarketType MarketType
	Odds       [3]int32 // indices 0-1 for TwoWay; 0-2 for ThreeWay; unused = 0
}

// BookTick carries a top-of-book update from an exchange.
type BookTick struct {
	GameUUID   [36]byte
	ExchangeID uint8
	BidOdds    int32
	AskOdds    int32
	BidSize    int32
	AskSize    int32
}

// FillTick carries a signed fill count from an execution report.
// Positive = contracts bought; negative = contracts sold.
type FillTick struct {
	GameUUID   [36]byte
	ExchangeID uint8
	FillCount  int32
}

// ConfigUpdateTick carries updated risk tier parameters for one game.
type ConfigUpdateTick struct {
	GameUUID [36]byte
	T0       int32
	T1       int32
	T2       int32
	Offset1  int32
	Offset2  int32
	Width1   int32
	Width2   int32
}

// CancelReplacePayload is the compiled order management payload sent to an exchange.
type CancelReplacePayload struct {
	GameUUID    [36]byte
	ExchangeID  uint8
	BidOdds     int32
	AskOdds     int32
	BidSize     int32
	AskSize     int32
	TimestampNS int64
}

// BulkCancelPayload cancels all resting orders for a game on one exchange.
type BulkCancelPayload struct {
	GameUUID    [36]byte
	ExchangeID  uint8
	TimestampNS int64
}

// QuoteMessage is the Redis Pub/Sub schema for published quotes (channel: quotes:{game_uuid}:{exchange}).
type QuoteMessage struct {
	V           int    `json:"v"`
	GameUUID    string `json:"game_uuid"`
	Exchange    uint8  `json:"exchange"`
	Bid         int32  `json:"bid"`
	Ask         int32  `json:"ask"`
	Tier        uint8  `json:"tier"`
	TimestampNS int64  `json:"ts_ns"`
}

// PositionMessage is the Redis Pub/Sub schema for position updates (channel: position:{game_uuid}:{exchange}).
type PositionMessage struct {
	V           int    `json:"v"`
	GameUUID    string `json:"game_uuid"`
	Exchange    uint8  `json:"exchange"`
	NetPosition int32  `json:"net_position"`
	Tier        uint8  `json:"tier"`
	TimestampNS int64  `json:"ts_ns"`
}

// RiskConfigMessage is the Redis Pub/Sub schema for risk config updates
// (channel: config:risk:{game_uuid}). Published by the Control Plane.
// All threshold and offset fields are int64 to match PositionTracker.NetPosition
// and eliminate any integer conversion on the hot-path comparison.
type RiskConfigMessage struct {
	V        int    `json:"v"`
	GameUUID string `json:"game_uuid"`
	T0       int64  `json:"t0"`
	T1       int64  `json:"t1"`
	T2       int64  `json:"t2"`
	Offset1  int64  `json:"offset_1"`
	Offset2  int64  `json:"offset_2"`
	Width1   int64  `json:"width_1"`
	Width2   int64  `json:"width_2"`
}

// AlphaMessage is the Redis Pub/Sub schema for fair value updates (channel: alpha:{game_uuid}).
type AlphaMessage struct {
	V           int     `json:"v"`
	GameUUID    string  `json:"game_uuid"`
	FairValue   float64 `json:"fair_value"`
	Alpha       float64 `json:"alpha"`
	TimestampNS int64   `json:"ts_ns"`
}
