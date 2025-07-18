package raft

import (
	"bytes"
	"encoding/gob"
	"go-raft/internal/domain"
	"go-raft/internal/store"
	"io"
	"sync"

	"github.com/lni/dragonboat/v4/statemachine"
)

type AssetRaftMachine struct {
	mu    sync.RWMutex
	store *store.CurrencyStore
}

var _ statemachine.IStateMachine = (*AssetRaftMachine)(nil)

func NewAssetRaftMachine() statemachine.IStateMachine {
	cs := store.NewCurrencyStore()
	return &AssetRaftMachine{store: cs}
}

func (a *AssetRaftMachine) Update(entry statemachine.Entry) (statemachine.Result, error) {
	var cmd domain.Asset
	if err := gob.NewDecoder(bytes.NewReader(entry.Cmd)).Decode(&cmd); err != nil {
		return statemachine.Result{}, err
	}

	a.store.Update(cmd.UID, cmd.Currency, cmd.Amount)
	return statemachine.Result{}, nil
}

func (a *AssetRaftMachine) Lookup(query interface{}) (interface{}, error) {
	if q, ok := query.(domain.Asset); ok {
		return a.store.Get(q.UID, q.Currency), nil
	}
	return nil, nil
}

func (a *AssetRaftMachine) SaveSnapshot(w io.Writer, _ statemachine.ISnapshotFileCollection, _ <-chan struct{}) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.store.SaveSnapshot(w)
}

func (a *AssetRaftMachine) RecoverFromSnapshot(r io.Reader, _ []statemachine.SnapshotFile, stop <-chan struct{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.store.RecoverFromSnapshot(r, stop)
}

func (a *AssetRaftMachine) Close() error { return nil }
