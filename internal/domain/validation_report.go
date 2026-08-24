package domain

import "fmt"

type ValidationCheck struct {
	Name    string   `json:"name"`
	Passed  bool     `json:"passed"`
	Summary string   `json:"summary"`
	Issues  []string `json:"issues,omitempty"`
}

type ValidationReport struct {
	BatchID        string            `json:"batchId"`
	BatchVersion   int64             `json:"batchVersion"`
	Ready          bool              `json:"ready"`
	Checks         []ValidationCheck `json:"checks"`
	Issues         []string          `json:"issues,omitempty"`
	ClipCount      int               `json:"clipCount"`
	CoveredClips   int               `json:"coveredClips"`
	OpenDisputes   int               `json:"openDisputes"`
	ObjectFailures int               `json:"objectFailures"`
}

func (a *Aggregate) EvaluateFreeze(objectIssues []string) ValidationReport {
	report := ValidationReport{BatchID: a.Batch.ID, BatchVersion: a.Batch.Version, ClipCount: len(a.Clips)}
	clipIssues := []string{}
	if len(a.Clips) == 0 {
		clipIssues = append(clipIssues, "批次没有录音片段")
	}
	report.Checks = append(report.Checks, ValidationCheck{Name: "clip_inventory", Passed: len(clipIssues) == 0, Summary: fmt.Sprintf("已登记 %d 个录音片段", len(a.Clips)), Issues: clipIssues})
	report.Issues = append(report.Issues, clipIssues...)

	covered, total := a.AnnotationCoverage()
	report.CoveredClips = covered
	coverageIssues := []string{}
	if covered != total {
		coverageIssues = append(coverageIssues, fmt.Sprintf("双标覆盖率不足：%d/%d", covered, total))
	}
	report.Checks = append(report.Checks, ValidationCheck{Name: "double_annotation_coverage", Passed: len(coverageIssues) == 0, Summary: fmt.Sprintf("双标覆盖 %d/%d", covered, total), Issues: coverageIssues})
	report.Issues = append(report.Issues, coverageIssues...)

	disputeIssues := []string{}
	for _, dispute := range a.Disputes {
		if dispute.State == DisputeOpen || dispute.State == DisputeReturned {
			report.OpenDisputes++
			disputeIssues = append(disputeIssues, "存在未解决争议："+dispute.ID)
		}
	}
	report.Checks = append(report.Checks, ValidationCheck{Name: "dispute_resolution", Passed: len(disputeIssues) == 0, Summary: fmt.Sprintf("未解决争议 %d 项", report.OpenDisputes), Issues: disputeIssues})
	report.Issues = append(report.Issues, disputeIssues...)

	licenseIssues := []string{}
	if a.Batch.LicenseStatement == "" {
		licenseIssues = append(licenseIssues, "授权声明缺失")
	}
	report.Checks = append(report.Checks, ValidationCheck{Name: "license_statement", Passed: len(licenseIssues) == 0, Summary: "批次授权声明校验", Issues: licenseIssues})
	report.Issues = append(report.Issues, licenseIssues...)

	report.ObjectFailures = len(objectIssues)
	report.Checks = append(report.Checks, ValidationCheck{Name: "object_integrity", Passed: len(objectIssues) == 0, Summary: fmt.Sprintf("对象完整性失败 %d 项", len(objectIssues)), Issues: append([]string(nil), objectIssues...)})
	report.Issues = append(report.Issues, objectIssues...)
	report.Ready = len(report.Issues) == 0
	return report
}
