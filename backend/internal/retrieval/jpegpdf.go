package retrieval

import (
	"bytes"
	"fmt"
	"image/jpeg"
)

// jpegPDF wraps JPEG pages in a small PDF without re-encoding the images.
func jpegPDF(pages [][]byte) ([]byte, error) {
	type object struct {
		body []byte
	}
	kids := make([]int, 0, len(pages))
	objects := []object{{body: []byte("<< /Type /Catalog /Pages 2 0 R >>")}, {}}
	for i := range pages {
		pageNumber := 3 + i*3
		imageNumber := pageNumber + 1
		contentNumber := pageNumber + 2
		kids = append(kids, pageNumber)
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(pages[i]))
		if err != nil {
			return nil, err
		}
		width, height := float64(cfg.Width), float64(cfg.Height)
		scale := 595.0 / width
		if height*scale > 842 {
			scale = 842.0 / height
		}
		w, h := width*scale, height*scale
		stream := fmt.Sprintf("q\n%.2f 0 0 %.2f %.2f %.2f cm\n/Im1 Do\nQ\n", w, h, (595-w)/2, (842-h)/2)
		objects = append(objects,
			object{body: []byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 %d 0 R >> >> /Contents %d 0 R >>", imageNumber, contentNumber))},
			object{body: []byte(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n%s\nendstream", cfg.Width, cfg.Height, len(pages[i]), pages[i]))},
			object{body: []byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))},
		)
	}
	kidRefs := bytes.NewBuffer(nil)
	for _, kid := range kids {
		fmt.Fprintf(kidRefs, "%d 0 R ", kid)
	}
	objects[1] = object{body: []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kidRefs.String(), len(kids)))}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(obj.body)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes(), nil
}
