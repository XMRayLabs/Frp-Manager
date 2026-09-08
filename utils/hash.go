package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func MD5[T []byte | string](input T) string {
	data := []byte(input)
	hash := md5.Sum(data)
	hashString := hex.EncodeToString(hash[:])
	return hashString
}

func SHA1(input string) string {
	hash := sha1.Sum([]byte(input))
	return fmt.Sprintf("%x", hash)
}

func SHA256(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash)
}

func SecureStringEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
