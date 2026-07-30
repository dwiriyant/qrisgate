package auth

import "testing"

func TestAPIKeyRoundTrip(t *testing.T) {
	raw, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 10 || raw[:3] != "qg_" {
		t.Fatalf("bad key %q", raw)
	}
	hash := HashAPIKey(raw)
	if !VerifyAPIKey(raw, hash) {
		t.Fatal("verify failed")
	}
}

func TestSignWebhook(t *testing.T) {
	sig := SignWebhook("secret", []byte("body"))
	if len(sig) != 64 {
		t.Fatalf("sig len %d", len(sig))
	}
}

func TestFormatStoredKey(t *testing.T) {
	hash := HashAPIKey("qg_test")
	got := FormatStoredKey(hash)
	if got != "sha256:"+hash {
		t.Fatalf("got %q", got)
	}
}
