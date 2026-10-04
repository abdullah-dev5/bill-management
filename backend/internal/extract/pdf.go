package extract

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFText returns the text layer of a PDF and its page count. Scanned PDFs have
// little or no text; the caller decides whether that's enough.
func PDFText(data []byte) (text string, pages int, err error) {
	defer func() {
		if p := recover(); p != nil { // the parser panics on some malformed files
			err = fmt.Errorf("pdf: unreadable file: %v", p)
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", 0, fmt.Errorf("pdf: %w", err)
	}
	pages = r.NumPage()
	var b strings.Builder
	for i := 1; i <= pages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		rows, err := p.GetTextByRow()
		if err != nil {
			continue
		}
		for _, row := range rows {
			for j, w := range row.Content {
				if j > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(w.S)
			}
			b.WriteByte('\n')
		}
	}
	return b.String(), pages, nil
}

// PDFPageCount counts pages without extracting text.
func PDFPageCount(data []byte) (pages int, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("pdf: unreadable file: %v", p)
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, fmt.Errorf("pdf: %w", err)
	}
	return r.NumPage(), nil
}
