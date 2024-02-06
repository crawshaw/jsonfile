// Copyright (c) David Crawshaw
// SPDX-License-Identifier: BSD-3-Clause

// Package jsonfiledb persists a Go value to a JSON file.
package jsonfiledb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// JSONFileDB holds a Go value of type DB and persists it to a JSON file.
// Data is accessed and modified using the Read and Write methods.
// Create a JSONFileDB using the New or Load functions.
type JSONFileDB[DB any] struct {
	path string

	mu   sync.RWMutex
	data []byte
	db   *DB
}

// New creates a new empty JSONFileDB at the given path.
func New[DB any](path string) (*JSONFileDB[DB], error) {
	p := &JSONFileDB[DB]{path: path, data: []byte("{}"), db: new(DB)}
	if err := p.Write(func(db *DB) error { return nil }); err != nil {
		return nil, fmt.Errorf("JSONFileDB.New: %w", err)
	}
	return p, nil
}

// Load loads an existing JSONFileDB from the given path.
//
// If the file does not exist, Load returns an error that can be
// checked with os.IsNotExist.
//
// Load and New are separate to avoid creating a new file when
// starting a service, which could lead to data loss. To both load an
// existing DB or create it (which you may want to do in a development
// environment), combine Load with New, like this:
//
//	db, err := Load[DB](path)
//	if os.IsNotExist(err) {
//		db, err = New[DB](path)
//	}
func Load[DB any](path string) (*JSONFileDB[DB], error) {
	p := &JSONFileDB[DB]{path: path, db: new(DB)}
	var err error
	p.data, err = os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("JSONFileDB.Load: %w", err)
	}
	if err := json.Unmarshal(p.data, p.db); err != nil {
		return nil, fmt.Errorf("JSONFileDB.Load: %w", err)
	}
	return p, nil
}

// Read calls fn with the current copy of the DB.
func (p *JSONFileDB[DB]) Read(fn func(db *DB)) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	fn(p.db)
}

// Write calls fn with a copy of the DB, then writes the changes to the file.
// If fn returns an error, Write does not change the file and returns the error.
func (p *JSONFileDB[DB]) Write(fn func(db *DB) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	db := new(DB) // operate on copy to allow concurrent reads and rollback
	if err := json.Unmarshal(p.data, db); err != nil {
		return fmt.Errorf("JSONFileDB.Write: %w", err)
	}
	if err := fn(db); err != nil {
		return err
	}
	data, err := json.Marshal(db)
	if err != nil {
		return fmt.Errorf("JSONFileDB.Write: %w", err)
	}
	if bytes.Equal(data, p.data) {
		return nil // no change
	}

	f, err := os.CreateTemp(filepath.Dir(p.path), filepath.Base(p.path)+".tmp")
	if err != nil {
		return fmt.Errorf("JSONFileDB.Write: temp: %w", err)
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("JSONFileDB.Write: %w", err)
	}
	if err := os.Rename(f.Name(), p.path); err != nil {
		return fmt.Errorf("JSONFileDB.Write: rename: %w", err)
	}

	db = new(DB) // avoid any aliased memory
	if err := json.Unmarshal(data, db); err != nil {
		return fmt.Errorf("JSONFileDB.Write: %w", err)
	}

	p.db = db
	p.data = data
	return nil
}
