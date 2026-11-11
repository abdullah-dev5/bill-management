package retrieval

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const defaultPITCDuplicateBillURL = "https://usersnap.pitc.com.pk/api/SnapsForDuplicateBill/ToDuplicate"

// HESCO fetches PITC-format duplicate bills for HESCO references. PITC returns bill pages as
// base64-encoded JPEGs, so the connector turns them into one PDF for the normal extraction path.
type HESCO struct {
	Client   *http.Client
	Endpoint string
}

func (h HESCO) fetchWebBill(ctx context.Context, req Request, meterJPEG []byte) (Document, bool) {
	const endpoint = "https://bill.pitc.com.pk/hescobill"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Document{}, false
	}
	httpReq.Header.Set("Accept", "text/html")
	httpReq.Header.Set("User-Agent", "Bill Management official retrieval")
	resp, err := h.Client.Do(httpReq)
	if err != nil {
		return Document{}, false
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	cookies := resp.Cookies()
	resp.Body.Close()
	if err != nil {
		return Document{}, false
	}
	form := url.Values{}
	form.Set("rbSearchByList", "refno")
	form.Set("searchTextBox", req.Reference)
	form.Set("ruCodeTextBox", "")
	form.Set("btnSearch", "Search")
	for _, match := range hiddenInput.FindAllSubmatch(page, -1) {
		form.Set(string(match[1]), string(match[2]))
	}
	for _, match := range hiddenInputReversed.FindAllSubmatch(page, -1) {
		form.Set(string(match[1]), string(match[2]))
	}
	body := strings.NewReader(form.Encode())
	httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return Document{}, false
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Referer", endpoint)
	if len(cookies) > 0 {
		values := make([]string, 0, len(cookies))
		for _, cookie := range cookies {
			values = append(values, cookie.Name+"="+cookie.Value)
		}
		httpReq.Header.Set("Cookie", strings.Join(values, "; "))
	}
	resp, err = h.Client.Do(httpReq)
	if err != nil {
		return Document{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Document{}, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil || !bytes.Contains(data, []byte("PAYABLE")) {
		return Document{}, false
	}
	data = preparePITCBillHTML(data, meterJPEG)
	return Document{
		Data: data, ContentType: "text/html",
		FileName: "hesco-official-" + req.Reference + "-" + req.Period + ".html",
	}, true
}

var hiddenInput = regexp.MustCompile(`(?i)<input[^>]+type=["']hidden["'][^>]+name=["']([^"']+)["'][^>]+value=["']([^"']*)["']`)
var hiddenInputReversed = regexp.MustCompile(`(?i)<input[^>]+name=["']([^"']+)["'][^>]+type=["']hidden["'][^>]+value=["']([^"']*)["']`)
var relativeAsset = regexp.MustCompile(`(?i)(src|href)=["']([^"'#][^"']*)["']`)
var noscriptBlock = regexp.MustCompile(`(?is)<noscript\b[^>]*>.*?</noscript\s*>`)
var refreshMeta = regexp.MustCompile(`(?is)<meta\b[^>]+http-equiv\s*=\s*["']?refresh["']?[^>]*>`)
var loaderContainer = regexp.MustCompile(`(?is)<div\b[^>]+id=["']loader-container["'][^>]*>.*?</div\s*>\s*</div\s*>\s*`)
var bodyClose = regexp.MustCompile(`(?is)</body\s*>`)

func preparePITCBillHTML(data, meterJPEG []byte) []byte {
	data = noscriptBlock.ReplaceAll(data, nil)
	data = refreshMeta.ReplaceAll(data, nil)
	data = loaderContainer.ReplaceAll(data, nil)
	if len(meterJPEG) > 0 {
		meter := base64.StdEncoding.EncodeToString(meterJPEG)
		section := []byte(`<section class="bill-management-meter-photo" style="break-inside: avoid; margin: 18px auto; max-width: 720px; text-align: center;"><h3>Meter photograph</h3><img src="data:image/jpeg;base64,` + meter + `" alt="Meter photograph" style="max-width: 100%; height: auto;"></section>`)
		if match := bodyClose.FindIndex(data); match != nil {
			data = append(append(append([]byte{}, data[:match[0]]...), section...), data[match[0]:]...)
		}
	}
	return relativeAsset.ReplaceAllFunc(data, func(match []byte) []byte {
		parts := relativeAsset.FindSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		asset := string(parts[2])
		if strings.HasPrefix(asset, "http://") || strings.HasPrefix(asset, "https://") ||
			strings.HasPrefix(asset, "data:") || strings.HasPrefix(asset, "mailto:") ||
			strings.HasPrefix(asset, "javascript:") {
			return match
		}
		asset = strings.TrimPrefix(asset, "./")
		asset = strings.TrimPrefix(asset, "/")
		return []byte(string(parts[1]) + `="https://bill.pitc.com.pk/` + asset + `"`)
	})
}

func NewHESCO(client *http.Client, endpoint string) HESCO {
	if client == nil {
		client = http.DefaultClient
	}
	if endpoint == "" {
		endpoint = defaultPITCDuplicateBillURL
	}
	return HESCO{Client: client, Endpoint: endpoint}
}

func (HESCO) ID() string          { return "hesco-pitc" }
func (HESCO) Name() string        { return "HESCO (PITC official bills)" }
func (HESCO) Providers() []string { return []string{"HESCO"} }

type pitcResponse struct {
	Status int          `json:"STATUS"`
	Data   []pitcRecord `json:"DATA"`
}

type pitcRecord struct {
	Reference string `json:"REF_NO"`
	Month     string `json:"BILL_MONTH"`
	Snap1     string `json:"SNAP_1"`
	Snap2     string `json:"SNAP_2"`
	Snap3     string `json:"SNAP_3"`
	Snap4     string `json:"SNAP_4"`
	Snap5     string `json:"SNAP_5"`
	Snap6     string `json:"SNAP_6"`
	Snap7     string `json:"SNAP_7"`
	Snap8     string `json:"SNAP_8"`
}

func (h HESCO) Fetch(ctx context.Context, req Request) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	if req.Reference == "" {
		return Document{}, errors.New("HESCO reference is empty")
	}
	period, err := time.Parse("2006-01", req.Period)
	if err != nil {
		return Document{}, fmt.Errorf("invalid HESCO billing period %q: %w", req.Period, err)
	}
	body, err := json.Marshal(map[string]string{
		"REF_NO":     req.Reference,
		"BILL_MONTH": period.Format("02-Jan-06"),
	})
	if err != nil {
		return Document{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Document{}, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Referer", "https://bill.pitc.com.pk/")
	httpReq.Header.Set("User-Agent", "Bill Management official retrieval")
	resp, err := h.Client.Do(httpReq)
	if err != nil {
		return Document{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Document{}, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Document{}, fmt.Errorf("HESCO source returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Document{}, err
	}
	var result pitcResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return Document{}, fmt.Errorf("invalid HESCO response: %w", err)
	}
	if result.Status != 1 || len(result.Data) == 0 {
		return Document{}, ErrNotFound
	}
	record := result.Data[0]
	if record.Reference != "" && record.Reference != req.Reference {
		return Document{}, fmt.Errorf("HESCO response reference does not match request")
	}
	if record.Month != "" {
		returnedMonth, err := time.Parse("01/02/2006 15:04:05", record.Month)
		if err != nil {
			return Document{}, fmt.Errorf("invalid HESCO bill month %q: %w", record.Month, err)
		}
		if returnedMonth.Format("2006-01") != req.Period {
			return Document{}, fmt.Errorf("HESCO response month %q does not match request %q", returnedMonth.Format("2006-01"), req.Period)
		}
	}
	encoded := []string{record.Snap1, record.Snap2, record.Snap3, record.Snap4, record.Snap5, record.Snap6, record.Snap7, record.Snap8}
	pages := make([][]byte, 0, len(encoded))
	for _, value := range encoded {
		value = strings.TrimSpace(value)
		if value == "" || value == "null" {
			continue
		}
		page, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return Document{}, fmt.Errorf("invalid HESCO bill image: %w", err)
		}
		if _, err := jpeg.DecodeConfig(bytes.NewReader(page)); err != nil {
			return Document{}, fmt.Errorf("HESCO bill page is not a JPEG: %w", err)
		}
		pages = append(pages, page)
	}
	if len(pages) == 0 {
		return Document{}, ErrNotFound
	}
	// PITC's duplicate-bill API currently returns only the meter snapshot in
	// SNAP_1. Use the official web-bill page for the actual consumer bill.
	if strings.Contains(h.Endpoint, "usersnap.pitc.com.pk") {
		if bill, ok := h.fetchWebBill(ctx, req, pages[0]); ok {
			return bill, nil
		}
		return Document{}, fmt.Errorf("HESCO official bill page could not be loaded")
	}
	pdf, err := jpegPDF(pages)
	if err != nil {
		return Document{}, err
	}

	if len(pages) == 1 {
		return Document{
			Data: pages[0], ContentType: "image/jpeg",
			FileName: "hesco-official-" + req.Reference + "-" + req.Period + ".jpg",
		}, nil
	}

	return Document{
		Data: pdf, ContentType: "application/pdf",
		FileName: "hesco-official-" + req.Reference + "-" + req.Period + ".pdf",
	}, nil
}
