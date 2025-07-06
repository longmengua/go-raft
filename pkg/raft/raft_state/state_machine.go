package raftstate

import (
	"bytes"
	"encoding/gob"
	raftconfig "go-raft/pkg/raft/raft_config"
	raftmodal "go-raft/pkg/raft/raft_modal"
	raftstore "go-raft/pkg/raft/raft_store"
	"io"
	"sync"

	"github.com/lni/dragonboat/v4/statemachine"
)

type AssetRaftMachine struct {
	mu    sync.RWMutex
	store *raftstore.Currency
}

var _ statemachine.IStateMachine = (*AssetRaftMachine)(nil)

func NewAssetRaftMachine() statemachine.IStateMachine {
	cs := raftstore.NewCurrencyStore(raftconfig.FileDir)
	_ = cs.Load() // 嘗試從磁碟載入
	return &AssetRaftMachine{store: cs}
}

func (a *AssetRaftMachine) Update(entry statemachine.Entry) (statemachine.Result, error) {
	var cmd raftmodal.Asset
	if err := gob.NewDecoder(bytes.NewReader(entry.Cmd)).Decode(&cmd); err != nil {
		return statemachine.Result{}, err
	}

	a.store.Add(cmd.UID, cmd.Currency, cmd.Amount)
	return statemachine.Result{}, nil
}

func (a *AssetRaftMachine) Lookup(query interface{}) (interface{}, error) {
	if q, ok := query.(raftmodal.Asset); ok {
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
