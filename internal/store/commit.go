package store

import (
	"encoding/json"
	"fmt"
	"os"
	"soundledger/internal/audit"
	"soundledger/internal/domain"
	"time"
)

type CommitRequest struct {
	Aggregate                     *domain.Aggregate
	ExpectedVersion               int64
	EventType, ActorID, RequestID string
	Payload                       any
	IdempotencyKey, Operation     string
	Response                      any
	Status                        int
	OccurredAt                    time.Time
}

func (s *FileStore) Commit(request CommitRequest) error {
	if request.Aggregate == nil {
		return fmt.Errorf("aggregate 不能为空")
	}
	var responseBody []byte
	if request.IdempotencyKey != "" {
		body, err := json.Marshal(request.Response)
		if err != nil {
			return err
		}
		responseBody = body
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.IdempotencyKey != "" {
		existing, ok := s.idempotency[request.IdempotencyKey]
		if ok && (existing.BatchID != request.Aggregate.Batch.ID || existing.Operation != request.Operation) {
			return ErrIdempotencyConflict
		}
	}
	current, exists := s.batches[request.Aggregate.Batch.ID]
	if !exists {
		if request.ExpectedVersion != 0 {
			return ErrVersionConflict
		}
	} else if current.Batch.Version != request.ExpectedVersion {
		return ErrVersionConflict
	}
	if request.Aggregate.Batch.Version <= request.ExpectedVersion {
		return fmt.Errorf("提交版本必须递增")
	}
	snapshot, err := cloneAggregate(request.Aggregate)
	if err != nil {
		return err
	}
	record := audit.EventRecord{Sequence: int64(len(s.events) + 1), BatchID: snapshot.Batch.ID, BatchVersion: snapshot.Batch.Version, Type: request.EventType, ActorID: request.ActorID, RequestID: request.RequestID, OccurredAt: request.OccurredAt.UTC(), Payload: request.Payload, AggregateSnapshot: snapshot, PreviousHash: s.chainHead}
	record, err = audit.SealEvent(record)
	if err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	log, err := os.OpenFile(s.eventPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return err
	}
	if _, err = log.Write(append(line, '\n')); err == nil {
		err = log.Sync()
	}
	closeErr := log.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = s.writeProjection(snapshot); err != nil {
		return err
	}
	s.events = append(s.events, record)
	s.chainHead = record.Hash
	s.batches[snapshot.Batch.ID] = snapshot
	if request.IdempotencyKey != "" {
		rec := IdempotencyRecord{Key: request.IdempotencyKey, BatchID: snapshot.Batch.ID, Operation: request.Operation, Status: request.Status, Response: responseBody}
		s.idempotency[request.IdempotencyKey] = rec
		if err = s.writeIdempotency(); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) writeProjection(a *domain.Aggregate) error {
	return writeJSONAtomic(s.projectionPath(a.Batch.ID), a)
}
func (s *FileStore) writeIdempotency() error {
	return writeJSONAtomic(s.idempotencyPath, s.idempotency)
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	dir := filepathDir(path)
	temp, err := os.CreateTemp(dir, "projection-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	renamed := false
	defer func() {
		temp.Close()
		if !renamed {
			_ = os.Remove(name)
		}
	}()
	if _, err = temp.Write(data); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	renamed = true
	return syncDirectory(dir)
}
