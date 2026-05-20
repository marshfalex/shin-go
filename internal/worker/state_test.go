package worker

import (
	"testing"
	"unsafe"
)

func TestGameWorkerStateSize(t *testing.T) {
	got := unsafe.Sizeof(GameWorkerState{})
	if got != 64 {
		t.Errorf("GameWorkerState size = %d bytes, want 64; add _pad [%d]byte field", got, 64-got)
	}
}
