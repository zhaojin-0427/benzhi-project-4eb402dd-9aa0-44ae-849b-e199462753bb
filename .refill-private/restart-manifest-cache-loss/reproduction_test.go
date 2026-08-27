package restartmanifestcacheloss_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"soundledger/internal/application"
	"soundledger/internal/domain"
	"soundledger/internal/httpapi"
	"soundledger/internal/store"
	"testing"
	"time"
)

func TestPublishAfterRestartUsesPersistedManifest(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	repository, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	object, err := repository.PutObject(bytes.NewReader([]byte("RIFF\x10\x00\x00\x00WAVEaudio")), 1024)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := readyAggregate(now, object)
	if err = repository.Commit(store.CommitRequest{
		Aggregate: aggregate, ExpectedVersion: 0, EventType: "batch.ready",
		ActorID: "setup", RequestID: "setup-ready", Payload: map[string]any{"ready": true}, OccurredAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	service, err := application.New(application.Config{
		Store:     repository,
		Clock:     func() time.Time { return now.Add(time.Minute) },
		IDFactory: func() string { return "certificate-1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.Freeze(application.FreezeCommand{
		BatchID: "batch-restart",
		Meta: application.CommandMeta{
			ActorID: "admin-1", Role: application.RoleAdministrator, ExpectedVersion: 1,
			IdempotencyKey: "freeze-1", RequestID: "request-freeze",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Batch.State != domain.BatchFrozen || frozen.Manifest == nil {
		t.Fatalf("冻结结果不完整: state=%s manifest=%v", frozen.Batch.State, frozen.Manifest)
	}

	restartedStore, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	restartedService, err := application.New(application.Config{
		Store:     restartedStore,
		Clock:     func() time.Time { return now.Add(2 * time.Minute) },
		IDFactory: func() string { return "certificate-2" },
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches/batch-restart/publish", nil)
	request.Header.Set("X-Actor-ID", "publisher-1")
	request.Header.Set("X-Role", string(application.RolePublisher))
	request.Header.Set("X-Expected-Version", "2")
	request.Header.Set("X-Request-ID", "request-publish")
	request.Header.Set("Idempotency-Key", "publish-after-restart")
	response := httptest.NewRecorder()
	httpapi.New(restartedService).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("重启后持久化清单仍应允许发布，得到 status=%d body=%s", response.Code, response.Body.String())
	}
}

func readyAggregate(now time.Time, object store.StoredObject) *domain.Aggregate {
	clip := domain.AudioClip{
		ID: "clip-1", BatchID: "batch-restart", SHA256: object.SHA256, ObjectPath: object.Path,
		MediaType: "audio/wav", ByteSize: object.Size, RecordedAt: now, DurationMillis: 1000,
	}
	return &domain.Aggregate{
		Batch: domain.SoundscapeBatch{
			ID: "batch-restart", Title: "重启发布复现", LicenseStatement: "科研授权",
			RequiredAnnotators: 2, State: domain.BatchReadyValidate, Version: 1,
			CreatedAt: now, UpdatedAt: now,
		},
		Clips: []domain.AudioClip{clip},
		Annotations: []domain.SpeciesAnnotation{
			{ID: "annotation-1", ClipID: clip.ID, AnnotatorID: "annotator-1"},
			{ID: "annotation-2", ClipID: clip.ID, AnnotatorID: "annotator-2"},
		},
		Consensus: []domain.Consensus{{
			ClipID: clip.ID, SpeciesCode: "species-a", StartMillis: 10, EndMillis: 900,
			SourceIDs: []string{"annotation-1", "annotation-2"},
		}},
	}
}
