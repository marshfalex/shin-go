package worker

import (
	"testing"

	"github.com/marshfalex/shin-go/pkg/contracts"
)

// Package-level sinks prevent the compiler from optimizing away benchmark work.
var (
	sinkSlot RingSlot
	sinkBool bool
)

func BenchmarkRingBufferEnqueue(b *testing.B) {
	rb := NewRingBuffer()
	slot := RingSlot{Type: contracts.TickSharpFeed}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkBool = rb.Enqueue(slot)
		rb.Dequeue() // drain immediately so buffer never fills
	}
}

func BenchmarkRingBufferDequeue(b *testing.B) {
	rb := NewRingBuffer()
	slot := RingSlot{Type: contracts.TickSharpFeed}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rb.Enqueue(slot) // pre-fill one slot per iteration
		sinkSlot, sinkBool = rb.Dequeue()
	}
}
