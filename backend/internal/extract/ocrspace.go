package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// OCRSpaceMaxBytes is the free plan's file size limit.
const OCRSpaceMaxBytes = 1 << 20

// OCRSpace turns a bill image into text with the OCR.space API; our parsers then read the fields.
type OCRSpace struct {
	APIKey   string
	Endpoint string // default https://api.ocr.space/parse/image
	HTTP     *http.Client
}

func (o *OCRSpace) Name() string { return "ocrspace" }

// Text returns the OCR text of the document.
func (o *OCRSpace) Text(ctx context.Context, in Input) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fileType := map[string]string{"image/jpeg": "JPG", "image/png": "PNG", "image/webp": "WEBP", "application/pdf": "PDF"}[in.ContentType]
	for k, v := range map[string]string{"language": "eng", "OCREngine": "2", "scale": "true", "isTable": "true", "filetype": fileType} {
		if err := mw.WriteField(k, v); err != nil {
			return "", err
		}
	}
	name := in.FileName
	if name == "" {
		name = "bill"
	}
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(in.Data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	endpoint := o.Endpoint
	if endpoint == "" {
		endpoint = "https://api.ocr.space/parse/image"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("apikey", o.APIKey)
	resp, err := httpClient(o.HTTP).Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTransient, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	msg := strings.TrimSpace(string(data))
	// The free plan answers rate limits with 403 and a plain-text message.
	if resp.StatusCode == http.StatusForbidden && strings.Contains(strings.ToLower(msg), "maximum") {
		return "", fmt.Errorf("%w: %s", ErrRateLimited, msg)
	}
	if err := classifyStatus(resp.StatusCode, msg); err != nil {
		return "", err
	}

	var parsed struct {
		ParsedResults []struct {
			ParsedText string `json:"ParsedText"`
		} `json:"ParsedResults"`
		IsErroredOnProcessing bool            `json:"IsErroredOnProcessing"`
		ErrorMessage          json.RawMessage `json:"ErrorMessage"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	if parsed.IsErroredOnProcessing {
		return "", fmt.Errorf("ocrspace: %s", ocrSpaceError(parsed.ErrorMessage))
	}
	var b strings.Builder
	for _, p := range parsed.ParsedResults {
		b.WriteString(p.ParsedText)
		b.WriteByte('\n')
	}
	return strings.ReplaceAll(b.String(), "\r\n", "\n"), nil
}

func ocrSpaceError(raw json.RawMessage) string {
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return strings.Join(list, "; ")
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}
