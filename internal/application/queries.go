package application

import (
	"soundledger/internal/audit"
	"soundledger/internal/domain"
)

type BatchView struct {
	Batch           domain.SoundscapeBatch     `json:"batch"`
	Clips           []domain.AudioClip         `json:"clips"`
	AnnotationCount int                        `json:"annotationCount"`
	Consensus       []domain.Consensus         `json:"consensus,omitempty"`
	Disputes        []domain.AnnotationDispute `json:"disputes,omitempty"`
}
type AnonymousAnnotation struct {
	Alias        string  `json:"alias"`
	SpeciesCode  string  `json:"speciesCode"`
	StartMillis  int64   `json:"startMillis"`
	EndMillis    int64   `json:"endMillis"`
	Confidence   float64 `json:"confidence"`
	EvidenceNote string  `json:"evidenceNote"`
}
type DisputeEvidence struct {
	Dispute  domain.AnnotationDispute `json:"dispute"`
	Evidence []AnonymousAnnotation    `json:"evidence"`
}

func (s *Service) GetBatch(batchID string) (BatchView, error) {
	a, err := s.store.Get(batchID)
	if err != nil {
		return BatchView{}, mapStoreError(err)
	}
	return BatchView{Batch: a.Batch, Clips: a.Clips, AnnotationCount: len(a.Annotations), Consensus: a.Consensus, Disputes: a.Disputes}, nil
}
func (s *Service) ListBatches() []domain.SoundscapeBatch { return s.store.List() }
func (s *Service) Manifest(batchID string) (domain.Manifest, error) {
	a, err := s.store.Get(batchID)
	if err != nil {
		return domain.Manifest{}, mapStoreError(err)
	}
	if a.Manifest == nil {
		return domain.Manifest{}, domain.NewError(domain.ErrNotFound, "冻结清单不存在")
	}
	return *a.Manifest, nil
}
func (s *Service) Certificate(batchID string) (domain.ReleaseCertificate, error) {
	a, err := s.store.Get(batchID)
	if err != nil {
		return domain.ReleaseCertificate{}, mapStoreError(err)
	}
	if a.Certificate == nil {
		return domain.ReleaseCertificate{}, domain.NewError(domain.ErrNotFound, "发布证书不存在")
	}
	if err = audit.VerifyCertificate(*a.Certificate, *a.Manifest, a.Certificate.EventChainHead); err != nil {
		return domain.ReleaseCertificate{}, err
	}
	return *a.Certificate, nil
}
func (s *Service) Events(batchID string) []audit.EventRecord {
	events := s.store.Events(batchID)
	a, err := s.store.Get(batchID)
	if err == nil && a.Batch.State != domain.BatchPublished {
		for i := range events {
			events[i].AggregateSnapshot = nil
		}
	}
	return events
}
func (s *Service) GetDisputeEvidence(batchID, disputeID string) (DisputeEvidence, error) {
	a, err := s.store.Get(batchID)
	if err != nil {
		return DisputeEvidence{}, mapStoreError(err)
	}
	var dispute *domain.AnnotationDispute
	for i := range a.Disputes {
		if a.Disputes[i].ID == disputeID {
			dispute = &a.Disputes[i]
			break
		}
	}
	if dispute == nil {
		return DisputeEvidence{}, domain.NewError(domain.ErrNotFound, "争议不存在")
	}
	result := DisputeEvidence{Dispute: *dispute}
	aliases := []string{"annotator-A", "annotator-B"}
	for i, id := range dispute.AnnotationIDs {
		for _, ann := range a.Annotations {
			if ann.ID == id {
				alias := "annotator"
				if i < len(aliases) {
					alias = aliases[i]
				}
				result.Evidence = append(result.Evidence, AnonymousAnnotation{Alias: alias, SpeciesCode: ann.SpeciesCode, StartMillis: ann.StartMillis, EndMillis: ann.EndMillis, Confidence: ann.Confidence, EvidenceNote: ann.EvidenceNote})
			}
		}
	}
	return result, nil
}
