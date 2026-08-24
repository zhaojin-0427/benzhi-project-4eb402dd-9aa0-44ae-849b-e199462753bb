package domain

import (
	"sort"
	"time"
)

func (a *Aggregate) PrepareFreeze(now time.Time) error {
	if a.Batch.State != BatchReadyValidate && a.Batch.State != BatchRemediation {
		return NewError(ErrInvalidState, "当前批次不能冻结")
	}
	issues := a.ValidationProblems()
	if len(issues) > 0 {
		a.Batch.ValidationIssues = issues
		a.Batch.State = BatchRemediation
		a.touch(now)
		return &DomainError{Code: ErrIncomplete, Message: "冻结校验失败"}
	}
	return nil
}

func (a *Aggregate) RecordValidationFailure(issues []string, now time.Time) error {
	if a.Batch.State != BatchReadyValidate && a.Batch.State != BatchRemediation {
		return NewError(ErrInvalidState, "当前批次不能执行冻结校验")
	}
	if len(issues) == 0 {
		return FieldError("issues", "整改问题清单不能为空")
	}
	a.Batch.ValidationIssues = append([]string(nil), issues...)
	a.Batch.State = BatchRemediation
	a.touch(now)
	return nil
}

func (a *Aggregate) Freeze(manifest Manifest, now time.Time) error {
	if a.Batch.State != BatchReadyValidate && a.Batch.State != BatchRemediation {
		return NewError(ErrInvalidState, "当前批次不能冻结")
	}
	if manifest.BatchID != a.Batch.ID || manifest.Digest == "" {
		return FieldError("manifest", "清单与批次不匹配")
	}
	a.Manifest = &manifest
	a.Batch.ManifestDigest = manifest.Digest
	a.Batch.ValidationIssues = nil
	a.Batch.State = BatchFrozen
	a.touch(now)
	return nil
}

func (a *Aggregate) Publish(certificate ReleaseCertificate, now time.Time) error {
	if a.Batch.State == BatchPublished {
		return NewError(ErrAlreadyPublished, "批次已经发布")
	}
	if a.Batch.State != BatchFrozen || a.Manifest == nil {
		return NewError(ErrInvalidState, "只有已冻结批次可以发布")
	}
	if certificate.BatchID != a.Batch.ID || certificate.ManifestDigest != a.Manifest.Digest {
		return FieldError("certificate", "证书未绑定当前冻结清单")
	}
	a.Certificate = &certificate
	a.Batch.State = BatchPublished
	a.touch(now)
	return nil
}

func (a *Aggregate) ManifestInput(now time.Time) Manifest {
	m := Manifest{SchemaVersion: "soundledger-manifest/v1", BatchID: a.Batch.ID, BatchVersion: a.Batch.Version + 1, GeneratedAt: now}
	for _, clip := range a.Clips {
		m.Clips = append(m.Clips, ManifestClip{ID: clip.ID, SHA256: clip.SHA256, MediaType: clip.MediaType, ByteSize: clip.ByteSize, RecordedAt: clip.RecordedAt.UTC().Format(time.RFC3339Nano)})
	}
	for _, c := range a.Consensus {
		basis := "consensus"
		for _, d := range a.Disputes {
			if d.ClipID == c.ClipID && d.State == DisputeResolved {
				basis = "arbitration"
			}
		}
		m.Conclusions = append(m.Conclusions, ManifestConclusion{ClipID: c.ClipID, SpeciesCode: c.SpeciesCode, StartMillis: c.StartMillis, EndMillis: c.EndMillis, Basis: basis})
	}
	sort.Slice(m.Clips, func(i, j int) bool { return m.Clips[i].ID < m.Clips[j].ID })
	sort.Slice(m.Conclusions, func(i, j int) bool { return m.Conclusions[i].ClipID < m.Conclusions[j].ClipID })
	return m
}
