// Package extracttest has helpers for tests that need sample documents.
package extracttest

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
)

// TextPDF builds a minimal PDF whose text layer holds lines (one per row).
func TextPDF(lines []string, pages int) []byte {
	var content bytes.Buffer
	content.WriteString("BT /F1 9 Tf 40 820 Td 11 TL\n")
	for _, l := range lines {
		l = strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(l)
		fmt.Fprintf(&content, "(%s) Tj T*\n", l)
	}
	content.WriteString("ET")

	var objs []string
	kids := ""
	for i := 0; i < pages; i++ {
		kids += fmt.Sprintf("%d 0 R ", 4+i)
	}
	objs = append(objs,
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, pages),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	)
	contentObj := 4 + pages
	for i := 0; i < pages; i++ {
		objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", contentObj))
	}
	objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()))

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// HescoText is the text of a HESCO (PITC-format) bill with personal details replaced.
//
//go:embed hesco-masked.txt
var HescoText string

// LatinLines keeps the ASCII part of each non-empty line (the test PDF font is Latin-only).
func LatinLines(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		var b strings.Builder
		for _, r := range l {
			if r < 128 {
				b.WriteRune(r)
			}
		}
		if s := strings.TrimSpace(b.String()); s != "" {
			out = append(out, s)
		}
	}
	return out
}
