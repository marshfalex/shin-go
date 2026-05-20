# PLAN.md — Phase 1: Math Core & Struct Alignment

## Scope

Implement the foundational Go packages that all hot-path logic depends on:
the Power Method devig calculator, the 64-byte aligned critical structs, and
the lock-free ring buffer. No exchange connectivity, no Redis, no worker
orchestration. Every task ends with a passing test and a verified benchmark.

**Reference:** SPEC.md — Power Method, Struct Alignment Invariants, Ring Buffer, Testing Strategy.

---

## Dependency Order

```
Task 1: pkg/contracts (shared types, no logic)
    ↓
Task 2: Critical struct definitions + alignment assertions
    ↓
Task 3: Ring buffer (lock-free, 1024-slot, bitwise index wrap)
    ↓
Task 4: Power Method — 2-way devig
    ↓
Task 5: Power Method — 3-way devig (Shin Method bisection)
    ↓
Task 6: Tier lookup engine (integer-only skew path)
    ↓
Task 7: Execution payload compiler
    ↓
Task 8: End-to-end hot path benchmark (all components wired, no I/O)
```

---

## Tasks

---

### Task 1 — Define shared contracts package

**Goal:** Establish the versioned types used across all packages. No logic — types and constants only.

- [ ] Create `pkg/contracts/types.go`
- [ ] Define tick type enum: `TickType uint8` with constants `SHARP_FEED_TICK`, `BOOK_TICK`, `FILL_TICK`, `CONFIG_UPDATE_TICK`
- [ ] Define `BookTick`, `SharpFeedTick`, `FillTick`, `ConfigUpdateTick` structs
- [ ] Define `CancelReplacePayload`, `BulkCancelPayload` structs
- [ ] Define `QuoteMessage`, `PositionMessage`, `AlphaMessage` Redis schema structs (with `V int` version field = 1)
- [ ] Define `MarketType uint8` constants: `TwoWay`, `ThreeWay`

**Acceptance:** `go build ./pkg/contracts/...` passes. `go vet ./pkg/contracts/...` clean.

**Verify:** `go build ./pkg/contracts/...`

**Files:** `pkg/contracts/types.go`

---

### Task 2 — Critical struct definitions with 64-byte alignment assertions

**Goal:** Define `GameWorkerState`, `PositionTracker`, `QuoteState` with explicit padding and compile-time size enforcement.

- [ ] Create `internal/worker/state.go` with `GameWorkerState`
  - Fields: ring buffer read/write cursors (`uint64`), `FairValue float64`, `Alpha float64`, `QuotesActive bool`, `CurrentTier uint8`
  - Explicit `_pad` field to bring total to 64 bytes
- [ ] Create `internal/risk/position.go` with `PositionTracker`
  - Fields: `NetPosition int64`, `CurrentTier uint8`, `NovigActive bool`, `SporttradeActive bool`
  - Explicit `_pad` field to 64 bytes
- [ ] Create `internal/execution/quote.go` with `QuoteState`
  - Fields: `BidPrice float64`, `AskPrice float64`, `BidSize int64`, `AskSize int64`, `ExchangeID uint8`, `LastUpdatedNS int64`
  - Explicit `_pad` field to 64 bytes
- [ ] Create `internal/worker/state_test.go` — assert `unsafe.Sizeof(GameWorkerState{}) == 64`
- [ ] Create `internal/risk/position_test.go` — assert `unsafe.Sizeof(PositionTracker{}) == 64`
- [ ] Create `internal/execution/quote_test.go` — assert `unsafe.Sizeof(QuoteState{}) == 64`

**Acceptance:** All three size assertions pass. If any struct is mis-padded, `go test` fails — not silently wrong.

**Verify:** `go test ./internal/worker/ ./internal/risk/ ./internal/execution/`

**Files:** `internal/worker/state.go`, `internal/worker/state_test.go`, `internal/risk/position.go`, `internal/risk/position_test.go`, `internal/execution/quote.go`, `internal/execution/quote_test.go`

**Zero-alloc target:** N/A (struct definitions only).

---

### Task 3 — Lock-free SPSC ring buffer

**Goal:** Single-producer single-consumer ring buffer, 1024 slots, bitwise index wrapping, zero heap allocations on enqueue/dequeue.

- [ ] Create `internal/worker/ringbuf.go`
  - Capacity constant: `ringBufCap = 1024`
  - Wrap mask constant: `ringBufMask = ringBufCap - 1`
  - `RingBuffer` struct: `[1024]RingSlot` array (stack/heap allocated at init only), `readCursor uint64`, `writeCursor uint64` — pad each cursor to its own 64-byte cache line
  - `Enqueue(slot RingSlot) bool` — returns false if full, uses `atomic.LoadUint64` / `atomic.StoreUint64`, index via `writeCursor & ringBufMask`
  - `Dequeue() (RingSlot, bool)` — returns false if empty, index via `readCursor & ringBufMask`
- [ ] `RingSlot` struct: `Type TickType`, `GameUUID [36]byte`, `Payload [128]byte` (fixed-size, no interface{})
- [ ] Create `internal/worker/ringbuf_test.go`
  - Test: enqueue 1024 slots, verify full signal on slot 1025
  - Test: enqueue N, dequeue N, verify FIFO order
  - Test: enqueue/dequeue interleaved across wrap boundary (cursor > 1024)
- [ ] Create `internal/worker/ringbuf_bench_test.go`
  - `BenchmarkRingBufferEnqueue` — target: 0 allocs/op
  - `BenchmarkRingBufferDequeue` — target: 0 allocs/op

**Acceptance:** All tests pass. Both benchmarks report 0 allocs/op. `-race` clean.

**Verify:** `go test -bench=BenchmarkRingBuffer -benchmem -run=^$ ./internal/worker/` and `go test -race ./internal/worker/`

**Files:** `internal/worker/ringbuf.go`, `internal/worker/ringbuf_test.go`, `internal/worker/ringbuf_bench_test.go`

**Zero-alloc target:** 0 allocs/op on both Enqueue and Dequeue.

---

### Task 4 — Power Method: 2-way market devig

**Goal:** Closed-form no-vig probability extraction from two American odds values. Zero heap allocations.

- [ ] Create `internal/alpha/devig.go`
- [ ] Implement `americanToImplied(odds int) float64`
  - `odds < 0`: `float64(-odds) / float64(-odds+100)`
  - `odds > 0`: `100.0 / float64(odds+100)`
- [ ] Implement `Devig2Way(o1, o2 int) (p1, p2 float64, err error)`
  - Compute implied probs, divide by overround
  - Return error if either input is 0 or overround ≤ 0
  - Invariant: `|p1 + p2 - 1.0| < 1e-9`
- [ ] Create `internal/alpha/devig_test.go`
  - Table-driven tests with fixed inputs and precomputed expected outputs (verified to 6 decimal places)
  - Cases: -110/-110 (standard vig), -200/+170 (lopsided), -105/-115, edge: 0 odds → error
- [ ] Create `internal/alpha/devig_bench_test.go`
  - `BenchmarkPowerMethod2Way` — target: 0 allocs/op

**Acceptance:** All table tests pass. Benchmark reports 0 allocs/op.

**Verify:** `go test -bench=BenchmarkPowerMethod2Way -benchmem -run=^$ ./internal/alpha/`

**Files:** `internal/alpha/devig.go`, `internal/alpha/devig_test.go`, `internal/alpha/devig_bench_test.go`

**Zero-alloc target:** 0 allocs/op.

---

### Task 5 — Power Method: 3-way market devig (Shin Method bisection)

**Goal:** Iterative Shin Method bisection for three-outcome markets. All scalar math, no allocations, convergence within 50 iterations.

- [ ] Add `Devig3Way(o1, o2, o3 int) (p1, p2, p3 float64, err error)` to `internal/alpha/devig.go`
  - Convert odds to implied probs via `americanToImplied`
  - Bisect z ∈ (0, 1) to satisfy: `Σ [ p_i² / (z + (1-z)·p_i) ] = 1`
  - ε = 1e-9, max_iters = 50
  - Return `ErrNoConvergence` if max_iters exhausted
  - Compute final: `fair_p_i = p_i² / (z + (1-z)·p_i) / S`
  - Invariant: `|p1 + p2 + p3 - 1.0| < 1e-9`
- [ ] Add `ErrNoConvergence` sentinel error to `internal/alpha/devig.go`
- [ ] Add table-driven tests to `internal/alpha/devig_test.go`
  - Standard three-way soccer market odds
  - Heavily skewed favorite
  - Near-equal three-way
  - Verify sum of outputs = 1.0 ± 1e-9 for all cases
- [ ] Add `BenchmarkPowerMethod3Way` to `internal/alpha/devig_bench_test.go`
  - Target: 0 allocs/op

**Acceptance:** All tests pass. Benchmark reports 0 allocs/op. `ErrNoConvergence` tested explicitly with degenerate input.

**Verify:** `go test -bench=BenchmarkPowerMethod3Way -benchmem -run=^$ ./internal/alpha/`

**Files:** `internal/alpha/devig.go` (extended), `internal/alpha/devig_test.go` (extended), `internal/alpha/devig_bench_test.go` (extended)

**Zero-alloc target:** 0 allocs/op.

---

### Task 6 — Tiered inventory skew engine

**Goal:** Integer-only tier lookup and quote offset application. No floating-point in this path.

- [ ] Create `internal/risk/tier.go`
- [ ] Define `TierConfig` struct: `T0, T1, T2 int32`, `Offset1, Offset2, Width1, Width2 int32` (all American odds integer ticks)
- [ ] Define `DefaultTierConfig() TierConfig` returning spec defaults (T0=5, T1=15, T2=30, Offset1=1, Offset2=3, Width1=1, Width2=3)
- [ ] Implement `EvaluateTier(netPosition int32, cfg TierConfig) uint8` — returns tier 0/1/2/3, integer comparison only
- [ ] Implement `ApplySkew(bidOdds, askOdds int32, netPosition int32, cfg TierConfig) (newBid, newAsk int32, tier uint8)`
  - Calls `EvaluateTier` to get tier
  - Tier 0: return bid/ask unchanged
  - Tier 1/2: shift bid and ask by ±OFFSET in direction of net position, widen by WIDTH
  - Tier 3: return 0, 0, 3 (caller fires BulkCancel on tier 3 signal)
  - Integer arithmetic only throughout
- [ ] Create `internal/risk/tier_test.go`
  - Table-driven: net=0 → tier 0, net=T0+1 → tier 1, net=T1+1 → tier 2, net=T2+1 → tier 3
  - Skew direction: long position shifts bid down/ask down, short shifts up
  - Tier 3 returns zero prices
- [ ] Create `internal/risk/tier_bench_test.go`
  - `BenchmarkTierLookup` — target: 0 allocs/op

**Acceptance:** All tests pass. Benchmark reports 0 allocs/op. No `float64` appears anywhere in `tier.go`.

**Verify:** `go test -bench=BenchmarkTierLookup -benchmem -run=^$ ./internal/risk/`

**Files:** `internal/risk/tier.go`, `internal/risk/tier_test.go`, `internal/risk/tier_bench_test.go`

**Zero-alloc target:** 0 allocs/op.

---

### Task 7 — Execution payload compiler

**Goal:** Compile a `CancelReplacePayload` or `BulkCancelPayload` from current quote state and tier signal. Zero allocations — output struct written into a caller-provided pointer.

- [ ] Create `internal/execution/compiler.go`
- [ ] Implement `CompileCancelReplace(dst *CancelReplacePayload, gameUUID [36]byte, exchange uint8, bidOdds, askOdds, bidSize, askSize int32, ts int64)`
  - Writes all fields into `dst` directly. No allocation. No return value.
- [ ] Implement `CompileBulkCancel(dst *BulkCancelPayload, gameUUID [36]byte, exchange uint8, ts int64)`
  - Same pattern.
- [ ] Create `internal/execution/compiler_test.go`
  - Verify all fields written correctly for both payload types
  - Verify UUID is copied by value (no pointer aliasing)
- [ ] Create `internal/execution/compiler_bench_test.go`
  - `BenchmarkPayloadCompile` covering `CompileCancelReplace` — target: 0 allocs/op

**Acceptance:** All tests pass. Benchmark reports 0 allocs/op.

**Verify:** `go test -bench=BenchmarkPayloadCompile -benchmem -run=^$ ./internal/execution/`

**Files:** `internal/execution/compiler.go`, `internal/execution/compiler_test.go`, `internal/execution/compiler_bench_test.go`

**Zero-alloc target:** 0 allocs/op.

---

### Task 8 — End-to-end hot path benchmark (no I/O)

**Goal:** Wire all Phase 1 components into a single synthetic hot path loop and verify the < 1ms p99 budget is achievable before any real I/O is introduced.

- [ ] Create `bench/hotpath_bench_test.go`
- [ ] `BenchmarkHotPathE2E` simulates one full tick cycle:
  1. Dequeue a `SharpFeedTick` from a pre-loaded ring buffer
  2. Call `Devig2Way` or `Devig3Way` (alternating per iteration)
  3. Compute alpha from fair value and a fixed mock market mid
  4. Call `ApplySkew` with current mock net position
  5. Call `CompileCancelReplace` into a stack-allocated payload
- [ ] Assert 0 allocs/op
- [ ] Record p99 latency via `testing.B` timer; fail if p99 > 1ms (use `b.ReportMetric`)

**Acceptance:** 0 allocs/op. p99 < 1ms on developer hardware. If p99 exceeds budget, do not proceed to Phase 2 — investigate and fix before marking complete.

**Verify:** `go test -bench=BenchmarkHotPathE2E -benchmem -benchtime=10s -run=^$ ./bench/`

**Files:** `bench/hotpath_bench_test.go`

**Zero-alloc target:** 0 allocs/op end-to-end.

---

## Phase 1 Completion Gate

Before marking Phase 1 done and moving to Phase 2 (Ring Buffer Worker + Sharp Feed WS Client):

- [ ] `go test ./...` — all tests pass
- [ ] `go test -bench=. -benchmem -run=^$ ./...` — all benchmarks report 0 allocs/op
- [ ] `go test -race ./...` — no data races
- [ ] `go vet ./...` — clean
- [ ] `unsafe.Sizeof` assertions pass for all three critical structs
- [ ] `BenchmarkHotPathE2E` p99 < 1ms
- [ ] All new files committed with passing CI

---

## Out of Scope for Phase 1

- Exchange WebSocket clients (Phase 2)
- Sharp feed WebSocket client (Phase 2)
- Redis Pub/Sub (Phase 3)
- Worker goroutine orchestration (Phase 2)
- Control Plane (Phase 4)
