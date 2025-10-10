/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 00:49:01
 * @FilePath: \electron-go-app\backend\internal\infra\security\cipher.go
 * @LastEditTime: 2025-10-11 00:49:06
 */
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	masterKeyOnce sync.Once
	masterKey     []byte
	masterKeyErr  error
)

const masterKeyEnv = "MODEL_CREDENTIAL_MASTER_KEY"

// loadMasterKey 从环境变量加载 AES-256-GCM 主密钥，要求 base64 编码。
func loadMasterKey() ([]byte, error) {
	masterKeyOnce.Do(func() {
		raw := os.Getenv(masterKeyEnv)
		if raw == "" {
			masterKeyErr = fmt.Errorf("%s not configured", masterKeyEnv)
			return
		}
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			masterKeyErr = fmt.Errorf("decode master key: %w", err)
			return
		}
		if len(key) != 32 {
			masterKeyErr = fmt.Errorf("master key must be 32 bytes, got %d", len(key))
			return
		}
		masterKey = key
	})
	return masterKey, masterKeyErr
}

// Encrypt 将明文加密为 nonce|ciphertext 格式的字节切片。
func Encrypt(plaintext []byte) ([]byte, error) {
	key, err := loadMasterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, sealed...), nil
}

// Decrypt 解析 nonce|ciphertext 并返回明文。
func Decrypt(ciphertext []byte) ([]byte, error) {
	key, err := loadMasterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plain, nil
}
