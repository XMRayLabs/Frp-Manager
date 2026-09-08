import { getProxyStatuses } from '@/api/proxy'
import { ProxyStatusResult } from '@/lib/pb/api_client'

const PROXY_STATUS_CACHE_MS = 8000

type PendingRequest = {
  resolve: (result: ProxyStatusResult | undefined) => void
  reject: (error: unknown) => void
}

const pending = new Map<number, PendingRequest[]>()
const cache = new Map<number, { result: ProxyStatusResult | undefined; expiresAt: number }>()
let scheduled = false

export function getProxyStatus(proxyId: number): Promise<ProxyStatusResult | undefined> {
  const cached = cache.get(proxyId)
  if (cached && cached.expiresAt > Date.now()) {
    return Promise.resolve(cached.result)
  }

  return new Promise((resolve, reject) => {
    pending.set(proxyId, [...(pending.get(proxyId) ?? []), { resolve, reject }])
    if (!scheduled) {
      scheduled = true
      setTimeout(() => void flush(), 0)
    }
  })
}

async function flush() {
  scheduled = false
  const requests = new Map(pending)
  pending.clear()
  const ids = [...requests.keys()]
  if (ids.length === 0) return

  try {
    const response = await getProxyStatuses({ proxyIds: ids })
    const results = new Map(response.proxyStatuses.map((result) => [result.proxyId ?? 0, result]))
    const expiresAt = Date.now() + PROXY_STATUS_CACHE_MS
    for (const id of ids) {
      const result = results.get(id)
      cache.set(id, { result, expiresAt })
      for (const request of requests.get(id) ?? []) {
        request.resolve(result)
      }
    }
  } catch (error) {
    for (const requestGroup of requests.values()) {
      for (const request of requestGroup) {
        request.reject(error)
      }
    }
  }
}
