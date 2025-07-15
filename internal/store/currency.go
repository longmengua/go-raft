package store

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"go-raft/pkg/maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/snappy"
	"golang.org/x/sync/singleflight"
)

type StoreV1 struct {
	Data map[string]float64
}

type StoreV2 struct {
	Data []struct {
		key   string
		value string
	}
}

const currentSnapshotVersion = 2 // 每次資料結構變更時 +1
type SnapshotFile struct {
	Version int
	Data    any // DataV1, DataV2
}

type CurrencyStore struct {
	baseDir string
	store   sync.Map // key: currency string, value: *maps.ThreadSafeFloatMap
	sfGroup singleflight.Group
}

func NewCurrencyStore(baseDir string) *CurrencyStore {
	return &CurrencyStore{
		baseDir: baseDir,
	}
}

func (cs *CurrencyStore) Update(uid, currency string, amount float64) {
	val, loaded := cs.store.Load(currency)
	if !loaded {
		sfm := maps.NewSafeFloatMap()
		err := cs.LoadCurrency(currency)
		if err != nil {
			cs.store.Store(currency, sfm)
			sfm.Add(uid, amount)
			return
		}
		val, _ = cs.store.Load(currency)
	}
	sfm := val.(*maps.SafeFloatMap)
	sfm.Add(uid, amount)
}

func (cs *CurrencyStore) Get(uid, currency string) float64 {
	val, ok := cs.store.Load(currency)
	if !ok {
		return 0
	}
	sfm := val.(*maps.SafeFloatMap)
	return sfm.Get(uid)
}

func (cs *CurrencyStore) GetOrLoad(uid, currency string) (float64, error) {
	if _, ok := cs.store.Load(currency); !ok {
		if err := cs.LoadCurrency(currency); err != nil {
			return 0, err
		}
	}
	return cs.Get(uid, currency), nil
}

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

func (cs *CurrencyStore) SaveSnapshot() error {
	// 0755 是 linux 權限設置
	if err := os.MkdirAll(cs.baseDir, 0755); err != nil {
		return err
	}
	var err error
	cs.store.Range(func(key, value any) bool {
		currency := key.(string)
		sfm := value.(*maps.SafeFloatMap)
		data := sfm.Snapshot()
		path := filepath.Join(cs.baseDir, currency+".snapshot.gz")
		if e := saveCurrency(path, data); e != nil {
			err = e
			return false
		}
		return true
	})
	return err
}

func (cs *CurrencyStore) RecoverFromSnapshot() error {
	if err := os.MkdirAll(cs.baseDir, 0755); err != nil {
		return err
	}
	files, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".snapshot.gz") {
			continue
		}
		currency := strings.TrimSuffix(file.Name(), ".snapshot.gz")
		if err := cs.LoadCurrency(currency); err != nil {
			return err
		}
	}
	return nil
}

func (cs *CurrencyStore) LoadCurrency(currency string) error {
	path := filepath.Join(cs.baseDir, currency+".snapshot.gz")
	result, err, _ := cs.sfGroup.Do(currency, func() (any, error) {
		return loadCurrency(path)
	})
	if err != nil {
		return err
	}
	newData := result.(map[string]float64)
	val, loaded := cs.store.Load(currency)
	if loaded {
		sfm := val.(*maps.SafeFloatMap)
		sfm.LoadData(newData)
	} else {
		sfm := maps.NewSafeFloatMap()
		sfm.LoadData(newData)
		cs.store.Store(currency, sfm)
	}
	return nil
}

func saveCurrency(path string, data map[string]float64) error {
	buf := new(bytes.Buffer)
	snapshot := SnapshotFile{
		Version: currentSnapshotVersion,
		Data:    data,
	}
	if err := gob.NewEncoder(buf).Encode(snapshot); err != nil {
		return err
	}
	// 使用 snappy 進行壓縮
	compressed := snappy.Encode(nil, buf.Bytes())
	return os.WriteFile(path, compressed, 0644)
}

func loadCurrency(path string) (*StoreV2, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decompressed, err := snappy.Decode(nil, raw)
	if err != nil {
		return nil, err
	}
	var snapshot SnapshotFile
	if err := gob.NewDecoder(bytes.NewReader(decompressed)).Decode(&snapshot); err != nil {
		return nil, err
	}
	//  snapshot 版本兼容
	switch snapshot.Version {
	case 1:
		dataV1, ok := snapshot.Data.(*StoreV1)
		if !ok {
			return nil, fmt.Errorf("invalid data type for version 1 snapshot")
		}
		return migrateFromV1(dataV1)
	case 2:
		dataV2, ok := snapshot.Data.(*StoreV2)
		if !ok {
			return nil, fmt.Errorf("invalid data type for version 2 snapshot")
		}
		return dataV2, nil
	default:
		return nil, fmt.Errorf("unsupported snapshot version %d", snapshot.Version)
	}
}

// 實作 migration 邏輯 for v1 ➔ current，假設目前是v2
func migrateFromV1(oldData *StoreV1) (*StoreV2, error) {
	if oldData == nil {
		return &StoreV2{}, nil
	}
	var v2 StoreV2
	for k, v := range oldData.Data {
		v2.Data = append(v2.Data, struct {
			key   string
			value string
		}{
			key:   k,
			value: fmt.Sprintf("%f", v),
		})
	}
	return &v2, nil
}
