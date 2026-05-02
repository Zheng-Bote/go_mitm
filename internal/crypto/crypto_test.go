/**
 * SPDX-FileComment: Cryptography Module Tests
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file crypto_test.go
 * @brief Unit Tests for Cryptography Logic
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestEncryptionDecryption(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plaintext := []byte("Hello, world! This is a secret message.")
	
	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Errorf("Ciphertext should not match plaintext")
	}

	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted text does not match original plaintext. Got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestDecryptionWithWrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()
	plaintext := []byte("Secret")

	ciphertext, _ := Encrypt(key1, plaintext)
	
	_, err := Decrypt(key2, ciphertext)
	if err == nil {
		t.Errorf("Decryption should have failed with the wrong key")
	}
}

func TestKeyGeneration(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("Expected 32-byte key, got %d bytes", len(key))
	}
}

func TestDeterministicBase64(t *testing.T) {
	// Ensure we can handle base64 keys as used in main.go
	keyStr := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=" // 32 bytes '12345678901234567890123456789012'
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		t.Fatalf("Failed to decode base64 key: %v", err)
	}

	plaintext := []byte("test")
	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Mismatch")
	}
}
