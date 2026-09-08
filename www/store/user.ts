import { GetPlatformInfoResponse } from '@/lib/pb/api_user'
import { User } from '@/lib/pb/common'
import { atom } from 'nanostores'
import { persistentAtom } from '@nanostores/persistent'

export const $userInfo = atom<User | undefined>()
export const $statusOnline = atom<boolean>(false)
export const $platformInfo = atom<GetPlatformInfoResponse | undefined>()

// 创建持久化的语言设置
export const $language = persistentAtom<string>('user-language', 'zh', {
  encode: JSON.stringify,
  decode: JSON.parse,
})

export type FrontendPreference = {
  githubProxyUrl?: string
  useServerGithubProxyUrl?: boolean
  clientApiUrl?: string
  clientRpcUrl?: string
}

export const $frontendPreference = persistentAtom<FrontendPreference>('frontend_preference', {}, {
  encode: JSON.stringify,
  decode: JSON.parse,
})
