package utils

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode"
)

const (
	whiteListChar = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
)

func ValidateUserName(userName string) error {
	userName = strings.TrimSpace(userName)
	if len(userName) < 3 || len(userName) > 64 {
		return fmt.Errorf("username length must be between 3 and 64 characters")
	}
	if !IsClientIDPermited(userName) {
		return fmt.Errorf("username can only contain letters, numbers, hyphen and underscore")
	}
	return nil
}

func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if len(email) < 3 || len(email) > 254 {
		return fmt.Errorf("invalid email length")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("invalid email address")
	}
	return nil
}

func ValidatePassword(password, userName string) error {
	if len(password) < 8 || len(password) > 128 {
		return fmt.Errorf("password length must be between 8 and 128 characters")
	}
	if userName != "" && strings.EqualFold(password, userName) {
		return fmt.Errorf("password must not be the same as username")
	}

	var classes int
	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	for _, ok := range []bool{hasLower, hasUpper, hasDigit, hasSpecial} {
		if ok {
			classes++
		}
	}
	if classes < 3 {
		return fmt.Errorf("password must include at least three of lowercase, uppercase, digit and special characters")
	}
	return nil
}

func IsClientIDPermited(clientID string) bool {
	if len(clientID) == 0 {
		return false
	}

	chrMap := make(map[rune]bool)
	for _, chr := range whiteListChar {
		chrMap[chr] = true
	}

	for _, chr := range clientID {
		if !chrMap[chr] {
			return false
		}
	}

	return true
}

func MakeClientIDPermited(clientID string) string {
	input := []rune(clientID)
	output := input
	chrMap := make(map[rune]bool)
	for _, chr := range whiteListChar {
		chrMap[chr] = true
	}
	for idx, chr := range input {
		if !chrMap[chr] {
			output[idx] = '-'
		}
	}
	return string(output)
}
