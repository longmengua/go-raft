package store

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"go-raft/pkg/maps"
	maps0 "maps"

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

var CurrentSnapshotVersion = 1

type SnapshotFile struct {
	SnapshotVersion int
	Data            any
}

type CurrencyStore struct {
	store   sync.Map
	sfGroup singleflight.Group
}

func NewCurrencyStore() *CurrencyStore {
	return &CurrencyStore{}
}

func (cs *CurrencyStore) Update(uid, currency string, amount float64) {
	val, loaded := cs.store.Load(currency)
	if !loaded {
		sfm := maps.NewSafeFloatMap()
		cs.store.Store(currency, sfm)
		sfm.Add(uid, amount)
		return
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

func (cs *CurrencyStore) SaveSnapshot(w io.Writer) error {
	data := cs.List()
	buf := new(bytes.Buffer)
	snapshot := SnapshotFile{
		SnapshotVersion: CurrentSnapshotVersion,
		Data:            &StoreV1{Data: flatten(data)},
	}
	if err := gob.NewEncoder(buf).Encode(snapshot); err != nil {
		return err
	}
	compressed := snappy.Encode(nil, buf.Bytes())
	_, err := w.Write(compressed)
	return err
}

func (cs *CurrencyStore) RecoverFromSnapshot(r io.Reader, stopc <-chan struct{}) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	select {
	case <-stopc:
		return errors.New("snapshot load stopped")
	default:
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
			return errors.New("invalid data type for version 1")
		}
		merged, _ = migrateFromV1(dataV1)
	case 2:
		dataV2, ok := snapshot.Data.(*StoreV2)
		if !ok {
			return errors.New("invalid data type for version 2")
		}
		merged = make(map[string]float64)
		for _, kv := range dataV2.Data {
			v, err := strconv.ParseFloat(kv.Value, 64)
			if err != nil {
				return err
			}
			merged[kv.Key] = v
		}
	default:
		return fmt.Errorf("unsupported snapshot version %d", snapshot.SnapshotVersion)
	}
	cs.store = sync.Map{}
	for k, v := range merged {
		parts := strings.Split(k, "::")
		if len(parts) != 2 {
			continue
		}
		uid, currency := parts[0], parts[1]
		cs.Update(uid, currency, v)
	}
	return nil
}

func flatten(data map[string]map[string]float64) map[string]float64 {
	result := make(map[string]float64)
	for uid, currencies := range data {
		for currency, value := range currencies {
			key := fmt.Sprintf("%s::%s", uid, currency)
			result[key] = value
		}
	}
	return result
}

func migrateFromV1(oldData *StoreV1) (map[string]float64, error) {
	if oldData == nil {
		return map[string]float64{}, nil
	}
	result := make(map[string]float64)
	maps0.Copy(result, oldData.Data)
	return result, nil
}
