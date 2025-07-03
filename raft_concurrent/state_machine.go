package raftconcurrent

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"go-raft/storage"
	"io"
	"sync"

	"github.com/lni/dragonboat/v4/statemachine"
)

var FileDir = "raft-snapshots"

type AssetConcurrentStateMachine struct {
	mu    sync.RWMutex
	store *storage.CurrencyStore
}

var _ statemachine.IConcurrentStateMachine = (*AssetConcurrentStateMachine)(nil)

func NewAssetRaftConcurrentMachine() statemachine.IConcurrentStateMachine {
	cs := storage.NewCurrencyStore(FileDir)
	_ = cs.Load() // 嘗試從磁碟載入
	return &AssetConcurrentStateMachine{store: cs}
}

// 批次更新
func (a *AssetConcurrentStateMachine) Update(entries []statemachine.Entry) ([]statemachine.Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, entry := range entries {
		var cmd storage.AssetCommand
		if err := gob.NewDecoder(bytes.NewReader(entry.Cmd)).Decode(&cmd); err != nil {
			entries[i].Result = statemachine.Result{Value: 1}
			continue
		}
		a.store.Add(cmd.UID, cmd.Currency, cmd.Amount)
		entries[i].Result = statemachine.Result{Value: 0}
	}
	return entries, nil
}

// 查詢
func (a *AssetConcurrentStateMachine) Lookup(query interface{}) (interface{}, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	switch q := query.(type) {
	case storage.AssetCommand:
		// 查單一使用者幣別餘額
		return a.store.Get(q.UID, q.Currency), nil
	case string:
		if q == "list" {
			result := a.store.List()
			// log.Printf("Returning list data: %+v", result) // 添加日誌
			return result, nil
		}
	}
	return nil, fmt.Errorf("unknown query")
}

// 快照儲存
func (a *AssetConcurrentStateMachine) SaveSnapshot(_ interface{}, w io.Writer, _ statemachine.ISnapshotFileCollection, _ <-chan struct{}) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.store.Save()
}

// 快照回復
func (a *AssetConcurrentStateMachine) RecoverFromSnapshot(_ io.Reader, _ []statemachine.SnapshotFile, _ <-chan struct{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.store.Load()
}

func (a *AssetConcurrentStateMachine) Close() error { return nil }

// PrepareSnapshot implements the statemachine.IConcurrentStateMachine interface.
func (a *AssetConcurrentStateMachine) PrepareSnapshot() (interface{}, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	// Return any state needed for snapshot, or nil if not needed.
	return nil, nil
}
