import * as z from 'zod'
import { Client, Server } from './pb/common'
import { GetPlatformInfoResponse } from './pb/api_user'
import { TypedProxyConfig } from '@/types/proxy'

// 寤惰繜鍔犺浇鍓嶇棣栭€夐」锛岄伩鍏?SSR 鏈熼棿寮曠敤 window/localStorage
type FrontendPreferenceLazy = {
  githubProxyUrl?: string
  useServerGithubProxyUrl?: boolean
  clientApiUrl?: string
  clientRpcUrl?: string
}

const getFrontendPreference = (): FrontendPreferenceLazy => {
  if (typeof window !== 'undefined') {
    try {
      // 鍔ㄦ€佸紩鍏ラ伩鍏嶆墦鍖呮椂闈欐€佷緷璧?
      // eslint-disable-next-line
      const { $frontendPreference } = require('@/store/user')
      return ($frontendPreference.get?.() ?? {}) as FrontendPreferenceLazy
    } catch {
      return {}
    }
  }
  return {}
}

export const API_PATH = '/api/v1'
export const X_CLIENT_REQUEST_ID = 'x-client-request-id'
export const REPOSITORY_URL = 'https://github.com/Sakurame1/frp-manager'
export const RELEASES_URL = `${REPOSITORY_URL}/releases`
export const LATEST_RELEASE_URL = `${REPOSITORY_URL}/releases/latest`
export const LATEST_DOWNLOADS = {
  windowsGui: `${REPOSITORY_URL}/releases/latest/download/frp-manager-client-windows-x64.exe`,
  macosGuiX64: `${REPOSITORY_URL}/releases/latest/download/frp-manager-client-macos-x64.dmg`,
  macosGuiArm64: `${REPOSITORY_URL}/releases/latest/download/frp-manager-client-macos-arm64.dmg`,
  androidGui: `${REPOSITORY_URL}/releases/latest/download/frp-manager-client-android.apk`,
  windowsCoreX64: `${REPOSITORY_URL}/releases/latest/download/frp-manager-windows-amd64.exe`,
  linuxCoreX64: `${REPOSITORY_URL}/releases/latest/download/frp-manager-linux-amd64`,
  linuxCoreArm64: `${REPOSITORY_URL}/releases/latest/download/frp-manager-linux-arm64`,
  macosCoreX64: `${REPOSITORY_URL}/releases/latest/download/frp-manager-darwin-amd64`,
  macosCoreArm64: `${REPOSITORY_URL}/releases/latest/download/frp-manager-darwin-arm64`,
} as const
export const ZodPortSchema = z.coerce
  .number({ required_error: 'validation.required' })
  .min(1, { message: 'validation.portRange.min' })
  .max(65535, { message: 'validation.portRange.max' })

export const ZodPortOptionalSchema = z.coerce
  .number({ required_error: 'validation.required' })
  .min(1, { message: 'validation.portRange.min' })
  .max(65535, { message: 'validation.portRange.max' })

export const ZodIPSchema = z
  .string({ required_error: 'validation.required' })
  .regex(/^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/, { message: 'validation.ipAddress' })
export const ZodStringSchema = z
  .string({ required_error: 'validation.required' })
  .min(1, { message: 'validation.required' })

export const ZodStringOptionalSchema = z.string().optional()
export const ZodEmailSchema = z
  .string({ required_error: 'validation.required' })
  .min(1, { message: 'validation.required' })
  .email({ message: 'auth.email.invalid' })

export const ConnectionProtocols = ['tcp', 'kcp', 'quic', 'websocket', 'wss']

export const TypedProxyConfigValid = (typedProxyCfg: TypedProxyConfig | undefined): boolean => {
  if (!typedProxyCfg) {
    return false
  }

  if (typedProxyCfg.plugin && typedProxyCfg.plugin.type) {
    if (typedProxyCfg.type === 'tcp' || typedProxyCfg.type === 'udp') {
      if (!typedProxyCfg.remotePort) {
        console.log('remotePort is undefined')
        return false
      }
    }
    return typedProxyCfg.name && typedProxyCfg.type ? true : false
  }

  if (typedProxyCfg.type === 'tcp' || typedProxyCfg.type === 'udp') {
    if (!typedProxyCfg.remotePort) {
      console.log('remotePort is undefined')
      return false
    }
  }

  return typedProxyCfg?.localPort && typedProxyCfg.localIP && typedProxyCfg.name && typedProxyCfg.type ? true : false
}

export const IsIDValid = (clientID: string | undefined): boolean => {
  if (clientID == undefined) {
    return false
  }
  const regex = /^[a-zA-Z0-9-_]+$/
  return clientID.length > 0 && regex.test(clientID)
}

export const ClientConfigured = (client: Client | undefined): boolean => {
  if (client == undefined) {
    return false
  }
  return !(
    (client.config == undefined || client.config == '') &&
    (client.clientIds == undefined || client.clientIds.length == 0)
  )
}

// .refine((e) => e === "abcd@fg.com", "This email is not in our database")

// 鑾峰彇鏈€缁?Github 浠ｇ悊 URL
const getGithubProxyUrl = (info: GetPlatformInfoResponse, applyPref = true): string => {
  const pref = getFrontendPreference()
  if (applyPref && pref.useServerGithubProxyUrl === false && pref.githubProxyUrl) {
    return pref.githubProxyUrl
  }
  // 鑻ュ墠绔湭鎸囧畾鎴栭€夋嫨浣跨敤鏈嶅姟鍣紝杩斿洖鍚庣
  return info.githubProxyUrl
}

// 鑾峰彇鏈€缁?API URL
const getClientApiUrl = (info: GetPlatformInfoResponse, applyPref = true): string => {
  const pref = getFrontendPreference()
  return applyPref && pref.clientApiUrl?.trim() ? pref.clientApiUrl.trim() : info.clientApiUrl
}

// 鑾峰彇鏈€缁?RPC URL
const getClientRpcUrl = (info: GetPlatformInfoResponse, applyPref = true): string => {
  const pref = getFrontendPreference()
  return applyPref && pref.clientRpcUrl?.trim() ? pref.clientRpcUrl.trim() : info.clientRpcUrl
}

const ConnectionArgsStr = <T extends { id?: string; secret?: string }>(
  type: 'client' | 'server',
  item: T,
  info: GetPlatformInfoResponse,
  applyPref = true,
) => {
  const apiUrl = getClientApiUrl(info, applyPref)
  const rpcUrl = getClientRpcUrl(info, applyPref)
  return `${type} -s ${item.secret} -i ${item.id} --api-url ${apiUrl} --rpc-url ${rpcUrl}`
}

export const ExecCommandStr = <T extends { id?: string; secret?: string }>(
  type: 'client' | 'server',
  item: T,
  info: GetPlatformInfoResponse,
  fileName = 'frp-manager',
  applyPref = true,
) => {
  return `${fileName} ${ConnectionArgsStr(type, item, info, applyPref)}`
}

export const JoinCommandStr = (info: GetPlatformInfoResponse, token: string, fileName?: string, clientID?: string, applyPref = true, role: 'client' | 'server' = 'client') => {
  const apiUrl = getClientApiUrl(info, applyPref)
  const rpcUrl = getClientRpcUrl(info, applyPref)
  return `${fileName || 'frp-manager'} ${role}${clientID ? ` -i ${clientID}` : ''} -j ${token} --api-url ${apiUrl} --rpc-url ${rpcUrl}`
}

export const WindowsInstallCommand = <T extends { id?: string; secret?: string }>(
  type: 'client' | 'server',
  item: T,
  info: GetPlatformInfoResponse,
  _githubProxy?: boolean,
  applyPref = true,
) => {
  return (
    `[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12;` +
    `$installer=Join-Path $env:TEMP 'frp-manager-install.ps1';` +
    `Invoke-WebRequest https://raw.githubusercontent.com/XMRayLabs/Frp-Manager/main/install.ps1 -OutFile $installer;` +
    `powershell.exe -NoProfile -ExecutionPolicy Bypass -File $installer -Version latest ${ConnectionArgsStr(type, item, info, applyPref)}`
  )
}

export const LinuxInstallCommand = <T extends { id?: string; secret?: string }>(
  type: 'client' | 'server',
  item: T,
  info: GetPlatformInfoResponse,
  _githubProxy?: boolean,
  applyPref = true,
) => {
  return `curl -fSL https://raw.githubusercontent.com/XMRayLabs/Frp-Manager/main/install.sh | bash -s -- --version latest ${ConnectionArgsStr(type, item, info, applyPref)}`
}

export const ClientEnvFile = <T extends Client | Server>(item: T, info: GetPlatformInfoResponse, applyPref = true) => {
  const apiUrl = getClientApiUrl(info, applyPref)
  const rpcUrl = getClientRpcUrl(info, applyPref)
  return `CLIENT_ID=${item.id}
CLIENT_SECRET=${item.secret}
CLIENT_API_URL=${apiUrl}
CLIENT_RPC_URL=${rpcUrl}`
}
