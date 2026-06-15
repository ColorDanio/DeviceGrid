/**
 * Decode base64 string to Uint8Array.
 * This is critical for UTF-8: atob() returns a Latin-1 string where each
 * character is one byte, which breaks multi-byte UTF-8 sequences.
 * Using Uint8Array preserves raw bytes so xterm.js can decode UTF-8 correctly.
 */
export function decodeBase64(b64: string): Uint8Array {
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}
