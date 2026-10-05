// Copies the on-device OCR engine (Tesseract.js worker, WebAssembly core, English model) into
// public/tesseract so it is served by the app itself: the content security policy allows no
// other origins, and no outside service sees the user's bills. Runs after `pnpm install`.
import { copyFileSync, existsSync, mkdirSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const require = createRequire(import.meta.url)
const root = dirname(dirname(fileURLToPath(import.meta.url)))
const out = join(root, 'public', 'tesseract')
mkdirSync(out, { recursive: true })

const pkgDir = (name) => dirname(require.resolve(`${name}/package.json`, { paths: [root, dirname(require.resolve('tesseract.js/package.json'))] }))
const tesseract = pkgDir('tesseract.js')
const core = pkgDir('tesseract.js-core')
const eng = pkgDir('@tesseract.js-data/eng')

const files = [
  [join(tesseract, 'dist', 'worker.min.js'), 'worker.min.js'],
  // LSTM-only builds (the engine we use), for each CPU feature level the worker may pick.
  [join(core, 'tesseract-core-lstm.wasm.js'), 'tesseract-core-lstm.wasm.js'],
  [join(core, 'tesseract-core-simd-lstm.wasm.js'), 'tesseract-core-simd-lstm.wasm.js'],
  [join(core, 'tesseract-core-relaxedsimd-lstm.wasm.js'), 'tesseract-core-relaxedsimd-lstm.wasm.js'],
  // The compact "best_int" English model (~3 MB).
  [join(eng, '4.0.0_best_int', 'eng.traineddata.gz'), 'eng.traineddata.gz'],
]
for (const [from, to] of files) {
  if (!existsSync(from)) throw new Error(`missing ${from}`)
  copyFileSync(from, join(out, to))
}
console.log(`on-device OCR files copied to ${out}`)
