import { getClientsStatus } from '@/api/platform'
import { ClientStatus } from '@/lib/pb/api_master'
import { ClientType } from '@/lib/pb/common'

const CLIENT_STATUS_CACHE_MS = 8000

type PendingRequest = {
  resolve: (status: ClientStatus | undefined) => void
  reject: (error: unknown) => void
}

type CachedStatus = {
  status: ClientStatus | undefined
  expiresAt: number
}

const pending = new Map<ClientType, Map<string, PendingRequest[]>>()
const scheduled = new Set<ClientType>()
const cache = new Map<string, CachedStatus>()

function cacheKey(type: ClientType, id: string) {
  return `${type}:${id}`
}

export function getClientStatus(type: ClientType, id: string): Promise<ClientStatus | undefined> {
  const key = cacheKey(type, id)
  const cached = cache.get(key)
  if (cached && cached.expiresAt > Date.now()) {
    return Promise.resolve(cached.status)
  }

  return new Promise((resolve, reject) => {
    let requests = pending.get(type)
    if (!requests) {
      requests = new Map()
      pending.set(type, requests)
    }
    requests.set(id, [...(requests.get(id) ?? []), { resolve, reject }])

    if (!scheduled.has(type)) {
      scheduled.add(type)
      setTimeout(() => void flush(type), 0)
    }
  })
}

async function flush(type: ClientType) {
  scheduled.delete(type)
  const requests = pending.get(type)
  pending.delete(type)
  if (!requests || requests.size === 0) return

  const ids = [...requests.keys()]
  try {
    const response = await getClientsStatus({ clientIds: ids, clientType: type })
    const expiresAt = Date.now() + CLIENT_STATUS_CACHE_MS
    for (const id of ids) {
      const status = response.clients[id]
      cache.set(cacheKey(type, id), { status, expiresAt })
      for (const request of requests.get(id) ?? []) {
        request.resolve(status)
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
