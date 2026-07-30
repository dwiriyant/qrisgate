package qris

import "testing"

func TestValidate_sampleStatic(t *testing.T) {
	res := Validate(sampleStaticQRIS())
	if !res.Valid {
		t.Fatalf("invalid: %v", res.Errors)
	}
}

func TestValidate_empty(t *testing.T) {
	res := Validate("")
	if res.Valid {
		t.Fatal("expected invalid")
	}
}
