package store

import (
	"bytes"
	"encoding/gob"
	"errors"
	"go-raft/pkg/maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/snappy"
	"golang.org/x/sync/singleflight"
)

type Currency struct {
	baseDir string
	store   sync.Map // key: currency string, value: *maps.ThreadSafeFloatMap
	sfGroup singleflight.Group
}

func NewCurrencyStore(baseDir string) *Currency {
	return &Currency{
		baseDir: baseDir,
	}
}

// Add 增加使用者資產
func (cs *Currency) Add(uid, currency string, amount float64) {
	val, loaded := cs.store.Load(currency)
	if !loaded {
		// 初始化 maps.ThreadSafeFloatMap
		sfm := maps.NewSafeFloatMap()
		// 使用 singleflight 確保只載入一次貨幣檔案
		err := cs.LoadCurrency(currency)
		if err != nil {
			// 若 LoadCurrency 失敗，還是要新增一個空 maps.ThreadSafeFloatMap
			cs.store.Store(currency, sfm)
			sfm.Add(uid, amount)
			return
		}
		// 重新讀取
		val, _ = cs.store.Load(currency)
	}

	sfm := val.(*maps.SafeFloatMap)
	sfm.Add(uid, amount)
}

// Get 讀取指定 user 的指定 currency 餘額，沒有鎖，讀取快
func (cs *Currency) Get(uid, currency string) float64 {
	val, ok := cs.store.Load(currency)
	if !ok {
		return 0
	}
	sfm := val.(*maps.SafeFloatMap)
	return sfm.Get(uid)
}

// GetOrLoad 讀取，若尚未載入則自動載入
func (cs *Currency) GetOrLoad(uid, currency string) (float64, error) {
	if _, ok := cs.store.Load(currency); !ok {
		if err := cs.LoadCurrency(currency); err != nil {
			return 0, err
		}
	}
	return cs.Get(uid, currency), nil
}

// List 回傳全部資料快照：map[uid]map[currency]balance
func (cs *Currency) List() map[string]map[string]float64 {
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

// Save 儲存全部貨幣資料
func (cs *Currency) Save() error {
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

// Load 載入全部貨幣 snapshot
func (cs *Currency) Load() error {
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

// LoadCurrency 單獨載入某貨幣檔案，使用 singleflight 避免重複讀取
func (cs *Currency) LoadCurrency(currency string) error {
	path := filepath.Join(cs.baseDir, currency+".snapshot.gz")

	result, err, _ := cs.sfGroup.Do(currency, func() (interface{}, error) {
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

// saveCurrency 序列化 + snappy 壓縮存檔
func saveCurrency(path string, data map[string]float64) error {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(data); err != nil {
		return err
	}
	compressed := snappy.Encode(nil, buf.Bytes())
	return os.WriteFile(path, compressed, 0644)
}

// loadCurrency snappy 解壓 + 反序列化
func loadCurrency(path string) (map[string]float64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decompressed, err := snappy.Decode(nil, raw)
	if err != nil {
		return nil, err
	}
	var m map[string]float64
	if err := gob.NewDecoder(bytes.NewReader(decompressed)).Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("decoded data is nil")
	}
	return m, nil
}
