package httpapi

import (
	"net/http"
	"soundledger/internal/application"
	"soundledger/internal/domain"
	"strconv"
	"time"
)

func (s *Server) UploadClip(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, r, domain.FieldError("multipart", "multipart 请求无效或超过大小限制"))
		return
	}
	file, header, err := r.FormFile("audio")
	if err != nil {
		writeError(w, r, domain.FieldError("audio", "必须提供 audio 文件字段"))
		return
	}
	defer file.Close()
	meta, err := commandMeta(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	recordedAt, err := time.Parse(time.RFC3339Nano, r.FormValue("recordedAt"))
	if err != nil {
		writeError(w, r, domain.FieldError("recordedAt", "录制时间必须使用 RFC3339 格式"))
		return
	}
	duration, err := strconv.ParseInt(r.FormValue("durationMillis"), 10, 64)
	if err != nil {
		writeError(w, r, domain.FieldError("durationMillis", "录音时长必须是整数毫秒"))
		return
	}
	mediaType := r.FormValue("mediaType")
	if mediaType == "" {
		mediaType = header.Header.Get("Content-Type")
	}
	result, err := s.app.UploadClip(application.UploadClipCommand{Meta: meta, BatchID: r.PathValue("batchID"), ClipID: r.FormValue("clipId"), MediaType: mediaType, RecorderCode: r.FormValue("recorderCode"), HabitatNote: r.FormValue("habitatNote"), RecordedAt: recordedAt, DurationMillis: duration, Content: file})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, result)
}

type annotationRequest struct {
	ID           string  `json:"id"`
	SpeciesCode  string  `json:"speciesCode"`
	StartMillis  int64   `json:"startMillis"`
	EndMillis    int64   `json:"endMillis"`
	Confidence   float64 `json:"confidence"`
	EvidenceNote string  `json:"evidenceNote"`
}

func (s *Server) SubmitAnnotation(w http.ResponseWriter, r *http.Request) {
	var input annotationRequest
	if err := decodeJSON(w, r, s.maxJSONBytes, &input); err != nil {
		writeError(w, r, err)
		return
	}
	meta, err := commandMeta(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.app.SubmitAnnotation(application.SubmitAnnotationCommand{Meta: meta, BatchID: r.PathValue("batchID"), ClipID: r.PathValue("clipID"), AnnotationID: input.ID, SpeciesCode: input.SpeciesCode, StartMillis: input.StartMillis, EndMillis: input.EndMillis, Confidence: input.Confidence, EvidenceNote: input.EvidenceNote})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, result)
}
