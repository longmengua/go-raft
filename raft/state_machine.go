package raft

import (
	"bytes"
	"encoding/gob"
	"go-raft/storage"
	"io"
	"sync"

	"github.com/lni/dragonboat/v4/statemachine"
)

var FileDir = "raft-snapshots"

type AssetRaftMachine struct {
	mu    sync.RWMutex
	store *storage.CurrencyStore
}

var _ statemachine.IStateMachine = (*AssetRaftMachine)(nil)

func NewAssetRaftMachine() statemachine.IStateMachine {
	cs := storage.NewCurrencyStore(FileDir)
	_ = cs.Load() // 嘗試從磁碟載入
	return &AssetRaftMachine{store: cs}
}

func (a *AssetRaftMachine) Update(entry statemachine.Entry) (statemachine.Result, error) {
	var cmd storage.AssetCommand
	if err := gob.NewDecoder(bytes.NewReader(entry.Cmd)).Decode(&cmd); err != nil {
		return statemachine.Result{}, err
	}

	a.store.Add(cmd.UID, cmd.Currency, cmd.Amount)
	return statemachine.Result{}, nil
}

func (a *AssetRaftMachine) Lookup(query interface{}) (interface{}, error) {
	if q, ok := query.(storage.AssetCommand); ok {
		return a.store.Get(q.UID, q.Currency), nil
	}
	return nil, nil
}

func (a *AssetRaftMachine) SaveSnapshot(w io.Writer, _ statemachine.ISnapshotFileCollection, _ <-chan struct{}) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.store.Save()
}

func (a *AssetRaftMachine) RecoverFromSnapshot(_ io.Reader, _ []statemachine.SnapshotFile, _ <-chan struct{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.store.Load()
}

func (a *AssetRaftMachine) Close() error { return nil }
