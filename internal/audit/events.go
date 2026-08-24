package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type EventRecord struct {
	SchemaVersion     string    `json:"schemaVersion"`
	Sequence          int64     `json:"sequence"`
	BatchID           string    `json:"batchId"`
	BatchVersion      int64     `json:"batchVersion"`
	Type              string    `json:"type"`
	ActorID           string    `json:"actorId"`
	RequestID         string    `json:"requestId"`
	OccurredAt        time.Time `json:"occurredAt"`
	Payload           any       `json:"payload"`
	AggregateSnapshot any       `json:"aggregateSnapshot,omitempty"`
	PreviousHash      string    `json:"previousHash"`
	Hash              string    `json:"hash"`
}

type hashMaterial struct {
	SchemaVersion     string    `json:"schemaVersion"`
	Sequence          int64     `json:"sequence"`
	BatchID           string    `json:"batchId"`
	BatchVersion      int64     `json:"batchVersion"`
	Type              string    `json:"type"`
	ActorID           string    `json:"actorId"`
	RequestID         string    `json:"requestId"`
	OccurredAt        time.Time `json:"occurredAt"`
	Payload           any       `json:"payload"`
	AggregateSnapshot any       `json:"aggregateSnapshot,omitempty"`
	PreviousHash      string    `json:"previousHash"`
}

func SealEvent(record EventRecord) (EventRecord, error) {
	record.SchemaVersion = "soundledger-event/v1"
	material := hashMaterial{record.SchemaVersion, record.Sequence, record.BatchID, record.BatchVersion, record.Type, record.ActorID, record.RequestID, record.OccurredAt, record.Payload, record.AggregateSnapshot, record.PreviousHash}
	b, err := CanonicalJSON(material)
	if err != nil {
		return EventRecord{}, err
	}
	sum := sha256.Sum256(b)
	record.Hash = hex.EncodeToString(sum[:])
	return record, nil
}

func VerifyEvent(record EventRecord, expectedSequence int64, previousHash string) error {
	if record.SchemaVersion != "soundledger-event/v1" {
		return fmt.Errorf("不支持的事件 schemaVersion %q", record.SchemaVersion)
	}
	if record.Sequence != expectedSequence {
		return fmt.Errorf("事件序号不连续：得到 %d，期望 %d", record.Sequence, expectedSequence)
	}
	if record.PreviousHash != previousHash {
		return fmt.Errorf("事件前序摘要不匹配")
	}
	want := record.Hash
	record.Hash = ""
	sealed, err := SealEvent(record)
	if err != nil {
		return err
	}
	if sealed.Hash != want {
		return fmt.Errorf("事件摘要校验失败")
	}
	return nil
}
