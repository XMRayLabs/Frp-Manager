import http from '@/api/http'
import { API_PATH } from '@/lib/consts'
import { BaseResponse } from '@/types/api'

const unwrap = (res: any) => (res.data as BaseResponse).body ?? (res.data as any).data

export interface InviteCode {
  id: number
  code: string
  tenant_id: number
  created_by: number
  max_uses: number
  used_count: number
  expires_at?: string
  disabled: boolean
  comment?: string
}

export const listInvites = async (): Promise<InviteCode[]> => {
  const res = await http.post(API_PATH + '/permission/invite/list', {})
  return unwrap(res) ?? []
}

export const createInvite = async (req: { code?: string; max_uses: number; expires_at?: number; comment?: string }) => {
  const res = await http.post(API_PATH + '/permission/invite/create', req)
  return unwrap(res)
}

export const updateInvite = async (req: { id: number; disabled: boolean }) => {
  const res = await http.post(API_PATH + '/permission/invite/update', req)
  return unwrap(res)
}

export interface RegisterSetting {
  register_enabled: boolean
  invite_required: boolean
}

export const getRegisterSetting = async (): Promise<RegisterSetting> => {
  const res = await http.post(API_PATH + '/permission/register-setting/get', {})
  return unwrap(res) ?? { register_enabled: false, invite_required: true }
}

export const updateRegisterSetting = async (setting: Partial<RegisterSetting>) => {
  const res = await http.post(API_PATH + '/permission/register-setting/update', setting)
  return unwrap(res)
}

export interface AdminUser {
  user_id: number
  user_name: string
  email: string
  status: number
  role: string
}

export interface UserGroup {
  group_id: string
  group_name: string
  comment?: string
  users?: AdminUser[]
}

export interface ResourcePermission {
  target_type: 'user' | 'group'
  target_id: string
  target_name: string
  permission: 'view' | 'edit'
}

export const listUsers = async (): Promise<AdminUser[]> => {
  const res = await http.post(API_PATH + '/permission/user/list', {})
  return unwrap(res) ?? []
}

export const updateUser = async (req: Partial<AdminUser> & { user_id: number }) => {
  const res = await http.post(API_PATH + '/permission/user/update', req)
  return unwrap(res)
}

export const listGroups = async (): Promise<UserGroup[]> => {
  const res = await http.post(API_PATH + '/permission/group/list', {})
  return unwrap(res) ?? []
}

export const listResourcePermissions = async (req: { obj_type: string; obj_id: string }): Promise<ResourcePermission[]> => {
  const res = await http.post(API_PATH + '/permission/resource/permissions', req)
  return unwrap(res) ?? []
}

export const grantPermission = async (req: { obj_type: string; obj_id: string; target_type: 'user' | 'group'; target_id: string; permission: 'view' | 'edit' }) => {
  const res = await http.post(API_PATH + '/permission/grant', req)
  return unwrap(res)
}

export const revokePermission = async (req: { obj_type: string; obj_id: string; target_type: 'user' | 'group'; target_id: string; permission: 'view' | 'edit' }) => {
  const res = await http.post(API_PATH + '/permission/revoke', req)
  return unwrap(res)
}
