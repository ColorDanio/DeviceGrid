package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}
	if len(key) != 64 {
		t.Errorf("key length = %d, want 64 hex chars", len(key))
	}

	enc, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []string{
		"",
		"hello world",
		"super-secret-password-123!",
		"私钥内容_with_special_chars\n\t",
	}

	for _, plaintext := range tests {
		ciphertext, err := enc.EncryptString(plaintext)
		if err != nil {
			t.Errorf("Encrypt(%q): %v", plaintext, err)
			continue
		}
		if ciphertext == plaintext && plaintext != "" {
			t.Errorf("ciphertext equals plaintext for %q", plaintext)
		}
		decrypted, err := enc.DecryptString(ciphertext)
		if err != nil {
			t.Errorf("Decrypt: %v", err)
			continue
		}
		if decrypted != plaintext {
			t.Errorf("Decrypt = %q, want %q", decrypted, plaintext)
		}
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	enc, _ := New("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	ct1, _ := enc.EncryptString("same-input")
	ct2, _ := enc.EncryptString("same-input")

	if ct1 == ct2 {
		t.Error("same plaintext produced same ciphertext (nonce not random)")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	enc1, _ := New("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	enc2, _ := New("fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")

	ciphertext, _ := enc1.EncryptString("secret")

	_, err := enc2.DecryptString(ciphertext)
	if err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}

func TestNewInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"too short", "abcd"},
		{"not hex", "gggggggggggggggggggggggggggggggg"},
		{"wrong length hex", "0123456789abcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.key)
			if err == nil {
				t.Errorf("New(%q) should return error", tt.key)
			}
		})
	}
}
