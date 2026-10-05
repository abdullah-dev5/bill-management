package httpapi

import (
	"net/http"
	"testing"

	"github.com/abdullah-dev5/bill-management/backend/internal/extract/extracttest"
)

type deviceExtraction struct {
	Status string `json:"status"`
	Error  *string
	Fields map[string]struct {
		Value      string  `json:"value"`
		Confidence float64 `json:"confidence"`
	} `json:"fields"`
	Tiers []struct {
		Tier, Outcome string
	} `json:"tiers"`
}

func TestExtractFromDeviceText(t *testing.T) {
	e := newEnv(t)
	cl, _ := e.signedUp("DeviceOCR")
	doc := cl.upload(fakeJPEG, "image/jpeg", nil)
	path := "/api/v1/documents/" + doc.Id.String() + "/extraction/device"

	var got deviceExtraction
	cl.must(http.StatusOK, http.MethodPost, path, map[string]any{"text": extracttest.HescoText, "confidence": 91}, &got)
	if got.Status != "succeeded" || got.Fields["payableMinor"].Value != "326600" || got.Fields["referenceNumber"].Value != "09123450012345" ||
		got.Fields["providerName"].Value != "HESCO" || len(got.Tiers) != 1 || got.Tiers[0].Tier != "device" {
		t.Fatalf("extraction = %+v", got)
	}
	if _, ok := got.Fields["customerName"]; ok {
		t.Error("the customer's name must not be stored")
	}
	confident := got.Fields["payableMinor"].Confidence

	// The same text recognised with low confidence: every field is less certain.
	cl.must(http.StatusOK, http.MethodPost, path, map[string]any{"text": extracttest.HescoText, "confidence": 35}, &got)
	if c := got.Fields["payableMinor"].Confidence; c >= confident {
		t.Errorf("low OCR confidence should lower field confidence: %.2f vs %.2f", c, confident)
	}

	// GET returns the latest one.
	var latest deviceExtraction
	cl.must(http.StatusOK, http.MethodGet, "/api/v1/documents/"+doc.Id.String()+"/extraction", nil, &latest)
	if latest.Status != "succeeded" || latest.Tiers[0].Tier != "device" {
		t.Errorf("latest = %+v", latest)
	}

	// Nothing bill-like: a failed extraction with a helpful message.
	cl.must(http.StatusOK, http.MethodPost, path, map[string]any{"text": "a blurry photo of a cat"}, &got)
	if got.Status != "failed" || got.Error == nil {
		t.Errorf("garbage = %+v", got)
	}
	cl.must(http.StatusBadRequest, http.MethodPost, path, map[string]any{"text": "   "}, nil)

	other, _ := e.signedUp("DeviceOCROther")
	other.must(http.StatusNotFound, http.MethodPost, path, map[string]any{"text": extracttest.HescoText}, nil)
}
