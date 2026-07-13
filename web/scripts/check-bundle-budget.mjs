import { readdirSync, readFileSync } from 'node:fs'
import process from 'node:process'
import { gzipSync } from 'node:zlib'

const limits = [
  { label: 'initial JavaScript', pattern: /^client-.*\.js$/, maxGzipBytes: 370_000 },
  { label: 'initial CSS', pattern: /^index-.*\.css$/, maxGzipBytes: 60_000 },
]

const assetsDir = new URL('../dist/assets/', import.meta.url)
const assets = readdirSync(assetsDir)
let failed = false

for (const limit of limits) {
  const asset = assets.find((name) => limit.pattern.test(name))
  if (!asset) {
    console.error(`Missing ${limit.label} asset matching ${limit.pattern}`)
    failed = true
    continue
  }

  const gzipBytes = gzipSync(readFileSync(new URL(asset, assetsDir))).length
  const result = `${limit.label}: ${(gzipBytes / 1024).toFixed(1)} KiB gzip (limit ${(limit.maxGzipBytes / 1024).toFixed(1)} KiB)`
  if (gzipBytes > limit.maxGzipBytes) {
    console.error(`Bundle budget exceeded: ${result}`)
    failed = true
  } else {
    console.log(result)
  }
}

if (failed) process.exit(1)
