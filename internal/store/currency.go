package store

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"go-raft/internal/configs"
	"go-raft/pkg/maps"
	maps0 "maps"

	"github.com/golang/snappy"
	"github.com/lni/dragonboat/v4/statemachine"
)

// Gob 註冊用，確保 gob 可以序列化這些結構
func init() {
	gob.Register(&StoreV1{})
	gob.Register(&StoreV2{})
	gob.Register(map[string]float64{})
}

// StoreV1 是 Snapshot 版本 1 的資料格式
type StoreV1 struct {
	Data map[string]float64
}

// StoreV2 是 Snapshot 版本 2 的資料格式
// 儲存成 slice，Key 是字串，Value 是字串（需轉換成 float64）
type StoreV2 struct {
	Data []struct {
		Key   string
		Value string
	}
}

// SnapshotFile 用於封裝版本與資料本體
type SnapshotFile struct {
	SnapshotVersion uint64
	Data            any
}

// CurrencyStore 貨幣帳戶資料結構
type CurrencyStore struct {
	clusterID uint64
	nodeID    uint64
	store     sync.Map // key=currency string, value=*maps.SafeFloatMap
}

// NewCurrencyStore 建構並回傳 CurrencyStore 實例，預設版本 1
func NewCurrencyStore(
	clusterID uint64,
	nodeID uint64,
) *CurrencyStore {
	return &CurrencyStore{clusterID: clusterID, nodeID: nodeID}
}

// Update 更新指定 uid、貨幣的金額（可加減）
func (cs *CurrencyStore) Update(uid, currency string, amount float64) {
	val, loaded := cs.store.Load(currency)
	if !loaded {
		sfm := maps.NewSafeFloatMap()
		actual, loaded := cs.store.LoadOrStore(currency, sfm)
		if loaded {
			sfm = actual.(*maps.SafeFloatMap)
		}
		sfm.Add(uid, amount)
		return
	}
	sfm := val.(*maps.SafeFloatMap)
	sfm.Add(uid, amount)
}

// Get 取得指定 uid、貨幣的餘額，找不到回傳 0
func (cs *CurrencyStore) Get(uid, currency string) float64 {
	val, ok := cs.store.Load(currency)
	if !ok {
		return 0
	}
	sfm := val.(*maps.SafeFloatMap)
	return sfm.Get(uid)
}

// List 回傳所有帳戶與貨幣餘額快照
func (cs *CurrencyStore) List() map[string]map[string]float64 {
	result := make(map[string]map[string]float64)
	cs.store.Range(func(key, value any) bool {
		currency := key.(string)
		sfm := value.(*maps.SafeFloatMap)
		snapshot := sfm.Snapshot()
		for uid, balance := range snapshot {
			if result[uid] == nil {
				result[uid] = make(map[string]float64)
			}
			result[uid][currency] = balance
		}
		return true
	})
	return result
}

// SaveSnapshot 實作 Dragonboat Snapshot 介面，將資料依版本存成多個分檔
func (cs *CurrencyStore) SaveSnapshot(w io.Writer, fss statemachine.ISnapshotFileCollection, done <-chan struct{}) error {
	version := configs.GetSnapshotVersion(cs.clusterID, cs.nodeID)

	// 儲存元資料（版本號）
	meta := SnapshotFile{
		SnapshotVersion: version,
		Data:            nil,
	}
	if err := gob.NewEncoder(w).Encode(meta); err != nil {
		return err
	}

	var saveErr error
	var index uint64 = 0

	// 遍歷所有貨幣並分別序列化存檔
	cs.store.Range(func(key, value any) bool {
		select {
		case <-done:
			saveErr = errors.New("snapshot save stopped")
			return false
		default:
		}

		currency := key.(string)
		sfm := value.(*maps.SafeFloatMap)
		dataMap := sfm.Snapshot()

		buf := new(bytes.Buffer)
		var snapshot SnapshotFile

		// 根據版本組裝序列化物件
		switch version {
		case 1:
			snapshot = SnapshotFile{
				SnapshotVersion: 1,
				Data:            &StoreV1{Data: dataMap},
			}
		case 2:
			// 將 map 轉成 slice []{Key,Value string}
			dataSlice := make([]struct {
				Key   string
				Value string
			}, 0, len(dataMap))
			for k, v := range dataMap {
				dataSlice = append(dataSlice, struct {
					Key   string
					Value string
				}{
					Key:   k,
					Value: fmt.Sprintf("%f", v),
				})
			}
			snapshot = SnapshotFile{
				SnapshotVersion: 2,
				Data:            &StoreV2{Data: dataSlice},
			}
		default:
			saveErr = fmt.Errorf("unsupported snapshot version %d", version)
			return false
		}

		if err := gob.NewEncoder(buf).Encode(snapshot); err != nil {
			saveErr = err
			return false
		}

		compressed := snappy.Encode(nil, buf.Bytes())

		filename := fmt.Sprintf("currency_%s.snap", currency)
		fss.AddFile(index, filename, compressed)

		index++
		return true
	})

	return saveErr
}

// RecoverFromSnapshot 依版本還原 Snapshot
func (cs *CurrencyStore) RecoverFromSnapshot(r io.Reader, files []statemachine.SnapshotFile, done <-chan struct{}) error {
	// 先解 meta，取得版本號
	var meta SnapshotFile
	if err := gob.NewDecoder(r).Decode(&meta); err != nil {
		return err
	}

	for _, file := range files {
		select {
		case <-done:
			return errors.New("snapshot recover stopped")
		default:
		}

		filename := filepath.Base(file.Filepath)
		parts := strings.Split(filename, "_")
		if len(parts) != 2 || !strings.HasSuffix(parts[1], ".snap") {
			continue
		}
		currency := strings.TrimSuffix(parts[1], ".snap")

		raw, err := os.ReadFile(file.Filepath)
		if err != nil {
			return err
		}

		decompressed, err := snappy.Decode(nil, raw)
		if err != nil {
			return err
		}

		var snapshot SnapshotFile
		if err := gob.NewDecoder(bytes.NewReader(decompressed)).Decode(&snapshot); err != nil {
			return err
		}

		var merged map[string]float64
		switch snapshot.SnapshotVersion {
		case 1:
			dataV1, ok := snapshot.Data.(*StoreV1)
			if !ok {
				return errors.New("invalid snapshot data type for v1")
			}
			merged, _ = migrateFromV1(dataV1)
		case 2:
			dataV2, ok := snapshot.Data.(*StoreV2)
			if !ok {
				return errors.New("invalid snapshot data type for v2")
			}
			merged, err = migrateFromV2(dataV2)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported snapshot version %d", snapshot.SnapshotVersion)
		}

		for uid, val := range merged {
			cs.Update(uid, currency, val)
		}
	}

	return nil
}

// migrateFromV1 將 V1 版本資料轉成 map[string]float64
func migrateFromV1(oldData *StoreV1) (map[string]float64, error) {
	if oldData == nil {
		return map[string]float64{}, nil
	}
	result := make(map[string]float64)
	maps0.Copy(result, oldData.Data)
	return result, nil
}

// migrateFromV2 將 V2 版本資料轉成 map[string]float64
func migrateFromV2(oldData *StoreV2) (map[string]float64, error) {
	result := make(map[string]float64)
	if oldData == nil {
		return result, nil
	}
	for _, entry := range oldData.Data {
		val, err := strconv.ParseFloat(entry.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("parse float error for key %s: %w", entry.Key, err)
		}
		result[entry.Key] = val
	}
	return result, nil
}
