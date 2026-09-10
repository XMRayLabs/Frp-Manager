package models

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TransferToPostgres requires an offline SQLite snapshot already upgraded to the
// current schema. DDL, rows and sequence repairs commit together in an empty schema.
func TransferToPostgres(ctx context.Context, source, destination *gorm.DB) (map[string]int64, error) {
	if source.Dialector.Name() != "sqlite" || destination.Dialector.Name() != "postgres" {
		return nil, fmt.Errorf("migration requires SQLite source and PostgreSQL destination")
	}
	counts := map[string]int64{}
	err := destination.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'").Scan(&existing).Error; err != nil {
			return err
		}
		if existing != 0 {
			return fmt.Errorf("destination schema must be empty; refusing to overwrite existing tables")
		}
		if err := MigrateSchema(tx); err != nil {
			return err
		}
		tables, err := tx.Migrator().GetTables()
		if err != nil {
			return err
		}
		sort.Strings(tables)
		sourceTables, err := source.Migrator().GetTables()
		if err != nil {
			return err
		}
		known := map[string]bool{}
		for _, table := range tables {
			known[table] = true
		}
		for _, table := range sourceTables {
			if !known[table] && !strings.HasPrefix(table, "sqlite_") {
				return fmt.Errorf("unrecognized source table %s; refusing incomplete migration", table)
			}
		}
		// Defer all FK validation until every base and join table has been copied.
		var constraints []struct {
			TableName      string
			ConstraintName string
		}
		if err = tx.Raw("SELECT table_name, constraint_name FROM information_schema.table_constraints WHERE table_schema = current_schema() AND constraint_type = 'FOREIGN KEY'").Scan(&constraints).Error; err != nil {
			return err
		}
		for _, c := range constraints {
			if err = tx.Exec("ALTER TABLE " + quoteIdentifier(c.TableName) + " ALTER CONSTRAINT " + quoteIdentifier(c.ConstraintName) + " DEFERRABLE INITIALLY DEFERRED").Error; err != nil {
				return err
			}
		}
		for _, table := range tables {
			if !source.Migrator().HasTable(table) {
				continue
			}
			columns, err := tx.Migrator().ColumnTypes(table)
			if err != nil {
				return err
			}
			types := map[string]string{}
			for _, col := range columns {
				types[col.Name()] = strings.ToLower(col.DatabaseTypeName())
			}
			rows, err := source.WithContext(ctx).Table(table).Rows()
			if err != nil {
				return err
			}
			names, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			for _, name := range names {
				if _, ok := types[name]; !ok {
					rows.Close()
					return fmt.Errorf("unrecognized source column %s.%s", table, name)
				}
			}
			// Bounded memory even when traffic history is large; preserve explicit zero values and IDs.
			batch := make([]map[string]interface{}, 0, 100)
			flush := func() error {
				if len(batch) == 0 {
					return nil
				}
				err := tx.Table(table).Create(&batch).Error
				batch = batch[:0]
				return err
			}
			for rows.Next() {
				values := make([]interface{}, len(names))
				pointers := make([]interface{}, len(names))
				for i := range values {
					pointers[i] = &values[i]
				}
				if err = rows.Scan(pointers...); err != nil {
					rows.Close()
					return err
				}
				record := map[string]interface{}{}
				for i, name := range names {
					value, e := postgresValue(values[i], types[name])
					if e != nil {
						rows.Close()
						return fmt.Errorf("%s.%s: %w", table, name, e)
					}
					record[name] = value
				}
				batch = append(batch, record)
				counts[table]++
				if len(batch) == 100 {
					if err = flush(); err != nil {
						rows.Close()
						return fmt.Errorf("copy %s: %w", table, err)
					}
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
			if err = flush(); err != nil {
				return fmt.Errorf("copy %s: %w", table, err)
			}
			var actual int64
			if err = tx.Table(table).Count(&actual).Error; err != nil {
				return err
			}
			if actual != counts[table] {
				return fmt.Errorf("row count mismatch: %s", table)
			}
			for _, col := range columns {
				var sequence *string
				if err = tx.Raw("SELECT pg_get_serial_sequence(?, ?)", quoteIdentifier(table), col.Name()).Scan(&sequence).Error; err != nil {
					return err
				}
				if sequence != nil && *sequence != "" {
					statement := "SELECT setval(?::regclass, GREATEST(COALESCE(MAX(" + quoteIdentifier(col.Name()) + "),0),1), COALESCE(MAX(" + quoteIdentifier(col.Name()) + "),0)>0) FROM " + quoteIdentifier(table)
					if err = tx.Exec(statement, *sequence).Error; err != nil {
						return err
					}
				}
			}
		}
		if err = tx.Exec("SET CONSTRAINTS ALL IMMEDIATE").Error; err != nil {
			return err
		}
		for _, c := range constraints {
			if err = tx.Exec("ALTER TABLE " + quoteIdentifier(c.TableName) + " ALTER CONSTRAINT " + quoteIdentifier(c.ConstraintName) + " NOT DEFERRABLE").Error; err != nil {
				return err
			}
		}
		return nil
	})
	return counts, err
}
func quoteIdentifier(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func postgresValue(value interface{}, kind string) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	switch kind {
	case "bool", "boolean":
		switch v := value.(type) {
		case bool:
			return v, nil
		case int64:
			if v == 0 || v == 1 {
				return v == 1, nil
			}
		}
		return nil, fmt.Errorf("invalid boolean value")
	case "bytea":
		switch v := value.(type) {
		case string:
			return []byte(v), nil
		case []byte:
			return v, nil
		}
	case "timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone":
		if _, ok := value.(time.Time); ok {
			return value, nil
		}
		var text string
		switch v := value.(type) {
		case string:
			text = v
		case []byte:
			text = string(v)
		default:
			return nil, fmt.Errorf("invalid timestamp type %T", value)
		}
		for _, format := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999-07:00 MST", "2006-01-02 15:04:05.999999999", "2006-01-02"} {
			if parsed, err := time.Parse(format, text); err == nil {
				return parsed, nil
			}
		}
		return nil, fmt.Errorf("invalid timestamp")
	default:
		if v, ok := value.([]byte); ok {
			return string(v), nil
		}
	}
	return value, nil
}
