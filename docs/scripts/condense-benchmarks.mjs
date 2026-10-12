// Usage: node scripts/condense-benchmarks.mjs <data.js> <history.json>
import { readFileSync, writeFileSync } from 'node:fs'
import { condense } from '../.vitepress/theme/benchmarkHistory.mjs'

const [input, output] = process.argv.slice(2)
const source = readFileSync(input, 'utf8').replace(/^\s*window\.BENCHMARK_DATA\s*=\s*/, '').replace(/;?\s*$/, '')
const history = condense(JSON.parse(source))
writeFileSync(output, JSON.stringify(history))
console.log(`${output}: ${history.series.length} benchmarks, ${history.runs.length} runs`)
