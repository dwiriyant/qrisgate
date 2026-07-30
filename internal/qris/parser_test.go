package qris

import "testing"

func TestParse_staticSample(t *testing.T) {
	raw := sampleStaticQRIS()
	d := Parse(raw)
	if d.Method != "static" {
		t.Fatalf("method=%q want static", d.Method)
	}
	if d.MerchantName != "QRIS GATE" {
		t.Fatalf("name=%q", d.MerchantName)
	}
	if len(d.MerchantAccountInfo) == 0 {
		t.Fatal("expected merchant account info")
	}
}
