package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"soundledger/internal/audit"
	"soundledger/internal/domain"
	"strings"
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
	chainHead                                                          string
}

func Open(root string) (*FileStore, error) {
	if root == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	s := &FileStore{root: root, objectDir: filepath.Join(root, "objects"), tmpDir: filepath.Join(root, "tmp"), projectionDir: filepath.Join(root, "projections"), eventPath: filepath.Join(root, "events.jsonl"), idempotencyPath: filepath.Join(root, "idempotency.json"), batches: map[string]*domain.Aggregate{}, idempotency: map[string]IdempotencyRecord{}}
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []audit.EventRecord{}
	for _, event := range s.events {
		if batchID == "" || event.BatchID == batchID {
			out = append(out, event)
		}
	}
	return out
}
func (s *FileStore) Idempotency(key string) (IdempotencyRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.idempotency[key]
	return record, ok
}

// ReleaseObject removes the content-addressed object identified by digest when
// no persisted aggregate references it. It is used to clean up objects written
// for uploads that fail before or during commit. The reference scan and removal
// happen under the store write lock so that a concurrent successful upload of
// the same digest cannot be deleted.
func (s *FileStore) ReleaseObject(digest string) {
	if digest == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objectReferencedLocked(digest) {
		return
	}
	clean := filepath.Clean(digest)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return
	}
	target := filepath.Join(s.objectDir, clean[:2], clean)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return
	}
	dir := filepath.Dir(target)
	_ = os.Remove(dir)
}

// objectReferencedLocked reports whether any batch aggregate currently holds a
// clip whose SHA-256 equals digest. Callers must hold s.mu (read or write).
func (s *FileStore) objectReferencedLocked(digest string) bool {
	for _, aggregate := range s.batches {
		for _, clip := range aggregate.Clips {
			if clip.SHA256 == digest {
				return true
			}
		}
	}
	return false
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
