// Copyright (c) David Crawshaw
// SPDX-License-Identifier: BSD-3-Clause

package jsonfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustWrite[Data any](t *testing.T, data *JSONFile[Data], fn func(db *Data)) {
	t.Helper()
	if err := data.Write(func(db *Data) error { fn(db); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestBasic(t *testing.T) {
	t.Parallel()
	type Data struct {
		Name    string
		Friends []string
		Ages    map[string]int
	}
	want := Data{
		Name:    "Alice",
		Friends: []string{"Bob", "Carol", "Dave"},
		Ages:    map[string]int{"Bob": 25, "Carol": 30, "Dave": 35},
	}

	path := filepath.Join(t.TempDir(), "testbasic.json")
	data, err := New[Data](path)
	if err != nil {
		t.Fatal(err)
	}

	mustWrite(t, data, func(data *Data) {
		data.Name = want.Name
		data.Friends = append([]string{}, want.Friends...)
		data.Ages = make(map[string]int, len(want.Ages))
		for k, v := range want.Ages {
			data.Ages[k] = v
		}
	})
	mustWrite(t, data, func(*Data) {}) // noop

	data.Read(func(data *Data) {
		if !reflect.DeepEqual(*data, want) {
			t.Errorf("got %+v, want %+v", *data, want)
		}
	})

	data, err = Load[Data](path)
	if err != nil {
		t.Fatal(err)
	}
	data.Read(func(data *Data) {
		if !reflect.DeepEqual(*data, want) {
			t.Errorf("got %+v, want %+v", *data, want)
		}
	})
}

func TestRollbackOnProgramError(t *testing.T) {
	t.Parallel()
	type DB struct{ Val int }

	path := filepath.Join(t.TempDir(), "testrollback.json")
	db, err := New[DB](path)
	if err != nil {
		t.Fatal(err)
	}

	mustWrite(t, db, func(db *DB) { db.Val = 3 })
	mustWrite(t, db, func(db *DB) { db.Val = 1 })

	var rollbackErr = fmt.Errorf("rollback")
	if err := db.Write(func(db *DB) error {
		db.Val = 2
		return rollbackErr
	}); err == nil || !errors.Is(err, rollbackErr) {
		t.Fatalf("Write err=%v, want %v", err, rollbackErr)
	}

	db.Read(func(db *DB) {
		if db.Val != 1 {
			t.Fatalf("Val = %d after rollback, want 1", db.Val)
		}
	})
}

func TestFileError(t *testing.T) {
	t.Parallel()
	type DB struct{ Val int }

	path := filepath.Join(t.TempDir(), "tstdir", "testfserr.json")
	os.MkdirAll(filepath.Dir(path), 0777)
	db, err := New[DB](path)
	if err != nil {
		t.Fatal(err)
	}

	mustWrite(t, db, func(db *DB) { db.Val = 1 })

	if err := os.Chmod(filepath.Dir(path), 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(filepath.Dir(path), 0700)
	})

	if err := db.Write(func(db *DB) error {
		db.Val = 2
		return nil
	}); err == nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Write err=%v, want %v", err, os.ErrPermission)
	}

	db.Read(func(db *DB) {
		if db.Val != 1 {
			t.Fatalf("Val = %d after rollback, want 1", db.Val)
		}
	})
}

func TestNoAlias(t *testing.T) {
	t.Parallel()
	type DB struct{ Vals []int }

	path := filepath.Join(t.TempDir(), "testrollback.json")
	db, err := New[DB](path)
	if err != nil {
		t.Fatal(err)
	}

	someVals := []int{1, 2, 3}
	mustWrite(t, db, func(db *DB) { db.Vals = someVals })

	checkVals := func(db *DB) {
		if !reflect.DeepEqual(db.Vals, []int{1, 2, 3}) {
			t.Fatalf("Vals = %v want 1, 2, 3", db.Vals)
		}
	}

	db.Read(checkVals)
	someVals[0] = 10
	db.Read(checkVals) // db.Vals not aliasing someVals
}

func TestBadLoad(t *testing.T) {
	t.Parallel()
	type DB struct{ Val int }

	path := filepath.Join(t.TempDir(), "testbadload.json")
	_, err := Load[DB](path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load err=%v, want %v", err, os.ErrNotExist)
	}

	if err := os.WriteFile(path, []byte("not json"), 0666); err != nil {
		t.Fatal(err)
	}
	_, err = Load[DB](path)
	if err == nil || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("Load err=%v, want error", err)
	}
}

func TestBadNew(t *testing.T) {
	t.Parallel()
	type DB struct{ Val int }

	_, err := New[DB](t.TempDir())
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("New err=%v, want %v", err, os.ErrExist)
	}
}
