package shared

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func NewMigrateDatabaseCmd(cfg conf.Config) *cobra.Command {
	var sourcePath, tempDir string
	var offline bool
	cmd := &cobra.Command{Use: "migrate-database", Short: "Copy offline SQLite data into an empty PostgreSQL database", RunE: func(cmd *cobra.Command, args []string) error {
		if !offline {
			return fmt.Errorf("stop the panel, back up its data volume, then pass --offline")
		}
		if cfg.DB.Type != "postgres" {
			return fmt.Errorf("set DB_TYPE=postgres and DB_DSN for the destination")
		}
		path, err := filepath.Abs(sourcePath)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("source must be a SQLite file")
		}
		// Never upgrade the original database. VACUUM INTO includes committed WAL data.
		dir, err := os.MkdirTemp(tempDir, "frp-manager-migrate-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		snapshot := filepath.Join(dir, "snapshot.db")
		sourceURL := url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: "mode=ro"}
		if filepath.VolumeName(path) != "" {
			sourceURL.Path = "/" + sourceURL.Path
		}
		source, err := gorm.Open(sqlite.Open(sourceURL.String()), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
		if err != nil {
			return fmt.Errorf("open source: %w", err)
		}
		raw, err := source.DB()
		if err != nil {
			return err
		}
		raw.SetMaxOpenConns(1)
		err = source.Exec("VACUUM INTO '" + strings.ReplaceAll(filepath.ToSlash(snapshot), "'", "''") + "'").Error
		raw.Close()
		if err != nil {
			return err
		}
		copyDB, err := gorm.Open(sqlite.Open(snapshot), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
		if err != nil {
			return err
		}
		copySQL, err := copyDB.DB()
		if err != nil {
			return err
		}
		defer copySQL.Close()
		copySQL.SetMaxOpenConns(1)
		if !copyDB.Migrator().HasTable(&models.User{}) {
			return fmt.Errorf("source is not a frp-manager database")
		}
		if err = models.MigrateSchema(copyDB); err != nil {
			return err
		}
		if err = models.MigrateNodeIdentity(copyDB); err != nil {
			return err
		}
		if err = models.MigrateLanguageGroups(copyDB); err != nil {
			return err
		}
		destination, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
		if err != nil {
			return fmt.Errorf("cannot connect to PostgreSQL; check DB_DSN and database availability")
		}
		targetSQL, err := destination.DB()
		if err != nil {
			return err
		}
		defer targetSQL.Close()
		targetSQL.SetMaxOpenConns(1)
		counts, err := models.TransferToPostgres(cmd.Context(), copyDB, destination)
		if err != nil {
			return err
		}
		tables := make([]string, 0, len(counts))
		for table := range counts {
			tables = append(tables, table)
		}
		sort.Strings(tables)
		for _, table := range tables {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d rows verified\n", table, counts[table])
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Migration committed. Original SQLite data retained. Keep APP_GLOBAL_SECRET unchanged, then start the panel with PostgreSQL.")
		return nil
	}}
	cmd.Flags().StringVar(&sourcePath, "source", "/data/data.db", "existing SQLite file")
	cmd.Flags().StringVar(&tempDir, "temp-dir", "", "directory with enough disk space for a SQLite snapshot")
	cmd.Flags().BoolVar(&offline, "offline", false, "confirm the panel is stopped and a backup has been made")
	return cmd
}
