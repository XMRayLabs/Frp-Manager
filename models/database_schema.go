package models

import (
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// Schema only: importing old data must happen before the data migrations run.
func MigrateSchema(db *gorm.DB) error {
	tables := []interface{}{&NodeAlias{}, &Client{}, &EnrollmentToken{}, &User{}, &Server{}, &Cert{}, &ProxyStats{}, &HistoryProxyStats{}, &Worker{}, &ProxyConfig{}, &UserGroup{}, &InviteCode{}, &SystemSetting{}, &WireGuard{}, &Network{}, &Endpoint{}, &WireGuardLink{}, &LanguageGroup{}, &LanguageGroupServer{}, &OrganizationAudit{}, &gormadapter.CasbinRule{}}
	bootstrap := db.Session(&gorm.Session{NewDB: true})
	config := *bootstrap.Config
	config.DisableForeignKeyConstraintWhenMigrating = true
	bootstrap.Config = &config
	if err := bootstrap.AutoMigrate(tables...); err != nil {
		return err
	}
	return db.AutoMigrate(tables...)
}
