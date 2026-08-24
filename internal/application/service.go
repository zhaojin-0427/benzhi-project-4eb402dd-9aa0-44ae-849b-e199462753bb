package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"soundledger/internal/domain"
	"soundledger/internal/store"
	"time"
)

func validateMeta(meta CommandMeta, allowed ...Role) error {
	if meta.ActorID == "" {
		return domain.FieldError("actorId", "操作者不能为空")
	}
	if meta.RequestID == "" {
		return domain.FieldError("requestId", "请求关联标识不能为空")
	}
	if meta.IdempotencyKey == "" {
		return domain.FieldError("idempotencyKey", "幂等键不能为空")
	}
	for _, role := range allowed {
		if meta.Role == role {
			return nil
		}
	}
	return domain.NewError(domain.ErrForbidden, "当前角色无权执行此操作")
}

func (s *Service) idempotent(key, operation, batchID string, target any) (bool, error) {
	record, ok := s.store.Idempotency(key)
	if !ok {
		return false, nil
	}
	if record.Operation != operation || record.BatchID != batchID {
		return false, domain.NewError(domain.ErrConflict, "幂等键已用于其他操作")
	}
	if err := json.Unmarshal(record.Response, target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) CreateBatch(command CreateBatchCommand) (domain.SoundscapeBatch, error) {
	if err := validateMeta(command.Meta, RoleAdministrator); err != nil {
		return domain.SoundscapeBatch{}, err
	}
	value, err := s.mailboxes.forBatch(command.ID).do(func() (any, error) {
		var cached domain.SoundscapeBatch
		if ok, err := s.idempotent(command.Meta.IdempotencyKey, "create_batch", command.ID, &cached); ok || err != nil {
			return cached, err
		}
		if _, err := s.store.Get(command.ID); err == nil {
			return nil, domain.NewError(domain.ErrDuplicate, "批次编号已存在")
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		now := s.clock().UTC()
		aggregate, err := domain.NewBatch(command.ID, command.Title, command.SiteBoundary, command.SampleWindow, command.LicenseStatement, command.RequiredAnnotators, now)
		if err != nil {
			return nil, err
		}
		result := aggregate.Batch
		err = s.store.Commit(store.CommitRequest{Aggregate: aggregate, ExpectedVersion: 0, EventType: "batch.created", ActorID: command.Meta.ActorID, RequestID: command.Meta.RequestID, Payload: map[string]any{"title": command.Title, "requiredAnnotators": command.RequiredAnnotators}, IdempotencyKey: command.Meta.IdempotencyKey, Operation: "create_batch", Response: result, Status: 201, OccurredAt: now})
		return result, mapStoreError(err)
	})
	if err != nil {
		return domain.SoundscapeBatch{}, err
	}
	return value.(domain.SoundscapeBatch), nil
}

func (s *Service) UploadClip(command UploadClipCommand) (UploadResult, error) {
	if err := validateMeta(command.Meta, RoleAdministrator); err != nil {
		return UploadResult{}, err
	}
	value, err := s.mailboxes.forBatch(command.BatchID).do(func() (any, error) {
		var cached UploadResult
		if ok, err := s.idempotent(command.Meta.IdempotencyKey, "upload_clip", command.BatchID, &cached); ok || err != nil {
			return cached, err
		}
		aggregate, err := s.store.Get(command.BatchID)
		if err != nil {
			return nil, mapStoreError(err)
		}
		if aggregate.Batch.Version != command.Meta.ExpectedVersion {
			return nil, domain.NewError(domain.ErrConflict, "批次版本已变化")
		}
		if !store.ValidateMediaType(command.MediaType) {
			return nil, domain.FieldError("mediaType", "仅支持 WAV、FLAC、MP3 或 OGG 录音")
		}
		validatedContent, err := store.ValidateAudioContent(command.Content, command.MediaType)
		if err != nil {
			return nil, domain.FieldError("audio", err.Error())
		}
		object, err := s.store.PutObject(validatedContent, s.maxUploadBytes)
		if err != nil {
			return nil, domain.FieldError("audio", err.Error())
		}
		clip := domain.AudioClip{ID: command.ClipID, BatchID: command.BatchID, SHA256: object.SHA256, ObjectPath: object.Path, MediaType: command.MediaType, ByteSize: object.Size, RecordedAt: command.RecordedAt.UTC(), DurationMillis: command.DurationMillis, RecorderCode: command.RecorderCode, HabitatNote: command.HabitatNote}
		now := s.clock().UTC()
		if err = aggregate.AddClip(clip, now); err != nil {
			return nil, err
		}
		result := UploadResult{Batch: aggregate.Batch, Clip: clip}
		err = s.store.Commit(store.CommitRequest{Aggregate: aggregate, ExpectedVersion: command.Meta.ExpectedVersion, EventType: "clip.uploaded", ActorID: command.Meta.ActorID, RequestID: command.Meta.RequestID, Payload: clip, IdempotencyKey: command.Meta.IdempotencyKey, Operation: "upload_clip", Response: result, Status: 201, OccurredAt: now})
		return result, mapStoreError(err)
	})
	if err != nil {
		return UploadResult{}, err
	}
	return value.(UploadResult), nil
}

func (s *Service) SubmitAnnotation(command SubmitAnnotationCommand) (AnnotationResult, error) {
	if err := validateMeta(command.Meta, RoleAnnotator); err != nil {
		return AnnotationResult{}, err
	}
	value, err := s.mailboxes.forBatch(command.BatchID).do(func() (any, error) {
		var cached AnnotationResult
		if ok, err := s.idempotent(command.Meta.IdempotencyKey, "submit_annotation", command.BatchID, &cached); ok || err != nil {
			return cached, err
		}
		aggregate, err := s.loadExpected(command.BatchID, command.Meta.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		annotation := domain.SpeciesAnnotation{ID: command.AnnotationID, ClipID: command.ClipID, AnnotatorID: command.Meta.ActorID, SpeciesCode: command.SpeciesCode, StartMillis: command.StartMillis, EndMillis: command.EndMillis, Confidence: command.Confidence, EvidenceNote: command.EvidenceNote}
		now := s.clock().UTC()
		if err = aggregate.SubmitAnnotation(annotation, now); err != nil {
			return nil, err
		}
		annotation.SubmittedAt = now
		annotation.Revision = 1
		result := AnnotationResult{Batch: aggregate.Batch, Annotation: annotation}
		err = s.commit(aggregate, command.Meta, "annotation.submitted", "submit_annotation", map[string]any{"clipId": command.ClipID, "annotationId": command.AnnotationID}, result, now)
		return result, err
	})
	if err != nil {
		return AnnotationResult{}, err
	}
	return value.(AnnotationResult), nil
}

func (s *Service) ComputeConsensus(command ComputeConsensusCommand) (ConsensusResult, error) {
	if err := validateMeta(command.Meta, RoleAdministrator); err != nil {
		return ConsensusResult{}, err
	}
	value, err := s.mailboxes.forBatch(command.BatchID).do(func() (any, error) {
		var cached ConsensusResult
		if ok, err := s.idempotent(command.Meta.IdempotencyKey, "compute_consensus", command.BatchID, &cached); ok || err != nil {
			return cached, err
		}
		aggregate, err := s.loadExpected(command.BatchID, command.Meta.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		now := s.clock().UTC()
		if err = aggregate.ComputeConsensus(s.ids, now); err != nil {
			return nil, err
		}
		result := ConsensusResult{Batch: aggregate.Batch, Consensus: aggregate.Consensus, Disputes: aggregate.Disputes}
		err = s.commit(aggregate, command.Meta, "consensus.computed", "compute_consensus", map[string]any{"consensusCount": len(aggregate.Consensus), "disputeCount": len(aggregate.Disputes)}, result, now)
		return result, err
	})
	if err != nil {
		return ConsensusResult{}, err
	}
	return value.(ConsensusResult), nil
}

func (s *Service) Arbitrate(command ArbitrateCommand) (ArbitrationResult, error) {
	if err := validateMeta(command.Meta, RoleArbiter); err != nil {
		return ArbitrationResult{}, err
	}
	value, err := s.mailboxes.forBatch(command.BatchID).do(func() (any, error) {
		var cached ArbitrationResult
		if ok, err := s.idempotent(command.Meta.IdempotencyKey, "arbitrate", command.BatchID, &cached); ok || err != nil {
			return cached, err
		}
		aggregate, err := s.loadExpected(command.BatchID, command.Meta.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		now := s.clock().UTC()
		if err = aggregate.ResolveDispute(command.DisputeID, command.FinalSpeciesCode, command.Rationale, command.Meta.ActorID, command.ReturnForRevision, now); err != nil {
			return nil, err
		}
		var dispute domain.AnnotationDispute
		for _, item := range aggregate.Disputes {
			if item.ID == command.DisputeID {
				dispute = item
			}
		}
		result := ArbitrationResult{Batch: aggregate.Batch, Dispute: dispute}
		err = s.commit(aggregate, command.Meta, "dispute.arbitrated", "arbitrate", map[string]any{"disputeId": command.DisputeID, "returned": command.ReturnForRevision}, result, now)
		return result, err
	})
	if err != nil {
		return ArbitrationResult{}, err
	}
	return value.(ArbitrationResult), nil
}

func (s *Service) Freeze(command FreezeCommand) (FreezeResult, error) {
	if err := validateMeta(command.Meta, RoleAdministrator); err != nil {
		return FreezeResult{}, err
	}
	value, err := s.mailboxes.forBatch(command.BatchID).do(func() (any, error) {
		var cached FreezeResult
		if ok, err := s.idempotent(command.Meta.IdempotencyKey, "freeze", command.BatchID, &cached); ok || err != nil {
			return cached, err
		}
		aggregate, err := s.loadExpected(command.BatchID, command.Meta.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		now := s.clock().UTC()
		validation := aggregate.EvaluateFreeze(s.store.VerifyClips(aggregate.Clips))
		if !validation.Ready {
			if err = aggregate.RecordValidationFailure(validation.Issues, now); err != nil {
				return nil, err
			}
			result := FreezeResult{Batch: aggregate.Batch, Validation: validation}
			err = s.commit(aggregate, command.Meta, "batch.validation_failed", "freeze", map[string]any{"issues": validation.Issues}, result, now)
			return result, err
		}
		if err = aggregate.PrepareFreeze(now); err != nil {
			return nil, err
		}
		manifest, err := s.evidence.BuildManifest(aggregate)
		if err != nil {
			return nil, err
		}
		if err = aggregate.Freeze(manifest, now); err != nil {
			return nil, err
		}
		result := FreezeResult{Batch: aggregate.Batch, Validation: validation, Manifest: &manifest}
		err = s.commit(aggregate, command.Meta, "batch.frozen", "freeze", map[string]any{"manifestDigest": manifest.Digest}, result, now)
		return result, err
	})
	if err != nil {
		return FreezeResult{}, err
	}
	return value.(FreezeResult), nil
}

func (s *Service) Publish(command PublishCommand) (PublishResult, error) {
	if err := validateMeta(command.Meta, RolePublisher); err != nil {
		return PublishResult{}, err
	}
	chainHead := s.store.ChainHead()
	value, err := s.mailboxes.forBatch(command.BatchID).do(func() (any, error) {
		var cached PublishResult
		if ok, err := s.idempotent(command.Meta.IdempotencyKey, "publish", command.BatchID, &cached); ok || err != nil {
			return cached, err
		}
		aggregate, err := s.loadExpected(command.BatchID, command.Meta.ExpectedVersion)
		if err != nil {
			return nil, err
		}
		if aggregate.Manifest == nil {
			return nil, domain.NewError(domain.ErrInvalidState, "冻结清单不存在")
		}
		certificate, err := s.evidence.IssueCertificate(command.BatchID, aggregate.Manifest.Digest, chainHead, command.Meta.ActorID)
		if err != nil {
			return nil, err
		}
		now := s.clock().UTC()
		if err = aggregate.Publish(certificate, now); err != nil {
			return nil, err
		}
		result := PublishResult{Batch: aggregate.Batch, Certificate: certificate}
		err = s.commit(aggregate, command.Meta, "batch.published", "publish", map[string]any{"certificateId": certificate.ID, "verificationCode": certificate.VerificationCode}, result, now)
		return result, err
	})
	if err != nil {
		return PublishResult{}, err
	}
	return value.(PublishResult), nil
}

func (s *Service) loadExpected(batchID string, expected int64) (*domain.Aggregate, error) {
	aggregate, err := s.store.Get(batchID)
	if err != nil {
		return nil, mapStoreError(err)
	}
	if aggregate.Batch.Version != expected {
		return nil, domain.NewError(domain.ErrConflict, "批次版本已变化")
	}
	return aggregate, nil
}
func (s *Service) commit(a *domain.Aggregate, meta CommandMeta, eventType, operation string, payload, response any, now time.Time) error {
	return mapStoreError(s.store.Commit(store.CommitRequest{Aggregate: a, ExpectedVersion: meta.ExpectedVersion, EventType: eventType, ActorID: meta.ActorID, RequestID: meta.RequestID, Payload: payload, IdempotencyKey: meta.IdempotencyKey, Operation: operation, Response: response, Status: 200, OccurredAt: now}))
}
func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return domain.NewError(domain.ErrNotFound, "批次不存在")
	}
	if errors.Is(err, store.ErrVersionConflict) {
		return domain.NewError(domain.ErrConflict, "批次版本已变化")
	}
	if errors.Is(err, store.ErrIdempotencyConflict) {
		return domain.NewError(domain.ErrConflict, "幂等键已用于其他操作")
	}
	return fmt.Errorf("存储提交失败: %w", err)
}
