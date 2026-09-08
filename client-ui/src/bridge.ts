import { Capacitor, registerPlugin } from '@capacitor/core'
import type { BridgeAction, ClientProfile, FrpManagerBridge, RuntimeStatus } from './types'

const storageKey = 'frp-manager-client-ui:profiles'

type AndroidPlugin = {
  getRuntimeStatus: () => Promise<RuntimeStatus>
  openPowerSettings: () => Promise<void>
  runAction: (options: { action: BridgeAction; profile: ClientProfile }) => Promise<{
    ok: boolean
    message: string
    output?: string
    status?: RuntimeStatus
  }>
}

const androidPlugin = registerPlugin<AndroidPlugin>('FrpManager')
const desktopBridge = (): FrpManagerBridge | undefined => window.frpManager
const isAndroid = () => Capacitor.isNativePlatform() && Capacitor.getPlatform() === 'android'

export async function getProfiles(): Promise<ClientProfile[]> {
  const bridge = desktopBridge()
  if (bridge) return bridge.getProfiles()
  return readLocalProfiles()
}

export async function saveProfile(profile: ClientProfile): Promise<ClientProfile[]> {
  const bridge = desktopBridge()
  if (bridge) return bridge.saveProfile(profile)
  const profiles = readLocalProfiles()
  const next = [profile, ...profiles.filter((item) => item.id !== profile.id)]
  localStorage.setItem(storageKey, JSON.stringify(next))
  return next
}

export async function deleteProfile(id: string): Promise<ClientProfile[]> {
  const bridge = desktopBridge()
  if (bridge) return bridge.deleteProfile(id)
  const next = readLocalProfiles().filter((profile) => profile.id !== id)
  localStorage.setItem(storageKey, JSON.stringify(next))
  return next
}

export async function getRuntimeStatus(): Promise<RuntimeStatus> {
  const bridge = desktopBridge()
  if (bridge) return bridge.getRuntimeStatus()
  if (isAndroid()) return androidPlugin.getRuntimeStatus()
  return {
    platform: 'web',
    serviceStatus: 'native bridge unavailable',
  }
}

export async function chooseBinary(): Promise<string | null> {
  const bridge = desktopBridge()
  if (bridge) return bridge.chooseBinary()
  return null
}

export async function runAction(action: BridgeAction, profile: ClientProfile) {
  const bridge = desktopBridge()
  if (bridge) return bridge.runAction(action, profile)
  if (isAndroid()) return androidPlugin.runAction({ action, profile })
  return {
    ok: false,
    message: `${action} requires the desktop or mobile native bridge for ${profile.name}.`,
  }
}

export async function openDataDir(): Promise<void> {
  const bridge = desktopBridge()
  if (bridge) await bridge.openDataDir()
}

export async function openPowerSettings(): Promise<void> {
  if (isAndroid()) await androidPlugin.openPowerSettings()
}

function readLocalProfiles(): ClientProfile[] {
  try {
    const raw = localStorage.getItem(storageKey)
    return raw ? (JSON.parse(raw) as ClientProfile[]) : []
  } catch {
    return []
  }
}
