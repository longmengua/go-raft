package storage

import (
	"bytes"
	"encoding/gob"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/snappy"
)

type CurrencyStore struct {
	mu      sync.RWMutex
	baseDir string
	store   map[string]map[string]float64 // currency -> uid -> balance
}

func NewCurrencyStore(baseDir string) *CurrencyStore {
	return &CurrencyStore{
		baseDir: baseDir,
		store:   make(map[string]map[string]float64),
	}
}

func (cs *CurrencyStore) Add(uid, currency string, amount float64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if _, ok := cs.store[currency]; !ok {
		cs.store[currency] = make(map[string]float64)
	}
	log.Printf("Adding %f to %s for user %s", amount, currency, uid)
	log.Printf("Current balance before addition: %f", cs.store[currency][uid])
	cs.store[currency][uid] += amount
}

func (cs *CurrencyStore) Get(uid, currency string) float64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.store[currency][uid]
}

func (cs *CurrencyStore) List() map[string]map[string]float64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	copy := make(map[string]map[string]float64)
	for currency, users := range cs.store {
		copy[currency] = make(map[string]float64)
		for uid, amount := range users {
			copy[currency][uid] = amount
		}
	}
	return copy
}

func (cs *CurrencyStore) Save() error {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// 建立資料夾（若不存在）
	if err := os.MkdirAll(cs.baseDir, 0755); err != nil {
		return err
	}

	for currency, data := range cs.store {
		path := filepath.Join(cs.baseDir, currency+".snapshot.gz")
		if err := saveCurrency(path, data); err != nil {
			return err
		}
	}
	return nil
}

func (cs *CurrencyStore) Load() error {
	if err := os.MkdirAll(cs.baseDir, 0755); err != nil {
		return err
	}

	files, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return err
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".snapshot.gz") {
			continue
		}
		currency := strings.TrimSuffix(file.Name(), ".snapshot.gz")
		data, err := loadCurrency(filepath.Join(cs.baseDir, file.Name()))
		if err != nil {
			return err
		}
		cs.store[currency] = data
	}
	return nil
}

func saveCurrency(path string, data map[string]float64) error {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(data); err != nil {
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
	var m map[string]float64
	if err := gob.NewDecoder(bytes.NewReader(decompressed)).Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("decoded data is nil")
	}
	return m, nil
}
