package models

import (
	"fmt"
	"strings"
)

// Enrollment can create a device, but cannot recover another device's credentials.
// New kernels generate and persist this secret before their first enrollment request.
func ValidateEnrollmentDeviceSecret(secret string) error {
	if len(secret) < 32 || len(secret) > 128 || strings.TrimSpace(secret) != secret {
		return fmt.Errorf("secure enrollment requires an updated kernel with a persisted device secret")
	}
	for _, c := range secret {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return fmt.Errorf("invalid enrollment device secret")
		}
	}
	return nil
}
