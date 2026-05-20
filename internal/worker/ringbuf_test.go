package worker

import (
	"runtime"
	"sync"
	"testing"

	"github.com/marshfalex/shin-go/pkg/contracts"
)

func TestRingBufferEnqueueFull(t *testing.T) {
	rb := NewRingBuffer()
	slot := RingSlot{Type: contracts.TickSharpFeed}

	for i := uint64(0); i < ringBufCap; i++ {
		if !rb.Enqueue(slot) {
			t.Fatalf("Enqueue failed at slot %d before capacity", i)
		}
	}
	if rb.Enqueue(slot) {
		t.Fatal("Enqueue on full buffer returned true, want false")
	}
}

func TestRingBufferDequeueEmpty(t *testing.T) {
	rb := NewRingBuffer()
	if _, ok := rb.Dequeue(); ok {
		t.Fatal("Dequeue on empty buffer returned true, want false")
	}
}

func TestRingBufferFIFO(t *testing.T) {
	rb := NewRingBuffer()
	const n = 16

	for i := 0; i < n; i++ {
		s := RingSlot{Type: contracts.TickBook}
		s.GameUUID[0] = byte(i)
		if !rb.Enqueue(s) {
			t.Fatalf("Enqueue failed at index %d", i)
		}
	}

	for i := 0; i < n; i++ {
		s, ok := rb.Dequeue()
		if !ok {
			t.Fatalf("Dequeue failed at index %d", i)
		}
		if s.GameUUID[0] != byte(i) {
			t.Errorf("index %d: got GameUUID[0]=%d, want %d", i, s.GameUUID[0], byte(i))
		}
	}

	if _, ok := rb.Dequeue(); ok {
		t.Fatal("Dequeue on drained buffer returned true, want false")
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	rb := NewRingBuffer()
	slot := RingSlot{Type: contracts.TickFill}

	// Three full fill-drain cycles forces cursors well past ringBufCap.
	for cycle := 0; cycle < 3; cycle++ {
		for i := uint64(0); i < ringBufCap; i++ {
			if !rb.Enqueue(slot) {
				t.Fatalf("cycle %d: Enqueue failed at slot %d", cycle, i)
			}
		}
		for i := uint64(0); i < ringBufCap; i++ {
			if _, ok := rb.Dequeue(); !ok {
				t.Fatalf("cycle %d: Dequeue failed at slot %d", cycle, i)
			}
		}
	}
}

func TestRingBufferBitwiseMask(t *testing.T) {
	// Verify the bitwise mask constant is correct for 1024-slot buffer.
	if ringBufCap != 1024 {
		t.Fatalf("ringBufCap = %d, want 1024", ringBufCap)
	}
	if ringBufMask != 1023 {
		t.Fatalf("ringBufMask = %d, want 1023", ringBufMask)
	}
	if ringBufCap&ringBufMask != 0 {
		t.Fatal("ringBufCap is not a power of two; bitwise wrap invalid")
	}
}

// TestRingBufferConcurrent exercises SPSC under -race.
// Producer and consumer run in separate goroutines.
func TestRingBufferConcurrent(t *testing.T) {
	rb := NewRingBuffer()
	const n = 50_000
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			s := RingSlot{Type: contracts.TickSharpFeed}
			s.GameUUID[0] = byte(i & 0xFF)
			for !rb.Enqueue(s) {
				runtime.Gosched()
			}
		}
	}()

	go func() {
		defer wg.Done()
		count := 0
		for count < n {
			if _, ok := rb.Dequeue(); ok {
				count++
			} else {
				runtime.Gosched()
			}
		}
	}()

	wg.Wait()
}
