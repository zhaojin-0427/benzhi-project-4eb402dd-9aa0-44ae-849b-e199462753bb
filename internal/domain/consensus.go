package domain

import (
	"math"
	"sort"
	"time"
)

const intervalToleranceMillis int64 = 250
const confidenceTolerance = 0.15

func CompareAnnotations(left, right SpeciesAnnotation) ([]string, *Consensus) {
	reasons := []string{}
	if left.SpeciesCode != right.SpeciesCode {
		reasons = append(reasons, "species_mismatch")
	}
	if abs64(left.StartMillis-right.StartMillis) > intervalToleranceMillis || abs64(left.EndMillis-right.EndMillis) > intervalToleranceMillis {
		reasons = append(reasons, "interval_mismatch")
	}
	if math.Abs(left.Confidence-right.Confidence) > confidenceTolerance {
		reasons = append(reasons, "confidence_mismatch")
	}
	if len(reasons) > 0 {
		return reasons, nil
	}
	ids := []string{left.ID, right.ID}
	sort.Strings(ids)
	return nil, &Consensus{ClipID: left.ClipID, SpeciesCode: left.SpeciesCode, StartMillis: (left.StartMillis + right.StartMillis) / 2, EndMillis: (left.EndMillis + right.EndMillis) / 2, Confidence: (left.Confidence + right.Confidence) / 2, SourceIDs: ids}
}

func (a *Aggregate) ComputeConsensus(idFactory func() string, now time.Time) error {
	if a.Batch.State != BatchAwaitConsensus {
		return NewError(ErrInvalidState, "只有双标齐备的批次可以计算共识")
	}
	if !a.allClipsCovered() {
		return NewError(ErrIncomplete, "所有片段必须完成双人标注")
	}
	a.Consensus = nil
	a.Disputes = nil
	for _, clip := range a.Clips {
		pair := []SpeciesAnnotation{}
		for _, ann := range a.Annotations {
			if ann.ClipID == clip.ID {
				pair = append(pair, ann)
			}
		}
		sort.Slice(pair, func(i, j int) bool { return pair[i].ID < pair[j].ID })
		reasons, conclusion := CompareAnnotations(pair[0], pair[1])
		if conclusion != nil {
			a.Consensus = append(a.Consensus, *conclusion)
			continue
		}
		a.Disputes = append(a.Disputes, AnnotationDispute{ID: idFactory(), BatchID: a.Batch.ID, ClipID: clip.ID, ReasonCodes: reasons, AnnotationIDs: []string{pair[0].ID, pair[1].ID}, State: DisputeOpen})
	}
	if len(a.Disputes) > 0 {
		a.Batch.State = BatchAwaitArbitration
	} else {
		a.Batch.State = BatchReadyValidate
	}
	a.touch(now)
	return nil
}

func (a *Aggregate) ResolveDispute(disputeID, species, rationale, arbiter string, returnForRevision bool, now time.Time) error {
	if a.Batch.State != BatchAwaitArbitration {
		return NewError(ErrInvalidState, "当前批次不处于仲裁阶段")
	}
	for i := range a.Disputes {
		d := &a.Disputes[i]
		if d.ID != disputeID {
			continue
		}
		if d.State != DisputeOpen {
			return NewError(ErrInvalidState, "争议已处理")
		}
		if rationale == "" || arbiter == "" {
			return FieldError("rationale", "仲裁专家和理由不能为空")
		}
		if returnForRevision {
			d.State = DisputeReturned
			d.Rationale = rationale
			d.ArbiterID = arbiter
			a.removeClipAnnotations(d.ClipID)
			a.removeClipConsensus(d.ClipID)
			a.Batch.State = BatchRemediation
			a.touch(now)
			return nil
		}
		if species == "" {
			return FieldError("finalSpeciesCode", "最终物种代码不能为空")
		}
		d.State = DisputeResolved
		d.FinalSpeciesCode = species
		d.Rationale = rationale
		d.ArbiterID = arbiter
		d.ResolvedAt = &now
		a.Consensus = append(a.Consensus, Consensus{ClipID: d.ClipID, SpeciesCode: species, SourceIDs: append([]string(nil), d.AnnotationIDs...)})
		all := true
		for _, item := range a.Disputes {
			if item.State != DisputeResolved {
				all = false
			}
		}
		if all {
			a.Batch.State = BatchReadyValidate
		}
		a.touch(now)
		return nil
	}
	return NewError(ErrNotFound, "争议不存在")
}

func (a *Aggregate) removeClipAnnotations(clipID string) {
	kept := a.Annotations[:0]
	for _, ann := range a.Annotations {
		if ann.ClipID != clipID {
			kept = append(kept, ann)
		}
	}
	a.Annotations = kept
}
func (a *Aggregate) removeClipConsensus(clipID string) {
	kept := a.Consensus[:0]
	for _, c := range a.Consensus {
		if c.ClipID != clipID {
			kept = append(kept, c)
		}
	}
	a.Consensus = kept
}
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
