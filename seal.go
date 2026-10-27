package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

var errUnseal = errors.New("the stored key cannot be opened with this master key")

func seal(master []byte, plain string) (string, error) {
	gcm, err := newGCM(master)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func unseal(master []byte, sealed string) (string, error) {
	gcm, err := newGCM(master)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", errUnseal
	}
	n := gcm.NonceSize()
	plain, err := gcm.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", errUnseal
	}
	return string(plain), nil
}

func newGCM(master []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(master)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
