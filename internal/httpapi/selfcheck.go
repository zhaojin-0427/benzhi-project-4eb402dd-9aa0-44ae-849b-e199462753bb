package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

type selfcheckEnvelope struct {
	Data  json.RawMessage                 `json:"data"`
	Error *struct{ Code, Message string } `json:"error,omitempty"`
}
type selfcheckBatch struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Version int64  `json:"version"`
}

func RunSelfCheck(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	baseURL = strings.TrimRight(baseURL, "/")
	sampleStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	batchID := "selfcheck-batch"
	create := map[string]any{"id": batchID, "title": "春季林地声景自检", "siteBoundary": map[string]any{"description": "自检样地", "north": 31.2, "south": 31.1, "east": 121.6, "west": 121.5}, "sampleWindow": map[string]any{"start": sampleStart, "end": sampleStart.Add(24 * time.Hour)}, "licenseStatement": "仅用于科研数据治理自检，已获得采样授权", "requiredAnnotators": 2}
	var batch selfcheckBatch
	if err := selfcheckJSON(ctx, client, "POST", baseURL+"/api/v1/batches", create, selfcheckHeaders("selfcheck-admin", "administrator", 0, "sc-create"), &batch); err != nil {
		return fmt.Errorf("创建批次: %w", err)
	}
	if batch.Version != 1 || batch.State != "draft" {
		return fmt.Errorf("创建状态断言失败: %+v", batch)
	}
	var upload struct {
		Batch selfcheckBatch `json:"batch"`
		Clip  struct {
			ID, SHA256 string
			ByteSize   int64
		} `json:"clip"`
	}
	fields := map[string]string{"clipId": "clip-001", "recordedAt": sampleStart.Add(time.Hour).Format(time.RFC3339Nano), "durationMillis": "5000", "recorderCode": "REC-SC", "habitatNote": "针阔混交林", "mediaType": "audio/wav"}
	if err := selfcheckMultipart(ctx, client, baseURL+"/api/v1/batches/"+batchID+"/clips", fields, []byte("RIFF\x10\x00\x00\x00WAVEfmt selfcheck-audio-content"), selfcheckHeaders("selfcheck-admin", "administrator", batch.Version, "sc-upload"), &upload); err != nil {
		return fmt.Errorf("上传片段: %w", err)
	}
	batch = upload.Batch
	if upload.Clip.SHA256 == "" || upload.Clip.ByteSize == 0 {
		return fmt.Errorf("录音摘要断言失败")
	}
	var annotation struct {
		Batch selfcheckBatch `json:"batch"`
	}
	first := map[string]any{"id": "ann-001", "speciesCode": "PARUS_MAJOR", "startMillis": 500, "endMillis": 1600, "confidence": 0.93, "evidenceNote": "三段式鸣声清晰"}
	if err := selfcheckJSON(ctx, client, "POST", baseURL+"/api/v1/batches/"+batchID+"/clips/clip-001/annotations", first, selfcheckHeaders("annotator-one", "annotator", batch.Version, "sc-ann-1"), &annotation); err != nil {
		return fmt.Errorf("提交第一份标注: %w", err)
	}
	batch = annotation.Batch
	second := map[string]any{"id": "ann-002", "speciesCode": "CINEREOUS_TIT", "startMillis": 900, "endMillis": 2300, "confidence": 0.61, "evidenceNote": "低频音节疑似煤山雀"}
	if err := selfcheckJSON(ctx, client, "POST", baseURL+"/api/v1/batches/"+batchID+"/clips/clip-001/annotations", second, selfcheckHeaders("annotator-two", "annotator", batch.Version, "sc-ann-2"), &annotation); err != nil {
		return fmt.Errorf("提交第二份标注: %w", err)
	}
	batch = annotation.Batch
	var consensus struct {
		Batch    selfcheckBatch `json:"batch"`
		Disputes []struct {
			ID          string   `json:"id"`
			ReasonCodes []string `json:"reasonCodes"`
		} `json:"disputes"`
	}
	if err := selfcheckJSON(ctx, client, "POST", baseURL+"/api/v1/batches/"+batchID+"/consensus", nil, selfcheckHeaders("selfcheck-admin", "administrator", batch.Version, "sc-consensus"), &consensus); err != nil {
		return fmt.Errorf("计算共识: %w", err)
	}
	batch = consensus.Batch
	if batch.State != "awaiting_arbitration" || len(consensus.Disputes) != 1 {
		return fmt.Errorf("争议生成断言失败")
	}
	disputeID := consensus.Disputes[0].ID
	var evidence struct {
		Evidence []struct {
			Alias string `json:"alias"`
		} `json:"evidence"`
	}
	if err := selfcheckGETWithHeaders(ctx, client, baseURL+"/api/v1/batches/"+batchID+"/disputes/"+disputeID, map[string]string{"X-Actor-ID": "taxonomy-expert", "X-Role": "arbiter"}, &evidence); err != nil {
		return fmt.Errorf("查询匿名证据: %w", err)
	}
	if len(evidence.Evidence) != 2 || evidence.Evidence[0].Alias == "annotator-one" {
		return fmt.Errorf("匿名证据断言失败")
	}
	decision := map[string]any{"finalSpeciesCode": "PARUS_MAJOR", "rationale": "频谱主峰和节奏符合大山雀，采用第一份分类", "returnForRevision": false}
	var arbitration struct {
		Batch selfcheckBatch `json:"batch"`
	}
	if err := selfcheckJSON(ctx, client, "POST", baseURL+"/api/v1/batches/"+batchID+"/disputes/"+disputeID+"/arbitrate", decision, selfcheckHeaders("taxonomy-expert", "arbiter", batch.Version, "sc-arbitrate"), &arbitration); err != nil {
		return fmt.Errorf("仲裁争议: %w", err)
	}
	batch = arbitration.Batch
	var freeze struct {
		Batch    selfcheckBatch `json:"batch"`
		Manifest struct {
			Digest string `json:"digest"`
		} `json:"manifest"`
	}
	if err := selfcheckJSON(ctx, client, "POST", baseURL+"/api/v1/batches/"+batchID+"/freeze", nil, selfcheckHeaders("selfcheck-admin", "administrator", batch.Version, "sc-freeze"), &freeze); err != nil {
		return fmt.Errorf("冻结批次: %w", err)
	}
	batch = freeze.Batch
	if batch.State != "frozen" || freeze.Manifest.Digest == "" {
		return fmt.Errorf("冻结清单断言失败")
	}
	var publish struct {
		Batch       selfcheckBatch                                    `json:"batch"`
		Certificate struct{ ManifestDigest, VerificationCode string } `json:"certificate"`
	}
	if err := selfcheckJSON(ctx, client, "POST", baseURL+"/api/v1/batches/"+batchID+"/publish", nil, selfcheckHeaders("release-owner", "publisher", batch.Version, "sc-publish"), &publish); err != nil {
		return fmt.Errorf("发布批次: %w", err)
	}
	if publish.Batch.State != "published" || publish.Certificate.ManifestDigest != freeze.Manifest.Digest || publish.Certificate.VerificationCode == "" {
		return fmt.Errorf("发布证书断言失败")
	}
	var events []map[string]any
	if err := selfcheckGET(ctx, client, baseURL+"/api/v1/batches/"+batchID+"/events", &events); err != nil {
		return fmt.Errorf("查询事件: %w", err)
	}
	if len(events) != 8 {
		return fmt.Errorf("事件数量断言失败，得到 %d", len(events))
	}
	var certificate map[string]any
	if err := selfcheckGET(ctx, client, baseURL+"/api/v1/batches/"+batchID+"/certificate", &certificate); err != nil {
		return fmt.Errorf("验证证书查询: %w", err)
	}
	return nil
}

func selfcheckHeaders(actor, role string, version int64, key string) map[string]string {
	return map[string]string{"X-Actor-ID": actor, "X-Role": role, "X-Expected-Version": fmt.Sprint(version), "X-Request-ID": "request-" + key, "Idempotency-Key": key}
}
func selfcheckJSON(ctx context.Context, client *http.Client, method, url string, body any, headers map[string]string, target any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return selfcheckDo(client, request, target)
}
func selfcheckGET(ctx context.Context, client *http.Client, url string, target any) error {
	return selfcheckGETWithHeaders(ctx, client, url, nil, target)
}

func selfcheckGETWithHeaders(ctx context.Context, client *http.Client, url string, headers map[string]string, target any) error {
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-Request-ID", "selfcheck-query")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return selfcheckDo(client, request, target)
}
func selfcheckMultipart(ctx context.Context, client *http.Client, url string, fields map[string]string, content []byte, headers map[string]string, target any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	fileHeader := textproto.MIMEHeader{}
	fileHeader.Set("Content-Disposition", `form-data; name="audio"; filename="selfcheck.wav"`)
	fileHeader.Set("Content-Type", "audio/wav")
	part, err := writer.CreatePart(fileHeader)
	if err != nil {
		return err
	}
	if _, err = part.Write(content); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", url, &body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return selfcheckDo(client, request, target)
}
func selfcheckDo(client *http.Client, request *http.Request, target any) error {
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	var envelope selfcheckEnvelope
	if err = json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("响应不是 JSON: %s", string(data))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if envelope.Error != nil {
			return fmt.Errorf("HTTP %d %s: %s", response.StatusCode, envelope.Error.Code, envelope.Error.Message)
		}
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if target != nil && len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, target)
	}
	return nil
}
