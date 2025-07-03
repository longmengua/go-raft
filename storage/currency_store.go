package storage

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
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
	cs.store[currency][uid] += amount
}

func (cs *CurrencyStore) Get(uid, currency string) float64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.store[currency][uid]
}

func (cs *CurrencyStore) Save() error {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for currency, data := range cs.store {
		if err := saveCurrency(filepath.Join(cs.baseDir, currency+".snapshot.gz"), data); err != nil {
			return err
		}
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

func (cs *CurrencyStore) Load() error {
	files, err := os.ReadDir(cs.baseDir)
	if err != nil {
		return err
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".gz" {
			continue
		}
		currency := file.Name()[:len(file.Name())-len(".snapshot.gz")]
		data, err := loadCurrency(filepath.Join(cs.baseDir, file.Name()))
		if err != nil {
			return err
		}
		cs.store[currency] = data
	}
	return nil
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
	return m, nil
}
