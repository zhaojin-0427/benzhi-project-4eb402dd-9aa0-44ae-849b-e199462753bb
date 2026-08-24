package application

import (
	"io"
	"soundledger/internal/domain"
	"time"
)

type Role string

const (
	RoleAdministrator Role = "administrator"
	RoleAnnotator     Role = "annotator"
	RoleArbiter       Role = "arbiter"
	RolePublisher     Role = "publisher"
)

type CommandMeta struct {
	ActorID         string `json:"actorId"`
	Role            Role   `json:"role"`
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	RequestID       string `json:"requestId"`
}

type CreateBatchCommand struct {
	Meta                        CommandMeta
	ID, Title, LicenseStatement string
	SiteBoundary                domain.SiteBoundary
	SampleWindow                domain.TimeWindow
	RequiredAnnotators          int
}
type UploadClipCommand struct {
	Meta                                                  CommandMeta
	BatchID, ClipID, MediaType, RecorderCode, HabitatNote string
	RecordedAt                                            time.Time
	DurationMillis                                        int64
	Content                                               io.Reader
}
type SubmitAnnotationCommand struct {
	Meta                                                     CommandMeta
	BatchID, ClipID, AnnotationID, SpeciesCode, EvidenceNote string
	StartMillis, EndMillis                                   int64
	Confidence                                               float64
}
type ComputeConsensusCommand struct {
	Meta    CommandMeta
	BatchID string
}
type ArbitrateCommand struct {
	Meta                                            CommandMeta
	BatchID, DisputeID, FinalSpeciesCode, Rationale string
	ReturnForRevision                               bool
}
type FreezeCommand struct {
	Meta    CommandMeta
	BatchID string
}
type PublishCommand struct {
	Meta    CommandMeta
	BatchID string
}

type UploadResult struct {
	Batch domain.SoundscapeBatch `json:"batch"`
	Clip  domain.AudioClip       `json:"clip"`
}
type AnnotationResult struct {
	Batch      domain.SoundscapeBatch   `json:"batch"`
	Annotation domain.SpeciesAnnotation `json:"annotation"`
}
type ConsensusResult struct {
	Batch     domain.SoundscapeBatch     `json:"batch"`
	Consensus []domain.Consensus         `json:"consensus"`
	Disputes  []domain.AnnotationDispute `json:"disputes"`
}
type ArbitrationResult struct {
	Batch   domain.SoundscapeBatch   `json:"batch"`
	Dispute domain.AnnotationDispute `json:"dispute"`
}
type FreezeResult struct {
	Batch      domain.SoundscapeBatch  `json:"batch"`
	Validation domain.ValidationReport `json:"validation"`
	Manifest   *domain.Manifest        `json:"manifest,omitempty"`
}
type PublishResult struct {
	Batch       domain.SoundscapeBatch    `json:"batch"`
	Certificate domain.ReleaseCertificate `json:"certificate"`
}
