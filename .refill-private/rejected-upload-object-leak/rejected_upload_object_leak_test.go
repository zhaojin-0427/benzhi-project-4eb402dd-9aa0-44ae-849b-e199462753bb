package rejected_upload_object_leak_test

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"soundledger/internal/application"
	"soundledger/internal/domain"
	"soundledger/internal/store"
	"testing"
	"time"
)

func TestRejectedUploadDoesNotLeaveUnreferencedObject(t *testing.T) {
	root := t.TempDir()
	repository, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	service, err := application.New(application.Config{Store: repository, Clock: func() time.Time { return now }, MaxUploadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateBatch(application.CreateBatchCommand{Meta: application.CommandMeta{ActorID: "admin", Role: application.RoleAdministrator, IdempotencyKey: "create-key", RequestID: "request-create"}, ID: "batch-upload", Title: "上传测试", SiteBoundary: domain.SiteBoundary{North: 1, South: 0, East: 1, West: 0}, SampleWindow: domain.TimeWindow{Start: now, End: now.Add(time.Hour)}, LicenseStatement: "科研授权", RequiredAnnotators: 2})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.UploadClip(application.UploadClipCommand{Meta: application.CommandMeta{ActorID: "admin", Role: application.RoleAdministrator, ExpectedVersion: created.Version, IdempotencyKey: "upload-key", RequestID: "request-upload"}, BatchID: created.ID, ClipID: "clip-invalid", MediaType: "audio/wav", RecordedAt: now.Add(time.Minute), DurationMillis: 0, Content: bytes.NewReader([]byte("RIFF\x10\x00\x00\x00WAVEunique-rejected-audio"))})
	if err == nil {
		t.Fatal("时长无效的上传应被拒绝")
	}
	objectCount := 0
	err = filepath.WalkDir(filepath.Join(root, "objects"), func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() {
			objectCount++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if objectCount != 0 {
		t.Fatalf("被拒绝的上传遗留了 %d 个未引用对象", objectCount)
	}
}
