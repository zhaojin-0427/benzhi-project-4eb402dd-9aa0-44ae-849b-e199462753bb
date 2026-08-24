package domain

import "time"

type BatchState string

const (
	BatchDraft            BatchState = "draft"
	BatchAnnotating       BatchState = "annotating"
	BatchAwaitConsensus   BatchState = "awaiting_consensus"
	BatchAwaitArbitration BatchState = "awaiting_arbitration"
	BatchReadyValidate    BatchState = "ready_for_validation"
	BatchRemediation      BatchState = "remediation"
	BatchFrozen           BatchState = "frozen"
	BatchPublished        BatchState = "published"
)

type TimeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type SiteBoundary struct {
	Description string  `json:"description"`
	North       float64 `json:"north"`
	South       float64 `json:"south"`
	East        float64 `json:"east"`
	West        float64 `json:"west"`
}

type SoundscapeBatch struct {
	ID                 string       `json:"id"`
	Title              string       `json:"title"`
	SiteBoundary       SiteBoundary `json:"siteBoundary"`
	SampleWindow       TimeWindow   `json:"sampleWindow"`
	LicenseStatement   string       `json:"licenseStatement"`
	RequiredAnnotators int          `json:"requiredAnnotators"`
	State              BatchState   `json:"state"`
	Version            int64        `json:"version"`
	CreatedAt          time.Time    `json:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt"`
	ManifestDigest     string       `json:"manifestDigest,omitempty"`
	ValidationIssues   []string     `json:"validationIssues,omitempty"`
}

type AudioClip struct {
	ID             string    `json:"id"`
	BatchID        string    `json:"batchId"`
	SHA256         string    `json:"sha256"`
	ObjectPath     string    `json:"objectPath"`
	MediaType      string    `json:"mediaType"`
	ByteSize       int64     `json:"byteSize"`
	RecordedAt     time.Time `json:"recordedAt"`
	DurationMillis int64     `json:"durationMillis"`
	RecorderCode   string    `json:"recorderCode"`
	HabitatNote    string    `json:"habitatNote"`
}

type SpeciesAnnotation struct {
	ID           string    `json:"id"`
	ClipID       string    `json:"clipId"`
	AnnotatorID  string    `json:"annotatorId"`
	SpeciesCode  string    `json:"speciesCode"`
	StartMillis  int64     `json:"startMillis"`
	EndMillis    int64     `json:"endMillis"`
	Confidence   float64   `json:"confidence"`
	EvidenceNote string    `json:"evidenceNote"`
	SubmittedAt  time.Time `json:"submittedAt"`
	Revision     int       `json:"revision"`
}

type DisputeState string

const (
	DisputeOpen     DisputeState = "open"
	DisputeResolved DisputeState = "resolved"
	DisputeReturned DisputeState = "returned"
)

type AnnotationDispute struct {
	ID               string       `json:"id"`
	BatchID          string       `json:"batchId"`
	ClipID           string       `json:"clipId"`
	ReasonCodes      []string     `json:"reasonCodes"`
	AnnotationIDs    []string     `json:"annotationIds"`
	State            DisputeState `json:"state"`
	FinalSpeciesCode string       `json:"finalSpeciesCode,omitempty"`
	Rationale        string       `json:"rationale,omitempty"`
	ArbiterID        string       `json:"arbiterId,omitempty"`
	ResolvedAt       *time.Time   `json:"resolvedAt,omitempty"`
}

type Consensus struct {
	ClipID      string   `json:"clipId"`
	SpeciesCode string   `json:"speciesCode"`
	StartMillis int64    `json:"startMillis"`
	EndMillis   int64    `json:"endMillis"`
	Confidence  float64  `json:"confidence"`
	SourceIDs   []string `json:"sourceIds"`
}

type Manifest struct {
	SchemaVersion string               `json:"schemaVersion"`
	BatchID       string               `json:"batchId"`
	BatchVersion  int64                `json:"batchVersion"`
	GeneratedAt   time.Time            `json:"generatedAt"`
	Clips         []ManifestClip       `json:"clips"`
	Conclusions   []ManifestConclusion `json:"conclusions"`
	Digest        string               `json:"digest"`
}

type ManifestClip struct {
	ID         string `json:"id"`
	SHA256     string `json:"sha256"`
	MediaType  string `json:"mediaType"`
	ByteSize   int64  `json:"byteSize"`
	RecordedAt string `json:"recordedAt"`
}

type ManifestConclusion struct {
	ClipID      string `json:"clipId"`
	SpeciesCode string `json:"speciesCode"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Basis       string `json:"basis"`
}

type ReleaseCertificate struct {
	ID               string    `json:"id"`
	BatchID          string    `json:"batchId"`
	ManifestDigest   string    `json:"manifestDigest"`
	EventChainHead   string    `json:"eventChainHead"`
	ApprovedBy       string    `json:"approvedBy"`
	IssuedAt         time.Time `json:"issuedAt"`
	SchemaVersion    string    `json:"schemaVersion"`
	VerificationCode string    `json:"verificationCode"`
}

type Aggregate struct {
	Batch       SoundscapeBatch     `json:"batch"`
	Clips       []AudioClip         `json:"clips"`
	Annotations []SpeciesAnnotation `json:"annotations"`
	Disputes    []AnnotationDispute `json:"disputes"`
	Consensus   []Consensus         `json:"consensus"`
	Manifest    *Manifest           `json:"manifest,omitempty"`
	Certificate *ReleaseCertificate `json:"certificate,omitempty"`
}
