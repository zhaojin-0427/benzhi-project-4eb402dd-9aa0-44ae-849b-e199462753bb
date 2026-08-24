package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"soundledger/internal/application"
	"soundledger/internal/domain"
	"strconv"
	"strings"
)

type responseEnvelope struct {
	Data any `json:"data"`
}
type errorEnvelope struct {
	Error errorBody `json:"error"`
}
type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Field     string `json:"field,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

type requestError struct{ message string }

func (e *requestError) Error() string { return e.message }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(responseEnvelope{Data: value})
}
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	body := errorBody{Code: "INTERNAL_ERROR", Message: "服务处理请求失败", RequestID: r.Header.Get("X-Request-ID")}
	var de *domain.DomainError
	if errors.As(err, &de) {
		body.Code = string(de.Code)
		body.Message = de.Message
		body.Field = de.Field
		switch de.Code {
		case domain.ErrValidation:
			status = 422
		case domain.ErrNotFound:
			status = 404
		case domain.ErrConflict, domain.ErrDuplicate, domain.ErrInvalidState, domain.ErrIncomplete, domain.ErrAlreadyPublished:
			status = 409
		case domain.ErrForbidden:
			status = 403
		}
	}
	var badRequest *requestError
	if errors.As(err, &badRequest) {
		status = 400
		body.Code = "INVALID_JSON"
		body.Message = badRequest.message
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: body})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, max int64, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, max)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &requestError{message: "JSON 请求体无效：" + err.Error()}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return &requestError{message: "请求体只能包含一个 JSON 值"}
		}
		return &requestError{message: "JSON 请求体无效：" + err.Error()}
	}
	return nil
}
func commandMeta(r *http.Request) (application.CommandMeta, error) {
	actor := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	role := application.Role(strings.TrimSpace(r.Header.Get("X-Role")))
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	versionText := strings.TrimSpace(r.Header.Get("X-Expected-Version"))
	if versionText == "" {
		versionText = "0"
	}
	version, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil || version < 0 {
		return application.CommandMeta{}, domain.FieldError("X-Expected-Version", "版本头必须是非负整数")
	}
	return application.CommandMeta{Context: r.Context(), ActorID: actor, Role: role, RequestID: requestID, IdempotencyKey: key, ExpectedVersion: version}, nil
}
