package jsonfiledb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func mustWrite[DB any](t *testing.T, db *JSONFileDB[DB], fn func(db *DB)) {
	t.Helper()
	if err := db.Write(func(db *DB) error { fn(db); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestBasic(t *testing.T) {
	t.Parallel()
	type DB struct {
		Name    string
		Friends []string
		Ages    map[string]int
	}
	want := DB{
		Name:    "Alice",
		Friends: []string{"Bob", "Carol", "Dave"},
		Ages:    map[string]int{"Bob": 25, "Carol": 30, "Dave": 35},
	}

	path := filepath.Join(t.TempDir(), "testbasic.json")
	db, err := New[DB](path)
	if err != nil {
		t.Fatal(err)
	}

	mustWrite(t, db, func(db *DB) {
		db.Name = want.Name
		db.Friends = append([]string{}, want.Friends...)
		db.Ages = make(map[string]int, len(want.Ages))
		for k, v := range want.Ages {
			db.Ages[k] = v
		}
	})

	db.Read(func(db *DB) {
		if !reflect.DeepEqual(*db, want) {
			t.Errorf("got %+v, want %+v", *db, want)
		}
	})

	db, err = Load[DB](path)
	if err != nil {
		t.Fatal(err)
	}
	db.Read(func(db *DB) {
		if !reflect.DeepEqual(*db, want) {
			t.Errorf("got %+v, want %+v", *db, want)
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
