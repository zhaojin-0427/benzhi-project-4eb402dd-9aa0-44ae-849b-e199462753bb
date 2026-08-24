package application

import (
	"bytes"
	"errors"
	"soundledger/internal/domain"
	"soundledger/internal/store"
	"testing"
	"time"
)

func TestCreateIdempotencyAndExpectedVersion(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	idCounter := 0
	service, err := New(Config{Store: repository, Clock: func() time.Time { return now }, IDFactory: func() string { idCounter++; return "generated" }, MaxUploadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	meta := CommandMeta{ActorID: "admin", Role: RoleAdministrator, ExpectedVersion: 0, IdempotencyKey: "create-1", RequestID: "request-1"}
	command := CreateBatchCommand{Meta: meta, ID: "batch-1", Title: "测试批次", SiteBoundary: domain.SiteBoundary{North: 31, South: 30, East: 121, West: 120}, SampleWindow: domain.TimeWindow{Start: now, End: now.Add(24 * time.Hour)}, LicenseStatement: "科研授权", RequiredAnnotators: 2}
	first, err := service.CreateBatch(command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateBatch(command)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.Version != second.Version || first.State != second.State || len(repository.Events("batch-1")) != 1 {
		t.Fatal("幂等重试未返回首次响应")
	}
	_, err = service.UploadClip(UploadClipCommand{Meta: CommandMeta{ActorID: "admin", Role: RoleAdministrator, ExpectedVersion: 99, IdempotencyKey: "upload-1", RequestID: "request-2"}, BatchID: "batch-1", ClipID: "clip-1", MediaType: "audio/wav", RecordedAt: now.Add(time.Hour), DurationMillis: 1000, Content: bytes.NewReader([]byte("RIFF\x10\x00\x00\x00WAVEaudio"))})
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != domain.ErrConflict {
		t.Fatalf("期望版本冲突，得到 %v", err)
	}
}
