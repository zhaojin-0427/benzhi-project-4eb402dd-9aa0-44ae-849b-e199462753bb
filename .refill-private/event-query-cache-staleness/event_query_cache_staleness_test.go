package eventquerycachestaleness_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"soundledger/internal/application"
	"soundledger/internal/httpapi"
	"soundledger/internal/store"
	"testing"
	"time"
)

func TestEventQueryRefreshesAfterSuccessfulCommit(t *testing.T) {
	dataStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	clock := func() time.Time { return time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC) }
	service, err := application.New(application.Config{Store: dataStore, Clock: clock, IDFactory: func() string { return "fixed-id" }})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	handler := httpapi.New(service).Handler()

	createBody := bytes.NewBufferString(`{"id":"batch-cache","title":"缓存失效复现","siteBoundary":{"north":31,"south":30,"east":121,"west":120},"sampleWindow":{"start":"2026-08-25T07:00:00Z","end":"2026-08-25T09:00:00Z"},"licenseStatement":"科研授权","requiredAnnotators":2}`)
	create := httptest.NewRequest(http.MethodPost, "/api/v1/batches", createBody)
	create.Header.Set("Content-Type", "application/json")
	setCommandHeaders(create, "create-key", "0")
	createResult := httptest.NewRecorder()
	handler.ServeHTTP(createResult, create)
	if createResult.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResult.Code, createResult.Body.String())
	}

	if count := queryEventCount(t, handler); count != 1 {
		t.Fatalf("initial event count = %d, want 1", count)
	}

	var uploadBody bytes.Buffer
	writer := multipart.NewWriter(&uploadBody)
	fields := map[string]string{
		"clipId":         "clip-cache",
		"mediaType":      "audio/wav",
		"recordedAt":     "2026-08-25T08:00:00Z",
		"durationMillis": "1000",
		"recorderCode":   "R-01",
		"habitatNote":    "林缘",
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("write multipart field: %v", err)
		}
	}
	audio, err := writer.CreateFormFile("audio", "sample.wav")
	if err != nil {
		t.Fatalf("create audio part: %v", err)
	}
	if _, err = audio.Write(append([]byte("RIFF\x04\x00\x00\x00WAVE"), []byte("payload")...)); err != nil {
		t.Fatalf("write audio: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	upload := httptest.NewRequest(http.MethodPost, "/api/v1/batches/batch-cache/clips", &uploadBody)
	upload.Header.Set("Content-Type", writer.FormDataContentType())
	setCommandHeaders(upload, "upload-key", "1")
	uploadResult := httptest.NewRecorder()
	handler.ServeHTTP(uploadResult, upload)
	if uploadResult.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", uploadResult.Code, uploadResult.Body.String())
	}

	if count := queryEventCount(t, handler); count != 2 {
		t.Fatalf("event count after committed upload = %d, want 2", count)
	}
}

func setCommandHeaders(request *http.Request, key, version string) {
	request.Header.Set("X-Actor-ID", "admin-1")
	request.Header.Set("X-Role", "administrator")
	request.Header.Set("X-Request-ID", "request-"+key)
	request.Header.Set("Idempotency-Key", key)
	request.Header.Set("X-Expected-Version", version)
}

func queryEventCount(t *testing.T, handler http.Handler) int {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/batches/batch-cache/events", nil)
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusOK {
		t.Fatalf("events status = %d, body = %s", result.Code, result.Body.String())
	}
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	return len(response.Data)
}
