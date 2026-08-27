package stalepublishchainhead_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"soundledger/internal/application"
	"soundledger/internal/audit"
	"soundledger/internal/domain"
	"soundledger/internal/httpapi"
	"soundledger/internal/store"
	"testing"
	"time"
)

func TestConcurrentCommitDoesNotStalePublishChainHead(t *testing.T) {
	dataStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	frozen := frozenAggregate(t, "publish-a", now)
	if err = dataStore.Commit(store.CommitRequest{
		Aggregate: frozen, ExpectedVersion: 0, EventType: "batch.frozen",
		ActorID: "admin-a", RequestID: "setup-a", Payload: map[string]any{"manifestDigest": frozen.Manifest.Digest}, OccurredAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	idRequested := make(chan struct{})
	releaseID := make(chan struct{})
	app, err := application.New(application.Config{
		Store: dataStore,
		Clock: func() time.Time { return now.Add(time.Minute) },
		IDFactory: func() string {
			close(idRequested)
			<-releaseID
			return "certificate-a"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(app).Handler()
	publishDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/batches/publish-a/publish", nil)
		req.Header.Set("X-Actor-ID", "publisher-a")
		req.Header.Set("X-Role", "publisher")
		req.Header.Set("X-Request-ID", "publish-request-a")
		req.Header.Set("Idempotency-Key", "publish-key-a")
		req.Header.Set("X-Expected-Version", "1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		publishDone <- response
	}()

	<-idRequested
	intervening := draftAggregate("intervening-b", now.Add(30*time.Second))
	if err = dataStore.Commit(store.CommitRequest{
		Aggregate: intervening, ExpectedVersion: 0, EventType: "batch.created",
		ActorID: "admin-b", RequestID: "create-request-b", Payload: map[string]any{"title": "并发批次"}, OccurredAt: now.Add(30 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	close(releaseID)

	publishResponse := <-publishDone
	if publishResponse.Code != http.StatusOK {
		t.Fatalf("发布返回 %d: %s", publishResponse.Code, publishResponse.Body.String())
	}
	var published struct {
		Data application.PublishResult `json:"data"`
	}
	if err = json.NewDecoder(publishResponse.Body).Decode(&published); err != nil {
		t.Fatal(err)
	}

	eventsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/batches/publish-a/events", nil)
	eventsResponse := httptest.NewRecorder()
	handler.ServeHTTP(eventsResponse, eventsRequest)
	if eventsResponse.Code != http.StatusOK {
		t.Fatalf("事件查询返回 %d: %s", eventsResponse.Code, eventsResponse.Body.String())
	}
	var queried struct {
		Data []audit.EventRecord `json:"data"`
	}
	if err = json.NewDecoder(eventsResponse.Body).Decode(&queried); err != nil {
		t.Fatal(err)
	}
	if len(queried.Data) != 2 {
		t.Fatalf("发布批次事件数为 %d，期望 2", len(queried.Data))
	}
	publishEvent := queried.Data[1]
	if published.Data.Certificate.EventChainHead != publishEvent.PreviousHash {
		t.Fatalf("发布证书绑定旧链头 %q，发布事件实际承接 %q", published.Data.Certificate.EventChainHead, publishEvent.PreviousHash)
	}
}

func frozenAggregate(t *testing.T, id string, now time.Time) *domain.Aggregate {
	t.Helper()
	manifest := domain.Manifest{
		SchemaVersion: "soundledger-manifest/v1",
		BatchID:       id,
		BatchVersion:  1,
		GeneratedAt:   now,
	}
	digest, err := audit.Digest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Digest = digest
	return &domain.Aggregate{
		Batch: domain.SoundscapeBatch{
			ID: id, Title: "待发布批次", LicenseStatement: "CC-BY-4.0", RequiredAnnotators: 2,
			State: domain.BatchFrozen, Version: 1, CreatedAt: now, UpdatedAt: now, ManifestDigest: digest,
		},
		Manifest: &manifest,
	}
}

func draftAggregate(id string, now time.Time) *domain.Aggregate {
	return &domain.Aggregate{Batch: domain.SoundscapeBatch{
		ID: id, Title: "并发写入批次", LicenseStatement: "CC-BY-4.0", RequiredAnnotators: 2,
		State: domain.BatchDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
	}}
}
