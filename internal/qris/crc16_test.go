package qris

import "testing"

func TestCalculateCRC16_knownPayload(t *testing.T) {
	// CRC is deterministic for a fixed input prefix.
	s := "000201010211"
	got := CalculateCRC16(s)
	if len(got) != 4 {
		t.Fatalf("want 4 hex chars, got %q", got)
	}
	if got != CalculateCRC16(s) {
		t.Fatal("crc not stable")
	}
}
