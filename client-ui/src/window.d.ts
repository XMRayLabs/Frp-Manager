import type { FrpManagerBridge } from './types'

declare global {
  interface Window {
    frpManager?: FrpManagerBridge
  }
}

export {}
