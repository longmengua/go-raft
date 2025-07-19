package raft

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"go-raft/internal/configs"
	"go-raft/internal/domain"
	"go-raft/internal/store"
	"io"

	"github.com/lni/dragonboat/v4/statemachine"
)

type AssetConcurrentStateMachine struct {
	nodeID    uint64
	clusterID uint64
	store     *store.CurrencyStore
}

var _ statemachine.IConcurrentStateMachine = (*AssetConcurrentStateMachine)(nil)

func NewAssetRaftConcurrentMachine(
	clusterID uint64,
	nodeID uint64,
) statemachine.IConcurrentStateMachine {
	cs := store.NewCurrencyStore(clusterID, nodeID)
	return &AssetConcurrentStateMachine{store: cs, clusterID: clusterID, nodeID: nodeID}
}

// 批次更新
func (a *AssetConcurrentStateMachine) Update(entries []statemachine.Entry) ([]statemachine.Entry, error) {
	defer func() {
		if r := recover(); r != nil {
			// log 錯誤，避免整個 engine 崩潰
			// 例如 logrus.Errorf("Recovered in Update: %v", r)
		}
	}()
	for i, entry := range entries {
		var cmd domain.Asset
		if err := gob.NewDecoder(bytes.NewReader(entry.Cmd)).Decode(&cmd); err != nil {
			entries[i].Result = statemachine.Result{Value: 1}
			continue
		}
		a.store.Update(cmd.UID, cmd.Currency, cmd.Amount)
		entries[i].Result = statemachine.Result{Value: 0}
	}
	return entries, nil
}

// 查詢
func (a *AssetConcurrentStateMachine) Lookup(query any) (any, error) {
	switch q := query.(type) {
	case domain.Asset:
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
func (a *AssetConcurrentStateMachine) SaveSnapshot(_ any, w io.Writer, fss statemachine.ISnapshotFileCollection, done <-chan struct{}) error {
	err := a.store.SaveSnapshot(w, fss, done)
	return err
}

// 快照回復
func (a *AssetConcurrentStateMachine) RecoverFromSnapshot(r io.Reader, files []statemachine.SnapshotFile, done <-chan struct{}) error {
	err := a.store.RecoverFromSnapshot(r, files, done)
	return err
}

// Close 關閉 IConcurrentStateMachine 實例，釋放資源。
// Close 方法不允許改變狀態機中對 Lookup 可見的狀態。
// 注意：Close 不一定會被呼叫，狀態機設計需考慮此點。
func (a *AssetConcurrentStateMachine) Close() error {
	// 若有需要，這裡可以釋放 store 相關資源或設定標誌位
	// 目前此實作無須釋放特別資源，直接回傳 nil 表示正常結束
	return nil
}

// PrepareSnapshot 準備快照，回傳代表狀態識別符的介面值。
// 一般回傳一個描述目前狀態版本的標識，如版本號或序列號。
// PrepareSnapshot 與 Update 互斥調用，可安全讀取狀態。
func (a *AssetConcurrentStateMachine) PrepareSnapshot() (any, error) {
	// 假設 store 有版本號 (Version)，用來標識目前狀態
	// 如果沒有版本控制，可以回傳 nil，表示無特殊標識
	version := configs.GetSnapshotVersion(a.nodeID, a.clusterID)
	return version, nil
}
