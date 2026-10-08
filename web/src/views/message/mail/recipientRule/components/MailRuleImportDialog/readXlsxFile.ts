export function readXlsxFile(file: File, signal: AbortSignal): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    const cleanup = () => {
      signal.removeEventListener('abort', cancel)
      reader.onload = null
      reader.onerror = null
      reader.onabort = null
    }
    const fail = () => {
      cleanup()
      reject(new Error('Excel file could not be read'))
    }
    const cancel = () => {
      cleanup()
      if (reader.readyState === FileReader.LOADING) reader.abort()
      reject(new DOMException('File reading cancelled', 'AbortError'))
    }
    reader.onerror = fail
    reader.onabort = fail
    reader.onload = () => {
      if (!(reader.result instanceof ArrayBuffer)) {
        fail()
        return
      }
      try {
        const bytes = new Uint8Array(reader.result)
        let binary = ''
        for (let index = 0; index < bytes.length; index += 32768)
          binary += String.fromCharCode(...bytes.subarray(index, index + 32768))
        cleanup()
        resolve(btoa(binary))
      } catch {
        fail()
      }
    }
    if (signal.aborted) cancel()
    else {
      signal.addEventListener('abort', cancel, { once: true })
      try {
        reader.readAsArrayBuffer(file)
      } catch {
        fail()
      }
    }
  })
}
