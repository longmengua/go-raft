package store

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"go-raft/internal/configs"
	"go-raft/pkg/maps"
	maps0 "maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/snappy"
	"golang.org/x/sync/singleflight"
)

func init() {
	gob.Register(&StoreV1{})
	gob.Register(&StoreV2{})
	gob.Register(map[string]float64{})
}

type StoreV1 struct {
	Data map[string]float64
}

type StoreV2 struct {
	Data []struct {
		Key   string
		Value string
	}
}

const currentSnapshotVersion = configs.SnapshotVersion

type SnapshotFile struct {
	SnapshotVersion int
	Data            any
}

type CurrencyStore struct {
	baseDir string
	store   sync.Map
	sfGroup singleflight.Group
}

func NewCurrencyStore(baseDir string) *CurrencyStore {
	return &CurrencyStore{baseDir: baseDir}
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
	if err := os.MkdirAll(cs.baseDir, 0755); err != nil {
		return err
	}
	today := time.Now().Format("20060102")
	var err error
	cs.store.Range(func(key, value any) bool {
		currency := key.(string)
		sfm := value.(*maps.SafeFloatMap)
		data := sfm.Snapshot()
		path := filepath.Join(cs.baseDir, fmt.Sprintf("%s_%s.snapshot.gz", currency, today))
		if e := saveCurrency(path, data); e != nil {
			err = e
			return false
		}
		return true
	})
	return err
}

func (cs *CurrencyStore) CleanupOldSnapshots(retentionDays int) error {
	files, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".snapshot.gz") {
			continue
		}
		parts := strings.Split(file.Name(), "_")
		if len(parts) != 2 {
			continue
		}
		dateStr := strings.TrimSuffix(parts[1], ".snapshot.gz")
		t, err := time.Parse("20060102", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			os.Remove(filepath.Join(cs.baseDir, file.Name()))
		}
	}
	return nil
}

func (cs *CurrencyStore) LoadCurrency(currency string) error {
	files, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return err
	}
	latestDate := ""
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		prefix := currency + "_"
		if strings.HasPrefix(file.Name(), prefix) && strings.HasSuffix(file.Name(), ".snapshot.gz") {
			dateStr := strings.TrimSuffix(strings.TrimPrefix(file.Name(), prefix), ".snapshot.gz")
			if len(dateStr) == 8 && dateStr > latestDate {
				latestDate = dateStr
			}
		}
	}
	if latestDate == "" {
		return fmt.Errorf("no snapshot found for currency: %s", currency)
	}

	filename := fmt.Sprintf("%s_%s.snapshot.gz", currency, latestDate)
	path := filepath.Join(cs.baseDir, filename)

	result, err, _ := cs.sfGroup.Do(path, func() (any, error) {
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
	var snapshot SnapshotFile
	if currentSnapshotVersion == 1 {
		snapshot = SnapshotFile{
			SnapshotVersion: 1,
			Data:            &StoreV1{Data: data},
		}
	} else {
		var arr []struct {
			Key   string
			Value string
		}
		for k, v := range data {
			arr = append(arr, struct {
				Key   string
				Value string
			}{
				Key:   k,
				Value: fmt.Sprintf("%f", v),
			})
		}
		snapshot = SnapshotFile{
			SnapshotVersion: 2,
			Data:            &StoreV2{Data: arr},
		}
	}
	if err := gob.NewEncoder(buf).Encode(snapshot); err != nil {
		return err
	}
	compressed := snappy.Encode(nil, buf.Bytes())
	return os.WriteFile(path, compressed, 0644)
}

func loadCurrency(path string) (map[string]float64, error) {
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
	switch snapshot.SnapshotVersion {
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
		m := make(map[string]float64)
		for _, kv := range dataV2.Data {
			v, err := strconv.ParseFloat(kv.Value, 64)
			if err != nil {
				return nil, err
			}
			m[kv.Key] = v
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unsupported snapshot version %d", snapshot.SnapshotVersion)
	}
}

func migrateFromV1(oldData *StoreV1) (map[string]float64, error) {
	if oldData == nil {
		return map[string]float64{}, nil
	}
	result := make(map[string]float64)
	maps0.Copy(result, oldData.Data)
	return result, nil
}
