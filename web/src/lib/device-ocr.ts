/**
 * Tier 3 of the reading chain (docs/05): recognise a bill photo's text on this device with
 * Tesseract.js. The engine and English model are served by the app (public/tesseract) and,
 * once used, cached by the service worker, so this also works offline.
 */

export interface DeviceText {
  text: string
  /** Mean word confidence, 0–100. */
  confidence: number
}

export async function readOnDevice(image: Blob, onProgress?: (fraction: number) => void): Promise<DeviceText> {
  const { createWorker, OEM } = await import('tesseract.js')
  const worker = await createWorker('eng', OEM.LSTM_ONLY, {
    workerPath: '/tesseract/worker.min.js',
    corePath: '/tesseract',
    langPath: '/tesseract',
    gzip: true,
    workerBlobURL: false, // the content security policy allows workers from this origin only
    logger: (m: { status: string; progress: number }) => {
      // Loading the model is ~20%; recognising the page the rest.
      if (m.status === 'recognizing text') onProgress?.(0.2 + 0.8 * m.progress)
      else if (m.status.startsWith('loading')) onProgress?.(0.2 * m.progress)
    },
  })
  try {
    const { data } = await worker.recognize(image)
    return { text: data.text, confidence: data.confidence }
  } finally {
    await worker.terminate()
  }
}
