package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const EnrollmentTokenPrefix = "fm_join_"

// One durable enrollment credential per account and node role.
// Kept separately from session tokens and per-device connection secrets.
type EnrollmentToken struct {
	UserID    int    `gorm:"primaryKey;autoIncrement:false"`
	Role      string `gorm:"primaryKey;size:16"`
	Token     string `json:"-" gorm:"size:80;not null"`
	Digest    string `json:"-" gorm:"size:64;uniqueIndex;not null"`
	UpdatedAt time.Time
}

func enrollmentDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newEnrollmentToken() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return EnrollmentTokenPrefix + hex.EncodeToString(secret), nil
}

func GetEnrollmentToken(db *gorm.DB, userID int, role string) (string, error) {
	if userID <= 0 || (role != "client" && role != "server") {
		return "", errors.New("invalid enrollment role")
	}
	var row EnrollmentToken
	err := db.Where("user_id = ? AND role = ?", userID, role).First(&row).Error
	if err == nil {
		return row.Token, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	token, err := newEnrollmentToken()
	if err != nil {
		return "", err
	}
	row = EnrollmentToken{UserID: userID, Role: role, Token: token, Digest: enrollmentDigest(token)}
	if err = db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "role"}}, DoNothing: true}).Create(&row).Error; err != nil {
		return "", err
	}
	// Concurrent first reads must all return the winning persisted token.
	err = db.Where("user_id = ? AND role = ?", userID, role).First(&row).Error
	return row.Token, err
}

func RotateEnrollmentToken(db *gorm.DB, userID int, role, current string) (string, error) {
	if current == "" || (role != "client" && role != "server") {
		return "", errors.New("invalid enrollment request")
	}
	token, err := newEnrollmentToken()
	if err != nil {
		return "", err
	}
	result := db.Model(&EnrollmentToken{}).Where("user_id = ? AND role = ? AND digest = ?", userID, role, enrollmentDigest(current)).Updates(map[string]any{"token": token, "digest": enrollmentDigest(token), "updated_at": time.Now()})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", errors.New("接入令牌已被更换，请重新打开接入窗口")
	}
	return token, nil
}

func ResolveEnrollmentToken(db *gorm.DB, token string) (*EnrollmentToken, error) {
	var row EnrollmentToken
	err := db.Where("digest = ?", enrollmentDigest(token)).First(&row).Error
	return &row, err
}
