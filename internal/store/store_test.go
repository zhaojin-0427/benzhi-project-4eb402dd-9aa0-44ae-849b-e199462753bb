package store

import (
	"os"
	"path/filepath"
	"soundledger/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestObjectAddressingAndRecovery(t *testing.T) {
	root := t.TempDir()
	repository, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	object, err := repository.PutObject(strings.NewReader("RIFF....WAVEaudio"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	again, err := repository.PutObject(strings.NewReader("RIFF....WAVEaudio"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if object.SHA256 != again.SHA256 || object.Path != again.Path {
		t.Fatal("相同内容未复用寻址对象")
	}
	clip := domain.AudioClip{ID: "clip-verify", SHA256: object.SHA256, ObjectPath: object.Path, ByteSize: object.Size}
	if issues := repository.VerifyClips([]domain.AudioClip{clip}); len(issues) != 0 {
		t.Fatalf("完整对象校验失败: %v", issues)
	}
	objectPath := filepath.Join(root, filepath.FromSlash(object.Path))
	if err = os.WriteFile(objectPath, []byte(strings.Repeat("x", int(object.Size))), 0600); err != nil {
		t.Fatal(err)
	}
	if issues := repository.VerifyClips([]domain.AudioClip{clip}); len(issues) != 1 {
		t.Fatalf("对象篡改应被识别: %v", issues)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	aggregate, err := domain.NewBatch("b1", "测试", domain.SiteBoundary{North: 1, South: 0, East: 1, West: 0}, domain.TimeWindow{Start: now, End: now.Add(time.Hour)}, "授权", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.Commit(CommitRequest{Aggregate: aggregate, ExpectedVersion: 0, EventType: "batch.created", ActorID: "u", RequestID: "r", Payload: map[string]any{"created": true}, OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "projections", "b1.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get("b1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Batch.ID != "b1" {
		t.Fatal("事件恢复投影失败")
	}
}

func TestTamperedEventLogFailsOpen(t *testing.T) {
	root := t.TempDir()
	repository, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	aggregate, err := domain.NewBatch("b1", "测试", domain.SiteBoundary{North: 1, South: 0, East: 1, West: 0}, domain.TimeWindow{Start: now, End: now.Add(time.Hour)}, "授权", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.Commit(CommitRequest{Aggregate: aggregate, ExpectedVersion: 0, EventType: "batch.created", ActorID: "u", RequestID: "r", OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "batch.created", "batch.changed", 1))
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(root); err == nil {
		t.Fatal("篡改日志后启动应失败")
	}
}
