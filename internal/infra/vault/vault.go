// Package vault implements domain.CredentialVault with AES-256-GCM
// and versioned keys: ciphertext format "v<N>|<base64(nonce||ct)>",
// which allows key rotation without re-encryption of old data.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

type Vault struct {
	mu      sync.RWMutex
	keys    map[string][]byte // version -> key
	current string
}

// New creates a vault with a single "v1" key derived from the secret.
func New(secret string) *Vault {
	return &Vault{keys: map[string][]byte{"v1": deriveKey(secret)}, current: "v1"}
}

// AddKey registers an additional key version and makes it current (rotation).
func (v *Vault) AddKey(version, secret string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys[version] = deriveKey(secret)
	v.current = version
}

func (v *Vault) Encrypt(plaintext []byte) ([]byte, error) {
	v.mu.RLock()
	version, key := v.current, v.keys[v.current]
	v.mu.RUnlock()

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return []byte(version + "|" + base64.StdEncoding.EncodeToString(sealed)), nil
}

func (v *Vault) Decrypt(ciphertext []byte) ([]byte, error) {
	s := string(ciphertext)
	parts := strings.SplitN(s, "|", 2)
	if len(parts) != 2 {
		return nil, errors.New("vault: bad ciphertext format")
	}
	version, data := parts[0], parts[1]
	v.mu.RLock()
	key, ok := v.keys[version]
	v.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("vault: unknown key version %q", version)
	}
	sealed, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("vault: bad base64: %w", err)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("vault: ciphertext too short")
	}
	nonce, ct := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("vault: decrypt: %w", err)
	}
	return plaintext, nil
}

func deriveKey(secret string) []byte {
	k := sha256.Sum256([]byte(secret))
	return k[:]
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
