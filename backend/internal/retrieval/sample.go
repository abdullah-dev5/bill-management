package retrieval

import (
	"context"
	"regexp"
	"strings"

	"github.com/abdullah-dev5/bill-management/backend/internal/extract/extracttest"
)

// Sample is a stand-in connector for development and tests: it "retrieves" the masked HESCO
// sample bill for that bill's reference number and nothing else. It makes no network requests.
// Never enabled in production.
type Sample struct{}

// SampleReference is the reference number printed on the sample bill.
const SampleReference = "09123450012345"

func (Sample) ID() string          { return "sample" }
func (Sample) Name() string        { return "Sample bills (development)" }
func (Sample) Providers() []string { return []string{"HESCO"} }

var nonASCII = regexp.MustCompile(`[^\x20-\x7e]`)

func (Sample) Fetch(ctx context.Context, req Request) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	if req.Reference != SampleReference {
		return Document{}, ErrNotFound
	}
	var lines []string
	for _, l := range strings.Split(extracttest.HescoText, "\n") {
		if l = strings.TrimSpace(nonASCII.ReplaceAllString(l, "")); l != "" {
			lines = append(lines, l)
		}
	}
	return Document{Data: extracttest.TextPDF(lines, 1), ContentType: "application/pdf", FileName: "hesco-official-" + req.Reference + ".pdf"}, nil
}
