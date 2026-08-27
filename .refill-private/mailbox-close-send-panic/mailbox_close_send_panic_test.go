package mailbox_close_send_panic_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"soundledger/internal/application"
	"soundledger/internal/domain"
	"soundledger/internal/httpapi"
	"soundledger/internal/store"
	"testing"
	"time"
)

func TestClosedServiceRejectsLateCommandWithoutPanic(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service, err := application.New(application.Config{
		Store:     repository,
		Clock:     func() time.Time { return now },
		IDFactory: func() string { return "generated-id" },
	})
	if err != nil {
		t.Fatal(err)
	}
	command := application.CreateBatchCommand{
		Meta: application.CommandMeta{
			ActorID:         "admin",
			Role:            application.RoleAdministrator,
			ExpectedVersion: 0,
			IdempotencyKey:  "create-before-close",
			RequestID:       "request-before-close",
		},
		ID:                 "lifecycle-batch",
		Title:              "邮箱生命周期复现批次",
		SiteBoundary:       domain.SiteBoundary{North: 31, South: 30, East: 121, West: 120},
		SampleWindow:       domain.TimeWindow{Start: now, End: now.Add(time.Hour)},
		LicenseStatement:   "科研授权",
		RequiredAnnotators: 2,
	}
	if _, err = service.CreateBatch(command); err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(service).Handler()
	service.Close()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("关闭后的命令不应崩溃: %v", recovered)
		}
	}()
	payload, err := json.Marshal(map[string]any{
		"id":                 "lifecycle-batch",
		"title":              "关闭后到达的请求",
		"siteBoundary":       domain.SiteBoundary{North: 31, South: 30, East: 121, West: 120},
		"sampleWindow":       domain.TimeWindow{Start: now, End: now.Add(time.Hour)},
		"licenseStatement":   "科研授权",
		"requiredAnnotators": 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor-ID", "admin")
	request.Header.Set("X-Role", "administrator")
	request.Header.Set("X-Expected-Version", "0")
	request.Header.Set("Idempotency-Key", "create-after-close")
	request.Header.Set("X-Request-ID", "request-after-close")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("关闭后的命令应返回 HTTP 503，得到 %d", response.Code)
	}
}
