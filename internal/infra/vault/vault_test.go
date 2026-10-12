package vault

import (
	"bytes"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	v := New("test-secret")
	pt := []byte("123456:ABC-DEF-token")
	ct, err := v.Encrypt(pt)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ct, pt) {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := v.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("roundtrip mismatch: %q", got)
	}
}

func TestWrongSecret(t *testing.T) {
	ct, err := New("secret-a").Encrypt([]byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New("secret-b").Decrypt(ct); err == nil {
		t.Fatal("want error for wrong secret")
	}
}

func TestVersionedKeyRotation(t *testing.T) {
	v := New("old-secret")
	ct, err := v.Encrypt([]byte("token"))
	if err != nil {
		t.Fatal(err)
	}
	// Rotate: v2 becomes current, old data encrypted with v1 must still decrypt.
	v.AddKey("v2", "new-secret")
	got, err := v.Decrypt(ct)
	if err != nil {
		t.Fatalf("old version must decrypt after rotation: %v", err)
	}
	if !bytes.Equal(got, []byte("token")) {
		t.Fatalf("mismatch: %q", got)
	}
	// New encryptions use v2.
	ct2, err := v.Encrypt([]byte("token2"))
	if err != nil {
		t.Fatal(err)
	}
	if string(ct2[:2]) != "v2" {
		t.Fatalf("new ciphertext must use current version, got %q", string(ct2[:5]))
	}
}
