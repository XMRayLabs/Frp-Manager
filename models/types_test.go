package models

import "testing"

func TestJSONDatabaseValues(t *testing.T) {
	for _, value := range []interface{}{[]byte("[\"a\"]"), "[\"a\"]"} {
		var arr GormArray[string]
		if err := arr.Scan(value); err != nil || len(arr) != 1 || arr[0] != "a" {
			t.Fatalf("%v %v", arr, err)
		}
	}
	for _, value := range []interface{}{[]byte("{\"Data\":{\"port\":6000}}"), "{\"Data\":{\"port\":6000}}"} {
		var obj JSON[map[string]int]
		if err := obj.Scan(value); err != nil || obj.Data["port"] != 6000 {
			t.Fatalf("%v %v", obj, err)
		}
	}
	var obj JSON[map[string]int]
	if err := obj.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if err := obj.Scan(23); err == nil {
		t.Fatal("unexpected type accepted")
	}
}
