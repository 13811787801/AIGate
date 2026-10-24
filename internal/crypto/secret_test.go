package crypto

import "testing"

func TestSealOpenRoundTrip(t *testing.T) {
	key := DeriveKey("test-passphrase-m3")
	sealed, err := Seal(key, "sk-secret-123")
	if err != nil {
		t.Fatal(err)
	}
	if !IsSealed(sealed) {
		t.Fatalf("expected sealed prefix, got %q", sealed)
	}
	if sealed == "sk-secret-123" {
		t.Fatal("ciphertext should not equal plaintext")
	}
	pt, err := Open(key, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if pt != "sk-secret-123" {
		t.Fatalf("got %q", pt)
	}
}

func TestOpenLegacyPlaintext(t *testing.T) {
	key := DeriveKey("x")
	pt, err := Open(key, "plain-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if pt != "plain-legacy" {
		t.Fatal(pt)
	}
}
