package httpapi

import (
	"net/http"
	"soundledger/internal/application"
	"soundledger/internal/domain"
)

type createBatchRequest struct {
	ID                 string              `json:"id"`
	Title              string              `json:"title"`
	SiteBoundary       domain.SiteBoundary `json:"siteBoundary"`
	SampleWindow       domain.TimeWindow   `json:"sampleWindow"`
	LicenseStatement   string              `json:"licenseStatement"`
	RequiredAnnotators int                 `json:"requiredAnnotators"`
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var input createBatchRequest
	if err := decodeJSON(w, r, s.maxJSONBytes, &input); err != nil {
		writeError(w, r, err)
		return
	}
	meta, err := commandMeta(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.app.CreateBatch(application.CreateBatchCommand{Meta: meta, ID: input.ID, Title: input.Title, SiteBoundary: input.SiteBoundary, SampleWindow: input.SampleWindow, LicenseStatement: input.LicenseStatement, RequiredAnnotators: input.RequiredAnnotators})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, result)
}
func (s *Server) ListBatches(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.app.ListBatches())
}
func (s *Server) GetBatch(w http.ResponseWriter, r *http.Request) {
	result, err := s.app.GetBatch(r.PathValue("batchID"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) ComputeConsensus(w http.ResponseWriter, r *http.Request) {
	meta, err := commandMeta(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.app.ComputeConsensus(application.ComputeConsensusCommand{Meta: meta, BatchID: r.PathValue("batchID")})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) Freeze(w http.ResponseWriter, r *http.Request) {
	meta, err := commandMeta(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.app.Freeze(application.FreezeCommand{Meta: meta, BatchID: r.PathValue("batchID")})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) Publish(w http.ResponseWriter, r *http.Request) {
	meta, err := commandMeta(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.app.Publish(application.PublishCommand{Meta: meta, BatchID: r.PathValue("batchID")})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) GetManifest(w http.ResponseWriter, r *http.Request) {
	result, err := s.app.Manifest(r.PathValue("batchID"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) GetCertificate(w http.ResponseWriter, r *http.Request) {
	result, err := s.app.Certificate(r.PathValue("batchID"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) GetEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.app.Events(r.PathValue("batchID")))
}
