package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"soundledger/internal/audit"
	"soundledger/internal/domain"
	"sync"
)

type IdempotencyRecord struct {
	Key       string          `json:"key"`
	BatchID   string          `json:"batchId"`
	Operation string          `json:"operation"`
	Status    int             `json:"status"`
	Response  json.RawMessage `json:"response"`
}

type FileStore struct {
	mu                                                                 sync.RWMutex
	root, objectDir, tmpDir, projectionDir, eventPath, idempotencyPath string
	batches                                                            map[string]*domain.Aggregate
	idempotency                                                        map[string]IdempotencyRecord
	events                                                             []audit.EventRecord
	eventCache                                                         map[string][]audit.EventRecord
	chainHead                                                          string
}

func Open(root string) (*FileStore, error) {
	if root == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	s := &FileStore{root: root, objectDir: filepath.Join(root, "objects"), tmpDir: filepath.Join(root, "tmp"), projectionDir: filepath.Join(root, "projections"), eventPath: filepath.Join(root, "events.jsonl"), idempotencyPath: filepath.Join(root, "idempotency.json"), batches: map[string]*domain.Aggregate{}, idempotency: map[string]IdempotencyRecord{}, eventCache: map[string][]audit.EventRecord{}}
	for _, dir := range []string{s.root, s.objectDir, s.tmpDir, s.projectionDir} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, err
		}
	}
	if err := s.loadEvents(); err != nil {
		return nil, err
	}
	if err := s.loadIdempotency(); err != nil {
		return nil, err
	}
	if err := s.loadProjections(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) Get(batchID string) (*domain.Aggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.batches[batchID]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneAggregate(value)
}

func (s *FileStore) List() []domain.SoundscapeBatch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.SoundscapeBatch, 0, len(s.batches))
	for _, a := range s.batches {
		out = append(out, a.Batch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
func (s *FileStore) ChainHead() string { s.mu.RLock(); defer s.mu.RUnlock(); return s.chainHead }
func (s *FileStore) Events(batchID string) []audit.EventRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.eventCache[batchID]; ok {
		return append([]audit.EventRecord(nil), cached...)
	}
	out := []audit.EventRecord{}
	for _, event := range s.events {
		if batchID == "" || event.BatchID == batchID {
			out = append(out, event)
		}
	}
	s.eventCache[batchID] = append([]audit.EventRecord(nil), out...)
	return append([]audit.EventRecord(nil), out...)
}
func (s *FileStore) Idempotency(key string) (IdempotencyRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.idempotency[key]
	return record, ok
}

func cloneAggregate(source *domain.Aggregate) (*domain.Aggregate, error) {
	b, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	var copy domain.Aggregate
	if err = json.Unmarshal(b, &copy); err != nil {
		return nil, err
	}
	return &copy, nil
}
