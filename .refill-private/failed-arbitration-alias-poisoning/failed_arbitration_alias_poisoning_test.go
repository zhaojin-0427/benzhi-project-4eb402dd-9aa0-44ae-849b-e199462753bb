package failed_arbitration_alias_poisoning_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"soundledger/internal/application"
	"soundledger/internal/domain"
	"soundledger/internal/httpapi"
	"soundledger/internal/store"
	"testing"
	"time"
)

func TestFailedArbitrationDoesNotPoisonStoredDispute(t *testing.T) {
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	root := t.TempDir()
	repository, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := &domain.Aggregate{
		Batch: domain.SoundscapeBatch{
			ID: "batch-alias", Title: "仲裁回滚测试", RequiredAnnotators: 2,
			State: domain.BatchAwaitArbitration, Version: 1, CreatedAt: now, UpdatedAt: now,
		},
		Clips: []domain.AudioClip{{
			ID: "clip-alias", BatchID: "batch-alias", DurationMillis: 1000,
		}},
		Annotations: []domain.SpeciesAnnotation{
			{ID: "annotation-a", ClipID: "clip-alias", AnnotatorID: "annotator-a", SpeciesCode: "species-a"},
			{ID: "annotation-b", ClipID: "clip-alias", AnnotatorID: "annotator-b", SpeciesCode: "species-b"},
		},
		Disputes: []domain.AnnotationDispute{{
			ID: "dispute-alias", BatchID: "batch-alias", ClipID: "clip-alias",
			AnnotationIDs: []string{"annotation-a", "annotation-b"}, State: domain.DisputeOpen,
		}},
	}
	if err = repository.Commit(store.CommitRequest{
		Aggregate: aggregate, ExpectedVersion: 0, EventType: "consensus.computed",
		ActorID: "admin", RequestID: "request-seed", OccurredAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service, err := application.New(application.Config{
		Store: repository,
		Clock: func() time.Time { return now.Add(time.Minute) },
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(service).Handler()

	eventPath := filepath.Join(root, "events.jsonl")
	if err = os.Remove(eventPath); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(eventPath, 0700); err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"finalSpeciesCode":"species-a","rationale":"专家复核","returnForRevision":false}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches/batch-alias/disputes/dispute-alias/arbitrate", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor-ID", "arbiter-a")
	request.Header.Set("X-Role", "arbiter")
	request.Header.Set("X-Request-ID", "request-arbitrate")
	request.Header.Set("Idempotency-Key", "arbitrate-alias")
	request.Header.Set("X-Expected-Version", "1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("事件日志失效时仲裁应返回 500，得到 %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/batches/batch-alias/disputes/dispute-alias", nil)
	request.Header.Set("X-Actor-ID", "arbiter-a")
	request.Header.Set("X-Role", "arbiter")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("查询争议失败: %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Data struct {
			Dispute domain.AnnotationDispute `json:"dispute"`
		} `json:"data"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Dispute.State != domain.DisputeOpen {
		t.Fatalf("失败仲裁污染了存储状态: 得到 %s，期望 %s", result.Data.Dispute.State, domain.DisputeOpen)
	}
}
