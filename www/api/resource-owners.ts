import http from '@/api/http'
import { API_PATH } from '@/lib/consts'

export interface ResourceOwners {
  owners: { id: number; name: string; group_id?: string; group_name?: string }[]
  resources: Record<string, number>
}
export async function getResourceOwners(kind: 'client' | 'proxy'): Promise<ResourceOwners> {
  const response = await http.get(API_PATH + '/platform/owners', { params: { kind } })
  return response.data.body
}
