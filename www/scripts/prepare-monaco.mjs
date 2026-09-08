import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const source = resolve(root, 'node_modules', 'monaco-editor', 'min', 'vs')
const target = resolve(root, 'public', 'vendor', 'monaco', 'vs')

if (!existsSync(source)) {
  throw new Error(`Missing local Monaco assets: ${source}`)
}

rmSync(target, { force: true, recursive: true })
mkdirSync(dirname(target), { recursive: true })
cpSync(source, target, { recursive: true })
console.log(`Prepared local Monaco assets in ${target}`)
