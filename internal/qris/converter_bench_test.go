package qris

import "testing"

func BenchmarkConvert(b *testing.B) {
	static := SampleStaticQRIS()
	opts := ConvertOptions{Amount: 25000}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Convert(static, opts)
	}
}
