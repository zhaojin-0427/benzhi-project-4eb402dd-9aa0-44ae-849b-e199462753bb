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

func cloneAggregate(source *domain.Aggregate) (*domain.Aggregate, error) {
	clone := *source
	clone.Batch.ValidationIssues = append([]string(nil), source.Batch.ValidationIssues...)
	clone.Clips = append([]domain.AudioClip(nil), source.Clips...)
	clone.Annotations = append([]domain.SpeciesAnnotation(nil), source.Annotations...)
	clone.Consensus = make([]domain.Consensus, len(source.Consensus))
	for i, c := range source.Consensus {
		clone.Consensus[i] = c
		clone.Consensus[i].SourceIDs = append([]string(nil), c.SourceIDs...)
	}
	clone.Disputes = make([]domain.AnnotationDispute, len(source.Disputes))
	for i, d := range source.Disputes {
		clone.Disputes[i] = d
		clone.Disputes[i].ReasonCodes = append([]string(nil), d.ReasonCodes...)
		clone.Disputes[i].AnnotationIDs = append([]string(nil), d.AnnotationIDs...)
		if d.ResolvedAt != nil {
			ts := *d.ResolvedAt
			clone.Disputes[i].ResolvedAt = &ts
		}
	}
	if source.Manifest != nil {
		manifest := *source.Manifest
		manifest.Clips = append([]domain.ManifestClip(nil), source.Manifest.Clips...)
		manifest.Conclusions = append([]domain.ManifestConclusion(nil), source.Manifest.Conclusions...)
		clone.Manifest = &manifest
	}
	if source.Certificate != nil {
		certificate := *source.Certificate
		clone.Certificate = &certificate
	}
	return &clone, nil
}
