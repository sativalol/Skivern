package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"skyvern/internal/config"
	"strings"
)

const (
	cryptEnvVar = "SKYVERN_CRYPT_KEY"
	defaultKey  = "skyvern_default_master_key_change_me"
	cryptPrefix = "$crypt$gcm$"
)

var aesKey []byte

func getAESKey() []byte {
	if aesKey != nil {
		return aesKey
	}
	keyStr := os.Getenv(cryptEnvVar)
	if keyStr == "" {
		keyStr = config.GetTuiCfg().CryptKey
	}
	if keyStr == "" {
		keyStr = defaultKey
		fmt.Println("[WARNING] SKYVERN_CRYPT_KEY is not set and no crypt_key is found in tui_config.json. Using default encryption key. Secrets are NOT secure. Configure crypt_key in tui_config.json.")
	}
	h := sha256.Sum256([]byte(keyStr))
	aesKey = h[:]
	return aesKey
}

func encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(getAESKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return cryptPrefix + hex.EncodeToString(ciphertext), nil
}

func decrypt(ciphertextStr string) (string, error) {
	if ciphertextStr == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertextStr, cryptPrefix) {
		return ciphertextStr, nil
	}
	payload := strings.TrimPrefix(ciphertextStr, cryptPrefix)
	data, err := hex.DecodeString(payload)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(getAESKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, cipherbytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherbytes, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
