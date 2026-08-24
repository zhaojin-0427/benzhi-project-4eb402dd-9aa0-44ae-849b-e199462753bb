package httpapi

import (
	"net/http"
	"soundledger/internal/application"
	"soundledger/internal/domain"
)

type arbitrateRequest struct {
	FinalSpeciesCode  string `json:"finalSpeciesCode"`
	Rationale         string `json:"rationale"`
	ReturnForRevision bool   `json:"returnForRevision"`
}

func (s *Server) GetDisputeEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Actor-ID") == "" || application.Role(r.Header.Get("X-Role")) != application.RoleArbiter {
		writeError(w, r, domain.NewError(domain.ErrForbidden, "只有分类学仲裁专家可以查看争议证据"))
		return
	}
	result, err := s.app.GetDisputeEvidence(r.PathValue("batchID"), r.PathValue("disputeID"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) Arbitrate(w http.ResponseWriter, r *http.Request) {
	var input arbitrateRequest
	if err := decodeJSON(w, r, s.maxJSONBytes, &input); err != nil {
		writeError(w, r, err)
		return
	}
	meta, err := commandMeta(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.app.Arbitrate(application.ArbitrateCommand{Meta: meta, BatchID: r.PathValue("batchID"), DisputeID: r.PathValue("disputeID"), FinalSpeciesCode: input.FinalSpeciesCode, Rationale: input.Rationale, ReturnForRevision: input.ReturnForRevision})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
