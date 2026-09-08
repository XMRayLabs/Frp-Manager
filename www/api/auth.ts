import http from '@/api/http'
import { API_PATH } from '@/lib/consts'
import { LoginRequest, LoginResponse, RegisterRequest, RegisterResponse } from '@/lib/pb/api_auth'
import { CommonResponse } from '@/lib/pb/common'
import { BaseResponse } from '@/types/api'

export const login = async (req: LoginRequest) => {
  const res = await http.post(API_PATH + '/auth/login', LoginRequest.toJson(req))
  return LoginResponse.fromJson((res.data as BaseResponse).body)
}

export type RegisterWithInviteRequest = RegisterRequest & {
  inviteCode?: string
}

export const register = async (req: RegisterWithInviteRequest) => {
  const res = await http.post(API_PATH + '/auth/register', {
    username: req.username,
    password: req.password,
    email: req.email,
    invite_code: req.inviteCode,
  })
  return RegisterResponse.fromJson((res.data as BaseResponse).body)
}

export const logout = async () => {
  const res = await http.get(API_PATH + '/auth/logout')
  return CommonResponse.fromJson((res.data as BaseResponse).body)
}
