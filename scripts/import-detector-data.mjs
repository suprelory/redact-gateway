// Reproduce the reviewed detector data; runtime detection has no Node dependency.
import { readFileSync, writeFileSync, mkdirSync, copyFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { resolve, dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { runInNewContext } from 'node:vm'

const source = process.argv[2]
if (!source) throw new Error('Usage: node scripts/import-detector-data.mjs <CosyRedactGateway checkout>')
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const checkout = resolve(source)
const revision = execFileSync('git', ['-C', checkout, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim()
const pinnedRevision = '0e2be2e3ba7fcbe982942a941e3a2fd1b89e87f6'
if (revision !== pinnedRevision) throw new Error('Review a new source revision before changing the pinned importer')
execFileSync('git', ['-C', checkout, 'diff', 'HEAD', '--exit-code', '--', 'worker.js', 'test/words.json', 'LICENSE', 'NOTICE', 'docs/THIRD_PARTY_NOTICES.md'])
const worker = readFileSync(join(checkout, 'worker.js'), 'utf8')
const model = JSON.parse(worker.match(/^const BIGRAM_COST = (.*);$/m)[1])
const thresholds = JSON.parse(worker.match(/^const ENTROPY_THRESHOLDS = (.*);$/m)[1])
const definitions = worker.slice(worker.indexOf('function regexEscape('), worker.indexOf('export const GITLEAK_PORTABLE_RULE_COUNT'))
const definitionsJSON = runInNewContext(definitions + `
JSON.stringify(GITLEAK_RULES.map(rule => ({
  id: rule.id, pattern: rule.regex.source, flags: rule.regex.flags,
  secret_group: rule.secretGroup, entropy: rule.entropy, keywords: rule.keywords,
  allow_regexes: rule.allowRegexes.map(re => ({pattern: re.source, flags: re.flags})),
  stopwords: rule.stopwords,
})))`, {}, { timeout: 1000, contextCodeGeneration: { strings: false, wasm: false } })
const re2 = ({ pattern, flags }) => {
  if (/[^gims]/.test(flags)) throw new Error('Unsupported regex flags: ' + flags)
  const modes = [...'ims'].filter(flag => flags.includes(flag)).join('')
  return (modes ? '(?' + modes + ')' : '') + pattern.replace(/\\u([a-fA-F0-9]{4})/g, '\\x{$1}')
}
const rules = JSON.parse(definitionsJSON).map(rule => ({
  id: rule.id, pattern: re2(rule), secret_group: rule.secret_group,
  entropy: rule.entropy, keywords: rule.keywords,
  allow_regexes: rule.allow_regexes.map(re2), stopwords: rule.stopwords,
}))
if (rules.length !== 218) throw new Error('Unexpected rule count')
const dataDir = join(root, 'internal/redact/data')
const fixtureDir = join(root, 'internal/redact/testdata')
const noticesDir = join(root, 'third_party/cosy-redact-gateway')
for (const directory of [dataDir, fixtureDir, noticesDir]) mkdirSync(directory, { recursive: true })
const provenance = { source: 'https://github.com/CassiopeiaCode/CosyRedactGateway', revision,
  modifications: 'Extracted for Go/RE2; see docs/detection.md for execution semantics.' }
writeFileSync(join(dataDir, 'entropy-model.json'), JSON.stringify({ ...provenance, thresholds, costs: model }, null, 2) + '\n')
writeFileSync(join(dataDir, 'gitleaks-rules.json'), JSON.stringify({ ...provenance, rules }, null, 2) + '\n')
copyFileSync(join(checkout, 'test/words.json'), join(fixtureDir, 'entropy-words.json'))
for (const file of ['LICENSE', 'NOTICE']) copyFileSync(join(checkout, file), join(noticesDir, file))
copyFileSync(join(checkout, 'docs/THIRD_PARTY_NOTICES.md'), join(noticesDir, 'GITLEAKS.md'))
console.log('Imported entropy model, calibration words, and ' + rules.length + ' credential rules from ' + revision)
