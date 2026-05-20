package risk

import (
	"testing"
	"unsafe"
)

func TestPositionTrackerSize(t *testing.T) {
	got := unsafe.Sizeof(PositionTracker{})
	if got != 64 {
		t.Errorf("PositionTracker size = %d bytes, want 64; add _pad [%d]byte field", got, 64-got)
	}
}
