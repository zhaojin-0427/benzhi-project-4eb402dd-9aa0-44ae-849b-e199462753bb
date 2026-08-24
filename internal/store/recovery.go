package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"soundledger/internal/audit"
	"soundledger/internal/domain"
	"strings"
)

func (s *FileStore) loadEvents() error {
	file, err := os.Open(s.eventPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	previous := ""
	sequence := int64(1)
	batchVersions := map[string]int64{}
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var event audit.EventRecord
		if err = json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("事件日志第 %d 行损坏: %w", sequence, err)
		}
		if err = audit.VerifyEvent(event, sequence, previous); err != nil {
			return fmt.Errorf("事件日志校验失败: %w", err)
		}
		raw, err := json.Marshal(event.AggregateSnapshot)
		if err != nil {
			return fmt.Errorf("事件快照无法读取: %w", err)
		}
		var aggregate domain.Aggregate
		if err = json.Unmarshal(raw, &aggregate); err != nil {
			return fmt.Errorf("事件快照无法恢复: %w", err)
		}
		if aggregate.Batch.ID != event.BatchID || aggregate.Batch.Version != event.BatchVersion {
			return fmt.Errorf("事件 %d 的聚合快照标识或版本不匹配", event.Sequence)
		}
		if previousVersion := batchVersions[event.BatchID]; event.BatchVersion != previousVersion+1 {
			return fmt.Errorf("批次 %s 的版本不连续：得到 %d，期望 %d", event.BatchID, event.BatchVersion, previousVersion+1)
		}
		batchVersions[event.BatchID] = event.BatchVersion
		s.batches[event.BatchID] = &aggregate
		s.events = append(s.events, event)
		previous = event.Hash
		sequence++
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	s.chainHead = previous
	return nil
}

func (s *FileStore) loadProjections() error {
	entries, err := os.ReadDir(s.projectionDir)
	if err != nil {
		return err
	}
	valid := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.projectionDir, entry.Name())
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		var aggregate domain.Aggregate
		if json.Unmarshal(data, &aggregate) != nil || aggregate.Batch.ID == "" {
			continue
		}
		canonical := s.batches[aggregate.Batch.ID]
		if canonical != nil && canonical.Batch.Version == aggregate.Batch.Version {
			valid[aggregate.Batch.ID] = true
		}
	}
	for id, aggregate := range s.batches {
		if !valid[id] {
			if err = s.writeProjection(aggregate); err != nil {
				return fmt.Errorf("重建批次 %s 投影失败: %w", id, err)
			}
		}
	}
	return nil
}

func (s *FileStore) loadIdempotency() error {
	data, err := os.ReadFile(s.idempotencyPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &s.idempotency); err != nil {
		return fmt.Errorf("幂等索引损坏: %w", err)
	}
	return nil
}
func (s *FileStore) projectionPath(id string) string {
	return filepath.Join(s.projectionDir, id+".json")
}
func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func filepathDir(path string) string { return filepath.Dir(path) }
