package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"soundledger/internal/domain"
	"sync"
	"time"
)

type EvidenceService struct {
	now       func() time.Time
	id        func() string
	mu        sync.RWMutex
	manifests map[string]domain.Manifest
}

func NewEvidenceService(now func() time.Time, id func() string) *EvidenceService {
	return &EvidenceService{now: now, id: id, manifests: map[string]domain.Manifest{}}
}

func (s *EvidenceService) BuildManifest(aggregate *domain.Aggregate) (domain.Manifest, error) {
	m := aggregate.ManifestInput(s.now().UTC())
	m.Digest = ""
	digest, err := Digest(m)
	if err != nil {
		return domain.Manifest{}, err
	}
	m.Digest = digest
	s.mu.Lock()
	s.manifests[m.BatchID] = m
	s.mu.Unlock()
	return m, nil
}

func (s *EvidenceService) IssueCertificate(batchID, manifestDigest, chainHead, approvedBy string) (domain.ReleaseCertificate, error) {
	if manifestDigest == "" || chainHead == "" || approvedBy == "" {
		return domain.ReleaseCertificate{}, fmt.Errorf("证书输入不完整")
	}
	s.mu.RLock()
	registered, ok := s.manifests[batchID]
	s.mu.RUnlock()
	if !ok {
		return domain.ReleaseCertificate{}, fmt.Errorf("冻结清单尚未登记")
	}
	if registered.Digest != manifestDigest {
		return domain.ReleaseCertificate{}, fmt.Errorf("冻结清单摘要与登记记录不一致")
	}
	issued := s.now().UTC()
	material := struct {
		BatchID, ManifestDigest, EventChainHead, ApprovedBy string
		IssuedAt                                            time.Time
	}{batchID, manifestDigest, chainHead, approvedBy, issued}
	b, err := CanonicalJSON(material)
	if err != nil {
		return domain.ReleaseCertificate{}, err
	}
	sum := sha256.Sum256(b)
	return domain.ReleaseCertificate{ID: s.id(), BatchID: batchID, ManifestDigest: manifestDigest, EventChainHead: chainHead, ApprovedBy: approvedBy, IssuedAt: issued, SchemaVersion: "soundledger-certificate/v1", VerificationCode: hex.EncodeToString(sum[:])}, nil
}

func VerifyCertificate(c domain.ReleaseCertificate, m domain.Manifest, chainHead string) error {
	if c.SchemaVersion != "soundledger-certificate/v1" {
		return fmt.Errorf("证书版本无效")
	}
	if c.ManifestDigest != m.Digest {
		return fmt.Errorf("证书与清单摘要不一致")
	}
	if c.EventChainHead != chainHead {
		return fmt.Errorf("证书与事件链头不一致")
	}
	copyManifest := m
	copyManifest.Digest = ""
	digest, err := Digest(copyManifest)
	if err != nil {
		return err
	}
	if digest != m.Digest {
		return fmt.Errorf("清单摘要校验失败")
	}
	material := struct {
		BatchID, ManifestDigest, EventChainHead, ApprovedBy string
		IssuedAt                                            time.Time
	}{c.BatchID, c.ManifestDigest, c.EventChainHead, c.ApprovedBy, c.IssuedAt}
	b, err := CanonicalJSON(material)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != c.VerificationCode {
		return fmt.Errorf("证书验证码校验失败")
	}
	return nil
}
