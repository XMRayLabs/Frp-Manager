package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type GormArray[T any] []T

func (p GormArray[T]) Value() (driver.Value, error) {
	raw, err := json.Marshal(p)
	return string(raw), err
}

func (p *GormArray[T]) Scan(data interface{}) error {
	return scanJSON(data, p)
}

type JSON[T any] struct {
	Data T
}

func (j JSON[T]) Value() (driver.Value, error) {
	raw, err := json.Marshal(j)
	return string(raw), err
}

func (j *JSON[T]) Scan(value interface{}) error {
	return scanJSON(value, j)
}

func scanJSON(value interface{}, target interface{}) error {
	var data []byte
	switch v := value.(type) {
	case nil:
		data = []byte("null")
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported JSON database value %T", value)
	}
	return json.Unmarshal(data, target)
}
