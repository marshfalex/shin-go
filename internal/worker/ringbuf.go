package worker

import (
	"sync/atomic"

	"github.com/marshfalex/shin-go/pkg/contracts"
)

const (
	ringBufCap  = 1024
	ringBufMask = ringBufCap - 1
)

// RingSlot is a fixed-size event carrier. All fields are value types to
// guarantee zero heap allocations on enqueue and dequeue.
type RingSlot struct {
	Type     contracts.TickType
	GameUUID [36]byte
	Payload  [128]byte
}

// RingBuffer is a lock-free single-producer single-consumer ring buffer
// with exactly 1024 slots. Cursors occupy separate cache lines to prevent
// false sharing between the producer and consumer goroutines.
type RingBuffer struct {
	// writeCursor is owned exclusively by the producer goroutine.
	writeCursor uint64
	_wPad       [56]byte // pad to 64-byte cache line

	// readCursor is owned exclusively by the consumer goroutine.
	readCursor uint64
	_rPad      [56]byte // pad to 64-byte cache line

	slots [ringBufCap]RingSlot
}

// NewRingBuffer allocates and returns an empty RingBuffer. The allocation
// occurs once at init time; Enqueue and Dequeue are allocation-free.
func NewRingBuffer() *RingBuffer {
	return &RingBuffer{}
}

// Enqueue writes slot into the buffer. Returns false if the buffer is full.
// Must be called from a single producer goroutine only.
func (rb *RingBuffer) Enqueue(slot RingSlot) bool {
	w := atomic.LoadUint64(&rb.writeCursor)
	r := atomic.LoadUint64(&rb.readCursor)
	if w-r >= ringBufCap {
		return false
	}
	rb.slots[w&ringBufMask] = slot
	atomic.AddUint64(&rb.writeCursor, 1)
	return true
}

// Dequeue removes and returns the next slot. Returns false if the buffer is empty.
// Must be called from a single consumer goroutine only.
func (rb *RingBuffer) Dequeue() (RingSlot, bool) {
	r := atomic.LoadUint64(&rb.readCursor)
	w := atomic.LoadUint64(&rb.writeCursor)
	if r == w {
		return RingSlot{}, false
	}
	slot := rb.slots[r&ringBufMask]
	atomic.AddUint64(&rb.readCursor, 1)
	return slot, true
}
