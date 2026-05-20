package alpha

import "testing"

var (
	sink2   [2]float64
	sink3   [3]float64
	sinkErr error
)

func BenchmarkPowerMethod2Way(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sink2, sinkErr = Devig2Way(-110, -110)
	}
}

func BenchmarkPowerMethod3Way(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sink3, sinkErr = Devig3Way(-125, 270, 450)
	}
}
