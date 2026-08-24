package domain

import (
	"fmt"
	"strings"
	"time"
)

func NewBatch(id, title string, boundary SiteBoundary, window TimeWindow, license string, required int, now time.Time) (*Aggregate, error) {
	if err := ValidateIdentifier("id", id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(title) == "" {
		return nil, FieldError("title", "批次标题不能为空")
	}
	if boundary.North < boundary.South || boundary.East < boundary.West {
		return nil, FieldError("siteBoundary", "地点范围坐标顺序无效")
	}
	if boundary.North > 90 || boundary.South < -90 || boundary.East > 180 || boundary.West < -180 {
		return nil, FieldError("siteBoundary", "地点范围坐标超出经纬度范围")
	}
	if window.Start.IsZero() || !window.End.After(window.Start) {
		return nil, FieldError("sampleWindow", "采样结束时间必须晚于开始时间")
	}
	if strings.TrimSpace(license) == "" {
		return nil, FieldError("licenseStatement", "授权声明不能为空")
	}
	if required != 2 {
		return nil, FieldError("requiredAnnotators", "当前治理规则要求恰好两名标注员")
	}
	return &Aggregate{Batch: SoundscapeBatch{ID: id, Title: strings.TrimSpace(title), SiteBoundary: boundary, SampleWindow: window, LicenseStatement: strings.TrimSpace(license), RequiredAnnotators: required, State: BatchDraft, Version: 1, CreatedAt: now, UpdatedAt: now}}, nil
}

func (a *Aggregate) FindClip(id string) (*AudioClip, error) {
	for i := range a.Clips {
		if a.Clips[i].ID == id {
			return &a.Clips[i], nil
		}
	}
	return nil, NewError(ErrNotFound, "录音片段不存在")
}

func (a *Aggregate) EnsureMutable() error {
	if a.Batch.State == BatchFrozen || a.Batch.State == BatchPublished {
		return NewError(ErrInvalidState, "数据集冻结后禁止修改样本或标注")
	}
	return nil
}

func (a *Aggregate) AddClip(clip AudioClip, now time.Time) error {
	if err := a.EnsureMutable(); err != nil {
		return err
	}
	if a.Batch.State != BatchDraft && a.Batch.State != BatchAnnotating && a.Batch.State != BatchRemediation {
		return NewError(ErrInvalidState, "当前批次状态不允许上传片段")
	}
	if clip.BatchID != a.Batch.ID {
		return FieldError("batchId", "片段批次编号不匹配")
	}
	if err := ValidateIdentifier("clipId", clip.ID); err != nil {
		return err
	}
	if clip.SHA256 == "" || clip.ByteSize <= 0 {
		return FieldError("audio", "录音内容为空或摘要缺失")
	}
	if clip.DurationMillis <= 0 {
		return FieldError("durationMillis", "录音时长必须大于零")
	}
	if clip.RecordedAt.Before(a.Batch.SampleWindow.Start) || clip.RecordedAt.After(a.Batch.SampleWindow.End) {
		return FieldError("recordedAt", "录制时间不在批次采样时段内")
	}
	for _, existing := range a.Clips {
		if existing.ID == clip.ID {
			return NewError(ErrDuplicate, "批次内已存在相同编号的片段")
		}
		if existing.SHA256 == clip.SHA256 {
			return NewError(ErrDuplicate, "批次内已存在相同内容的片段")
		}
	}
	a.Clips = append(a.Clips, clip)
	a.Batch.State = BatchAnnotating
	a.touch(now)
	return nil
}

func (a *Aggregate) SubmitAnnotation(annotation SpeciesAnnotation, now time.Time) error {
	if err := a.EnsureMutable(); err != nil {
		return err
	}
	if a.Batch.State != BatchAnnotating && a.Batch.State != BatchAwaitConsensus && a.Batch.State != BatchRemediation {
		return NewError(ErrInvalidState, "当前批次状态不允许提交标注")
	}
	clip, err := a.FindClip(annotation.ClipID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(annotation.AnnotatorID) == "" {
		return FieldError("annotatorId", "标注员不能为空")
	}
	if err := ValidateIdentifier("annotationId", annotation.ID); err != nil {
		return err
	}
	if strings.TrimSpace(annotation.SpeciesCode) == "" {
		return FieldError("speciesCode", "物种代码不能为空")
	}
	if annotation.StartMillis < 0 || annotation.EndMillis <= annotation.StartMillis || annotation.EndMillis > clip.DurationMillis {
		return FieldError("interval", "鸣声时间区间超出片段范围")
	}
	if annotation.Confidence < 0 || annotation.Confidence > 1 {
		return FieldError("confidence", "置信度必须位于 0 到 1 之间")
	}
	if strings.TrimSpace(annotation.EvidenceNote) == "" {
		return FieldError("evidenceNote", "证据说明不能为空")
	}
	count := 0
	for _, existing := range a.Annotations {
		if existing.ID == annotation.ID {
			return NewError(ErrDuplicate, "标注编号已存在")
		}
		if existing.ClipID != annotation.ClipID {
			continue
		}
		count++
		if existing.AnnotatorID == annotation.AnnotatorID {
			return NewError(ErrDuplicate, "同一标注员不能重复占位")
		}
	}
	if count >= a.Batch.RequiredAnnotators {
		return NewError(ErrInvalidState, "片段的标注席位已满")
	}
	annotation.SubmittedAt = now
	annotation.Revision = 1
	a.Annotations = append(a.Annotations, annotation)
	if a.allClipsCovered() {
		a.Batch.State = BatchAwaitConsensus
	}
	a.touch(now)
	return nil
}

func (a *Aggregate) allClipsCovered() bool {
	if len(a.Clips) == 0 {
		return false
	}
	for _, clip := range a.Clips {
		seen := map[string]bool{}
		for _, ann := range a.Annotations {
			if ann.ClipID == clip.ID {
				seen[ann.AnnotatorID] = true
			}
		}
		if len(seen) != a.Batch.RequiredAnnotators {
			return false
		}
	}
	return true
}

func (a *Aggregate) AnnotationCoverage() (int, int) {
	covered := 0
	for _, clip := range a.Clips {
		seen := map[string]bool{}
		for _, ann := range a.Annotations {
			if ann.ClipID == clip.ID {
				seen[ann.AnnotatorID] = true
			}
		}
		if len(seen) == a.Batch.RequiredAnnotators {
			covered++
		}
	}
	return covered, len(a.Clips)
}

func (a *Aggregate) ValidationProblems() []string {
	issues := []string{}
	if len(a.Clips) == 0 {
		issues = append(issues, "批次没有录音片段")
	}
	covered, total := a.AnnotationCoverage()
	if covered != total {
		issues = append(issues, fmt.Sprintf("双标覆盖率不足：%d/%d", covered, total))
	}
	if strings.TrimSpace(a.Batch.LicenseStatement) == "" {
		issues = append(issues, "授权声明缺失")
	}
	for _, d := range a.Disputes {
		if d.State == DisputeOpen || d.State == DisputeReturned {
			issues = append(issues, "存在未解决争议："+d.ID)
		}
	}
	return issues
}

func (a *Aggregate) touch(now time.Time) { a.Batch.Version++; a.Batch.UpdatedAt = now }
