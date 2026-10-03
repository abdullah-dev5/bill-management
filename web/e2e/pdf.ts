import { readFileSync } from 'node:fs'

/** A minimal PDF whose text layer holds the given lines (same layout as the Go test helper). */
export function textPdf(lines: string[], pages = 1): Buffer {
  const esc = (s: string) => s.replace(/\\/g, '\\\\').replace(/\(/g, '\\(').replace(/\)/g, '\\)')
  const content = 'BT /F1 9 Tf 40 820 Td 11 TL\n' + lines.map((l) => `(${esc(l)}) Tj T*\n`).join('') + 'ET'
  const kids = Array.from({ length: pages }, (_, i) => `${4 + i} 0 R`).join(' ')
  const contentObj = 4 + pages
  const objs = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    `<< /Type /Pages /Kids [${kids}] /Count ${pages} >>`,
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>',
    ...Array.from(
      { length: pages },
      () => `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents ${contentObj} 0 R >>`,
    ),
    `<< /Length ${content.length} >>\nstream\n${content}\nendstream`,
  ]
  let out = '%PDF-1.4\n'
  const offsets: number[] = []
  objs.forEach((o, i) => {
    offsets.push(out.length)
    out += `${i + 1} 0 obj\n${o}\nendobj\n`
  })
  const xref = out.length
  out += `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n` + offsets.map((o) => `${String(o).padStart(10, '0')} 00000 n \n`).join('')
  out += `trailer\n<< /Size ${objs.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`
  return Buffer.from(out, 'latin1')
}

/** The masked HESCO sample bill, English parts only (the PDF font is Latin). */
export function hescoLines(): string[] {
  const text = readFileSync(new URL('../../backend/internal/extract/extracttest/hesco-masked.txt', import.meta.url), 'utf8')
  return text
    .split('\n')
    .map((l) => l.replace(/[^\x20-\x7e]/g, '').trim())
    .filter(Boolean)
}
