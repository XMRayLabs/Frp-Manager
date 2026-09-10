import http from '@/api/http'
import { API_PATH } from '@/lib/consts'
export type LanguageGroup = { id: string; name: string; shared: boolean }
export type OrganizationContext = { user_id: number; role: string; language_group_id: string }
export type Assignment = { language_group_id: string; server_id: string }
export type Audit = { id: number; actor_id: number; action: string; target: string; detail: string; created_at: string }
export async function organization<T>(path: string, body?: unknown): Promise<T> {
  const response = body === undefined ? await http.get(`${API_PATH}/organization/${path}`) : await http.post(`${API_PATH}/organization/${path}`, body)
  return response.data.body ?? response.data.data
}
