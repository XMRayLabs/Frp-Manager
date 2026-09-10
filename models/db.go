package models

import (
	"context"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"gorm.io/gorm"
)

type dbManagerImpl struct {
	DBs           map[string]map[string]*gorm.DB // map[db type]map[db role]*gorm.DB
	defaultDBType string
	debug         bool
}

func (dbm *dbManagerImpl) Init() {
	for _, dbGroup := range dbm.DBs {
		for _, db := range dbGroup {
			ctx := context.Background()
			if err := BackupBeforeLanguageMigration(db); err != nil {
				logger.Logger(ctx).WithError(err).Fatal("cannot back up database before 1.1.0 migration")
			}
			if err := db.Migrator().DropIndex(&Endpoint{}, "idx_client_id_host_port"); err != nil {
				logger.Logger(ctx).WithError(err).Infof("cannot drop index [%s], your db is updated", "idx_client_id_host_port")
			}

			if err := MigrateSchema(db); err != nil {
				logger.Logger(ctx).WithError(err).Fatal("cannot initialize database schema")
			}
			if err := MigrateNodeIdentity(db); err != nil {
				logger.Logger(ctx).WithError(err).Fatal("cannot migrate node identity")
			}

			if err := MigrateLanguageGroups(db); err != nil {
				logger.Logger(ctx).WithError(err).Fatal("cannot migrate language groups")
			}
		}
	}
}

func NewDBManager(defaultDBType string) *dbManagerImpl {
	dbs := map[string]map[string]*gorm.DB{}
	return &dbManagerImpl{
		DBs:           dbs,
		defaultDBType: defaultDBType,
	}
}

func (dbm *dbManagerImpl) GetDB(dbType string, dbRole string) *gorm.DB {
	return dbm.DBs[dbType][dbRole]
}

func (dbm *dbManagerImpl) SetDB(dbType string, dbRole string, db *gorm.DB) {
	if dbm.DBs[dbType] == nil {
		dbm.DBs[dbType] = map[string]*gorm.DB{}
	}
	dbm.DBs[dbType][dbRole] = db
}

func (dbm *dbManagerImpl) RemoveDB(dbType string, dbRole string) {
	if dbm.DBs[dbType] == nil {
		return
	}
	delete(dbm.DBs[dbType], dbRole)
}

func (dbm *dbManagerImpl) GetDefaultDB() *gorm.DB {
	dbGroup := dbm.DBs[dbm.defaultDBType]
	if dbm.debug {
		return dbGroup[defs.DBRoleDefault].Debug()
	}
	return dbGroup[defs.DBRoleDefault]
}

func (dbm *dbManagerImpl) SetDebug(debug bool) {
	dbm.debug = debug
}
