package qris

import "testing"

func TestConvert_injectsAmount(t *testing.T) {
	static := sampleStaticQRIS()
	dynamic := Convert(static, ConvertOptions{Amount: 15000})
	res := Validate(dynamic)
	if !res.Valid {
		t.Fatalf("dynamic invalid: %v", res.Errors)
	}
	parsed := Parse(dynamic)
	if parsed.Method != "dynamic" {
		t.Fatalf("method=%q", parsed.Method)
	}
	if parsed.Amount != "15000" {
		t.Fatalf("amount=%q", parsed.Amount)
	}
}

func TestPNGBase64(t *testing.T) {
	b64, err := PNGBase64(sampleStaticQRIS(), 128)
	if err != nil {
		t.Fatal(err)
	}
	if len(b64) < 100 {
		t.Fatalf("short base64: %d", len(b64))
	}
}
