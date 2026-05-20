package execution

import (
	"testing"
	"unsafe"
)

func TestQuoteStateSize(t *testing.T) {
	got := unsafe.Sizeof(QuoteState{})
	if got != 64 {
		t.Errorf("QuoteState size = %d bytes, want 64; add _pad [%d]byte field", got, 64-got)
	}
}
