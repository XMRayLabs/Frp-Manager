import type { ClientVersion } from '@/lib/pb/api_master'

type ParsedVersion = {
  numbers: [number, number, number]
  prerelease: boolean
}

function parseVersion(value: string | undefined): ParsedVersion | undefined {
  const match = value?.trim().match(/^v?(\d+)\.(\d+)\.(\d+)(?:-([^+]+))?(?:\+.*)?$/i)
  if (!match) return undefined
  return {
    numbers: [Number(match[1]), Number(match[2]), Number(match[3])],
    prerelease: Boolean(match[4]),
  }
}

// Compatibility for the architecture suffix used by older ARMv7 packages.
export function normalizeClientVersion(value: string | undefined) {
  const text = value?.trim() ?? ''
  const match = text.match(/^v?(\d+\.\d+\.\d+)-armv7(?:-test)?$/i)
  return match ? match[1] : text
}

export function NeedUpgrade(version: ClientVersion | undefined, currentVersion: string | undefined) {
  const installedVersion = normalizeClientVersion(version?.gitVersion)
  if (installedVersion.toLowerCase() === 'main') return true
  const installed = parseVersion(installedVersion)
  const current = parseVersion(normalizeClientVersion(currentVersion))
  if (!installed || !current) return false

  for (let index = 0; index < current.numbers.length; index += 1) {
    if (current.numbers[index] !== installed.numbers[index]) {
      return current.numbers[index] > installed.numbers[index]
    }
  }
  return installed.prerelease && !current.prerelease
}
