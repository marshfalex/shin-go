# SPEC.md — shin-go: Pure Exchange Real-Time Market Maker

## Objective

Autonomous two-sided market maker that independently quotes Novig and Sporttrade prediction market exchanges. The system anchors fair value to a real-time Pinnacle sharp feed, computes no-vig probabilities via an iterative Power Method loop, and adjusts resting quotes using a discrete tiered inventory skew engine. The primary edge is capturing spread from retail order flow while managing per-exchange inventory risk.

**Target user:** System operator running the bot autonomously.
**Active markets:** 10–50 simultaneous game markets (each a specific line: two-way ML, three-way ML, spread, or total).
**Success:** Bot posts and maintains valid two-sided quotes on both exchanges for every active game, reacts to sharp feed ticks and exchange book movements within the hot path latency budget, and never exceeds the max-risk position threshold without pulling quotes.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Data Plane | Go ≥ 1.22 |
| Control Plane | Next.js ≥ 14 (App Router, TypeScript) |
| IPC Boundary | Redis Pub/Sub (co-located, UNIX socket preferred) |
| Sharp Feed | WebSocket (third-party Pinnacle aggregator or proprietary proxy) |
| Exchange APIs | Novig WebSocket + REST; Sporttrade WebSocket + REST |
| Benchmarking | `go test -bench=. -benchmem -run=^$ ./...` |

---

## Commands

```bash
# Data Plane
go build ./cmd/dataplane/...
go test ./...
go test -bench=. -benchmem -run=^$ ./...
go test -race ./...
go vet ./...

# Control Plane
cd control-plane
npm run dev
npm run build
npm run lint
npm test
```

---

## Project Structure

```
shin-go/
├── cmd/
│   └── dataplane/          # Go binary entrypoint and process lifecycle
├── internal/
│   ├── worker/             # Per-game goroutine, ring buffer, event loop
│   ├── book/               # Exchange order book interfaces + WS clients
│   ├── feed/               # Sharp feed WebSocket client (Pinnacle aggregator)
│   ├── alpha/              # Power Method devig calculator
│   ├── risk/               # Tiered inventory skew engine
│   ├── execution/          # Cancel-replace and bulk-cancel payload compiler
│   └── pubsub/             # Redis Pub/Sub publish/subscribe client
├── pkg/
│   └── contracts/          # Versioned Redis message schemas (shared types)
├── control-plane/          # Next.js application
│   ├── app/                # App Router pages and API routes
│   ├── components/         # UI components
│   └── lib/
│       └── redis/          # Redis subscriber + config publisher
├── SPEC.md
└── CLAUDE.md
```

Tests live alongside source: `internal/alpha/alpha_test.go`, etc. Benchmark-only files use `_bench_test.go` suffix.

---

## Latency Budgets (SLA)

The **local hot path** is defined as: WebSocket tick received → execution payload compiled.
This budget covers both trigger types independently.

| Segment | Budget |
|---|---|
| WS tick receive → ring buffer enqueue | ≤ 50 µs |
| Ring buffer dequeue → worker dispatch | ≤ 20 µs |
| Power Method convergence (sharp feed trigger) | ≤ 400 µs |
| Tier lookup + quote skew calculation | ≤ 100 µs |
| Execution payload struct compilation | ≤ 100 µs |
| **Total hot path (end-to-end local)** | **< 1 ms** |

Exchange API round-trip (order submission) is outside this budget and not subject to the 1ms SLA.

---

## Concurrency Model

The system uses a **sharded per-game-UUID architecture** to eliminate head-of-line blocking and inter-game cache contention.

- One goroutine per active game UUID (10–50 goroutines).
- One lock-free SPSC ring buffer per game, fed by both the sharp feed dispatcher and the exchange book dispatcher.
- No shared mutable state between game goroutines. All game state is owned exclusively by its goroutine.
- The sharp feed client and exchange book clients each run their own goroutines and route ticks to the correct game ring buffer by UUID.
- The 64-byte struct alignment invariant (see below) is load-bearing here: goroutines pinned to different OS threads must not invalidate each other's CPU cache lines.

**Ring buffer tick types:**
```
SHARP_FEED_TICK  — new Pinnacle line for this game's market
BOOK_TICK        — Novig or Sporttrade order book update for this game
```

Both tick types trigger a full hot path execution within the receiving goroutine.

---

## Order Book Interfaces

Abstract interfaces for Novig and Sporttrade. All exchange-specific WS/REST logic is hidden behind these. Mock implementations of both are required for all unit tests; real implementations are integration-tested only.

```go
type ExchangeOrderBook interface {
    // Subscribe begins streaming book updates for the given game UUID.
    Subscribe(gameUUID string) (<-chan BookTick, error)
    Unsubscribe(gameUUID string) error

    // BestBid and BestAsk return the current top-of-book.
    BestBid(gameUUID string) (price float64, size int, err error)
    BestAsk(gameUUID string) (price float64, size int, err error)
}

type ExchangeOrderManager interface {
    // CancelReplace atomically cancels the existing resting order and
    // places a new order at the updated price/size.
    CancelReplace(ctx context.Context, payload CancelReplacePayload) error

    // BulkCancel cancels all resting orders for the given game UUID.
    // Called immediately when max-risk threshold is crossed.
    BulkCancel(ctx context.Context, gameUUID string) error
}
```

`BookTick`, `CancelReplacePayload`, and all shared types are defined in `pkg/contracts`.

---

## Power Method: Single-Market Devig

The Power Method processes one market per execution. It extracts the no-vig implied probability for the target outcome from Pinnacle's vig-inclusive lines.

**Input:** n American odds values where n ∈ {2, 3}
**Output:** `fair_value float64` — no-vig probability [0.0, 1.0] for the target outcome

### Step 1: Convert American Odds to Implied Probability

```
For each odds value o_i:
  if o_i < 0:  p_i = |o_i| / (|o_i| + 100)
  if o_i > 0:  p_i = 100   / (o_i  + 100)
```

### Step 2: Two-Way Market (n = 2) — Closed Form

```
overround  = p_1 + p_2
fair_p_i   = p_i / overround
```
Converges in one step. No iteration required.

### Step 3: Three-Way Market (n = 3) — Shin Method Iteration

Find scalar z ∈ (0, 1) such that the following holds:

```
Σ [ p_i² / (z + (1 - z) · p_i) ]  =  1
```

Solve via bisection on z over (0, 1):

```
ε           = 1e-9          (convergence threshold)
max_iters   = 50            (hard cap; panic if not converged)
z_lo        = 0.0
z_hi        = 1.0

for k = 0 to max_iters:
    z_mid = (z_lo + z_hi) / 2
    S     = Σ [ p_i² / (z_mid + (1 - z_mid) · p_i) ]
    if |S - 1| < ε: break
    if S > 1: z_lo = z_mid
    else:     z_hi = z_mid

fair_p_i = p_i² / (z_mid + (1 - z_mid) · p_i) / S
```

### Step 4: Alpha Derivation

```
market_mid  = (best_bid + best_ask) / 2.0   (from target exchange book)
alpha       = fair_value - market_mid
```

`alpha > 0`: fair value above market mid → edge quoting the ask.
`alpha < 0`: fair value below market mid → edge quoting the bid.

### Invariants

- `fair_value` must satisfy: `0.0 < fair_value < 1.0`
- `Σ fair_p_i` must satisfy: `|Σ fair_p_i - 1.0| < ε`
- If `max_iters` exhausted without convergence: emit error, skip quote update for this tick.
- No floating-point branching in the skew path downstream of this step.

---

## Tiered Inventory Skew Engine

Net position is tracked as a signed integer count of contracts per game per exchange.
Positive = net long; negative = net short.

Tier thresholds and tick offsets are runtime-configurable via the Control Plane (Redis config channel). The engine reads them at startup and on config update ticks.

### Tier Table

| Tier | Condition | Quote Behavior |
|---|---|---|
| 0 — Neutral | `\|net\| ≤ T0` | Symmetric spread, minimum width |
| 1 — Skewed | `T0 < \|net\| ≤ T1` | Shift bid/ask by ±OFFSET_1 ticks in position direction; widen by WIDTH_1 |
| 2 — Heavy | `T1 < \|net\| ≤ T2` | Shift by ±OFFSET_2 ticks; widen by WIDTH_2 |
| 3 — Max Risk | `\|net\| > T2` | Fire `BulkCancel` immediately; set quotes_active = false |

Default parameters (all overridable via config):
```
T0       = 5    contracts
T1       = 15   contracts
T2       = 30   contracts
OFFSET_1 = 1    tick
OFFSET_2 = 3    ticks
WIDTH_1  = 1    tick
WIDTH_2  = 3    ticks
```

### Skew Direction Rule

```
if net_position > 0 (net long):
    bid -= OFFSET_n   (discourage more buys)
    ask -= OFFSET_n   (attract sellers)
if net_position < 0 (net short):
    bid += OFFSET_n   (attract buyers)
    ask += OFFSET_n   (discourage more sells)
spread += WIDTH_n   (symmetric around shifted mid)
```

### Invariants

- Tier lookup uses integer comparison only. No floating-point math.
- `BulkCancel` must fire within the same hot path iteration that detects Tier 3.
- Tier state is re-evaluated on every tick, not cached between ticks.

---

## Struct Alignment Invariants (64-byte Cache Lines)

The following structs are on the hot path and must not share cache lines across goroutines. All must satisfy `unsafe.Sizeof(s) % 64 == 0`.

**Enforcement rule:** A compile-time check using a blank `[0]` array assertion or `_test.go` assertion via `unsafe.Sizeof` must fail the build if alignment is violated. This is non-negotiable — the sharded concurrency model's performance guarantee depends on it.

### Critical Structs

**`GameWorkerState`** — primary per-game hot state:
```
Fields: ring buffer read/write pointers, current fair_value, current alpha,
        quotes_active bool, current tier (uint8)
Padding: to 64-byte boundary
```

**`PositionTracker`** — per-exchange per-game contract counter:
```
Fields: net_position int64, current_tier uint8, novig_quotes_active bool,
        sporttrade_quotes_active bool
Padding: to 64-byte boundary
```

**`QuoteState`** — current resting quote per exchange per game:
```
Fields: bid_price float64, ask_price float64, bid_size int64, ask_size int64,
        exchange_id uint8, last_updated_ns int64
Padding: to 64-byte boundary
```

---

## Data Plane ↔ Control Plane Contract (Redis Pub/Sub)

The boundary is strict: the Go Data Plane **never** reads from a Control Plane process directly. The Next.js Control Plane **never** executes or routes orders. All communication is asynchronous via Redis channels.

### Channels Published by Data Plane

| Channel | Payload | Description |
|---|---|---|
| `quotes:{game_uuid}:{exchange}` | `QuoteMessage` | Current bid/ask + tier after each hot path execution |
| `position:{game_uuid}:{exchange}` | `PositionMessage` | Net contracts + current tier after each fill or cancel |
| `alpha:{game_uuid}` | `AlphaMessage` | Latest fair_value + alpha after each sharp feed tick |
| `system:health` | `HealthMessage` | Per-worker liveness, ring buffer depth, tick rates |

### Channels Published by Control Plane

| Channel | Payload | Description |
|---|---|---|
| `config:risk:{game_uuid}` | `RiskConfigMessage` | Updated tier thresholds and tick offsets |
| `config:games` | `GameListMessage` | Add or remove active game UUIDs |

### Message Schema (all JSON, versioned with `"v":1`)

```
QuoteMessage    { v, game_uuid, exchange, bid, ask, tier, ts_ns }
PositionMessage { v, game_uuid, exchange, net_position, tier, ts_ns }
AlphaMessage    { v, game_uuid, fair_value, alpha, ts_ns }
HealthMessage   { v, game_uuid, worker_status, ring_buf_depth, ticks_per_sec, ts_ns }
RiskConfigMessage { v, game_uuid, t0, t1, t2, offset_1, offset_2, width_1, width_2 }
GameListMessage { v, action, game_uuid, market_type, exchange_ids }
```

Schema definitions live in `pkg/contracts/`. Breaking schema changes require a version bump.

---

## Testing Strategy

### Mandate

TDD is binary. A failing test must exist before any implementation line. No exceptions.

### Levels

| Level | Scope | Location |
|---|---|---|
| Unit | All pure functions (Power Method, tier lookup, payload compiler) | `internal/*/..._test.go` |
| Integration | Redis Pub/Sub round-trip, mock exchange WS client | `internal/*/..._integration_test.go` |
| Benchmark | All hot-path functions | `internal/*/..._bench_test.go` |

### Benchmark Execution

```bash
go test -bench=. -benchmem -run=^$ ./...
```

Required benchmarks (minimum set):

| Benchmark | Target |
|---|---|
| `BenchmarkPowerMethod2Way` | 0 allocs/op |
| `BenchmarkPowerMethod3Way` | 0 allocs/op |
| `BenchmarkTierLookup` | 0 allocs/op |
| `BenchmarkPayloadCompile` | 0 allocs/op |
| `BenchmarkRingBufferEnqueue` | 0 allocs/op |
| `BenchmarkRingBufferDequeue` | 0 allocs/op |
| `BenchmarkHotPathE2E` | < 1ms p99 |

**Regression gate:** Any `allocs/op` increase on an existing benchmark blocks merge. CI must run benchmarks on every PR and diff against `main` baseline.

### Mock Requirements

- `MockExchangeOrderBook` and `MockExchangeOrderManager` must implement the full interface.
- All unit tests use mocks only. Real exchange clients are never instantiated in unit tests.
- Power Method tests use fixed odds inputs with precomputed expected outputs verified to 6 decimal places.

---

## Boundaries

**Always:**
- Red test before any implementation
- Run `-benchmem` and verify 0 allocs/op on hot-path functions before marking a task complete
- 64-byte alignment check in `_test.go` for all three critical structs
- Tier 3 detection must fire `BulkCancel` in the same tick iteration, not deferred
- Schema version bump on any Redis message change

**Ask first:**
- Adding a dependency to the Go Data Plane (any non-stdlib import)
- Adding a new Redis channel
- Changing tier threshold defaults
- Changing the ring buffer capacity

**Never:**
- Import Control Plane packages from the Data Plane or vice versa
- Skip `-race` flag on integration tests
- Use floating-point arithmetic in the tier lookup or skew application path
- Defer `BulkCancel` to the next tick
- Commit without passing `go vet ./...`

---

## Success Criteria

- [ ] All active game workers start, subscribe to sharp feed and both exchange books, and post two-sided quotes within 5 seconds of launch
- [ ] `BenchmarkHotPathE2E` measures < 1ms p99 under simulated load of 50 concurrent game workers
- [ ] `BenchmarkPowerMethod2Way` and `BenchmarkPowerMethod3Way` report 0 allocs/op
- [ ] `unsafe.Sizeof` assertions for `GameWorkerState`, `PositionTracker`, `QuoteState` pass at compile/test time
- [ ] Tier 3 detection triggers `BulkCancel` within the same hot path iteration — verified by integration test with mock exchange
- [ ] Redis Pub/Sub round-trip test: quote published by Data Plane is received by Control Plane subscriber within 10ms
- [ ] `-race` flag produces no data race warnings under 60-second load test

---

## Architectural Decisions (Finalized)

1. **Tick size:** Internal offsets standardized to discrete integer American odds ticks (e.g., -110 → -111). No floating-point arithmetic in the skew application path. `OFFSET_n` and `WIDTH_n` are signed integers representing whole American odds steps.

2. **Fill notification:** Execution reports ingested from private account streams (separate from order book WebSocket). Each report is translated into a signed integer fill count and routed into the game worker's ring buffer as a `FILL_TICK`. This triggers instant inventory re-evaluation within the normal hot path loop — no separate fill-handling goroutine required.

3. **Ring buffer capacity:** Hardcoded at exactly **1024 slots** per game worker. Index wrapping uses bitwise AND: `idx & 1023`. Modulo is forbidden on the hot path.

4. **Config hot-reload:** `RiskConfigMessage` updates are injected into the ring buffer as a `CONFIG_UPDATE_TICK`. The worker processes it sequentially between price evaluations, updating its local risk tier arrays with no race conditions and no locking.

5. **Three-way market targets:** The Power Method calculates fair probabilities for all three outcomes simultaneously per tick. Each outcome maintains a distinct `QuoteState` track. Which outcomes are actively quoted is controlled per-game via the config (`active_outcomes []uint8`). All three fair values are always computed; quoting is gated by the active flag.
