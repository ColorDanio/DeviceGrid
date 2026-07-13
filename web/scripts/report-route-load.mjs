import { readFileSync } from 'node:fs'
import { gzipSync } from 'node:zlib'

const dist = new URL('../dist/', import.meta.url)
const manifest = JSON.parse(readFileSync(new URL('.vite/manifest.json', dist), 'utf8'))

function size(file) { return gzipSync(readFileSync(new URL(file, dist))).length }
function closure(key, seen = new Set()) {
  if (seen.has(key)) return seen
  seen.add(key)
  for (const imported of manifest[key]?.imports || []) closure(imported, seen)
  return seen
}

for (const [key, entry] of Object.entries(manifest)) {
  if (!key.startsWith('src/views/') || !key.endsWith('.vue')) continue
  const files = [...closure(key)]
  const js = files.reduce((total, imported) => total + (manifest[imported]?.file.endsWith('.js') ? size(manifest[imported].file) : 0), 0)
  const css = files.reduce((total, imported) => total + (manifest[imported]?.css || []).reduce((sum, file) => sum + size(file), 0), 0)
  console.log(`${entry.name}: ${(js / 1024).toFixed(1)} KiB JS, ${(css / 1024).toFixed(1)} KiB CSS gzip`)
}
