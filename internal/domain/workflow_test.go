package domain

import (
	"errors"
	"testing"
	"time"
)

func testAggregate(t *testing.T) *Aggregate {
	t.Helper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, err := NewBatch("batch-1", "林地声景", SiteBoundary{Description: "样地", North: 31.2, South: 31.1, East: 121.6, West: 121.5}, TimeWindow{Start: start, End: start.Add(24 * time.Hour)}, "科研授权", 2, start)
	if err != nil {
		t.Fatal(err)
	}
	clip := AudioClip{ID: "clip-1", BatchID: "batch-1", SHA256: "abc", ObjectPath: "objects/ab/abc", MediaType: "audio/wav", ByteSize: 100, RecordedAt: start.Add(time.Hour), DurationMillis: 5000}
	if err = a.AddClip(clip, start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestBlindAnnotationsCreateDisputeAndResolve(t *testing.T) {
	a := testAggregate(t)
	now := a.Batch.UpdatedAt.Add(time.Minute)
	first := SpeciesAnnotation{ID: "a1", ClipID: "clip-1", AnnotatorID: "u1", SpeciesCode: "PARUS_MAJOR", StartMillis: 100, EndMillis: 800, Confidence: .9, EvidenceNote: "节律清楚"}
	if err := a.SubmitAnnotation(first, now); err != nil {
		t.Fatal(err)
	}
	if err := a.SubmitAnnotation(SpeciesAnnotation{ID: "a2", ClipID: "clip-1", AnnotatorID: "u1", SpeciesCode: "PARUS_MAJOR", StartMillis: 100, EndMillis: 800, Confidence: .8, EvidenceNote: "重复"}, now); err == nil {
		t.Fatal("同一标注员重复占位应失败")
	}
	second := SpeciesAnnotation{ID: "a2", ClipID: "clip-1", AnnotatorID: "u2", SpeciesCode: "CINEREOUS_TIT", StartMillis: 900, EndMillis: 1800, Confidence: .5, EvidenceNote: "频率较低"}
	if err := a.SubmitAnnotation(second, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if a.Batch.State != BatchAwaitConsensus {
		t.Fatalf("状态 = %s", a.Batch.State)
	}
	if err := a.ComputeConsensus(func() string { return "d1" }, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if a.Batch.State != BatchAwaitArbitration || len(a.Disputes) != 1 {
		t.Fatalf("未生成争议: %+v", a.Disputes)
	}
	if err := a.ResolveDispute("d1", "PARUS_MAJOR", "频谱支持第一分类", "expert", false, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if a.Batch.State != BatchReadyValidate {
		t.Fatalf("仲裁后状态 = %s", a.Batch.State)
	}
}

func TestAnnotationIntervalAndFrozenMutationRejected(t *testing.T) {
	a := testAggregate(t)
	err := a.SubmitAnnotation(SpeciesAnnotation{ID: "bad", ClipID: "clip-1", AnnotatorID: "u1", SpeciesCode: "X", StartMillis: 4000, EndMillis: 6000, Confidence: .5, EvidenceNote: "越界"}, time.Now())
	var de *DomainError
	if !errors.As(err, &de) || de.Field != "interval" {
		t.Fatalf("期望区间错误，得到 %v", err)
	}
	a.Batch.State = BatchFrozen
	if err = a.AddClip(AudioClip{}, time.Now()); err == nil {
		t.Fatal("冻结后应拒绝片段修改")
	}
}

func TestAgreeingAnnotationsReachValidation(t *testing.T) {
	a := testAggregate(t)
	now := time.Now()
	for _, ann := range []SpeciesAnnotation{{ID: "a1", ClipID: "clip-1", AnnotatorID: "u1", SpeciesCode: "OWL", StartMillis: 100, EndMillis: 700, Confidence: .80, EvidenceNote: "证据一"}, {ID: "a2", ClipID: "clip-1", AnnotatorID: "u2", SpeciesCode: "OWL", StartMillis: 180, EndMillis: 760, Confidence: .85, EvidenceNote: "证据二"}} {
		if err := a.SubmitAnnotation(ann, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.ComputeConsensus(func() string { return "unused" }, now); err != nil {
		t.Fatal(err)
	}
	if a.Batch.State != BatchReadyValidate || len(a.Consensus) != 1 {
		t.Fatalf("共识结果异常: %+v", a)
	}
}
