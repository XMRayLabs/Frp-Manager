import http from '@/api/http'
import { API_PATH } from '@/lib/consts'
import { GetClientsStatusRequest, GetClientsStatusResponse } from '@/lib/pb/api_master'
import { GetPlatformInfoResponse } from '@/lib/pb/api_user'
import { RespCode } from '@/lib/pb/common'
import { BaseResponse } from '@/types/api'
import { mapBatches } from '@/lib/map-batches'

export const getPlatformInfo = async () => {
  const res = await http.get(API_PATH + '/platform/baseinfo')
  return GetPlatformInfoResponse.fromJson((res.data as BaseResponse).body)
}

export const getClientsStatus = async (req: GetClientsStatusRequest): Promise<GetClientsStatusResponse> => {
  const responses = await mapBatches([...new Set(req.clientIds)], async (clientIds) => {
    const res = await http.post(
      API_PATH + '/platform/clientsstatus',
      GetClientsStatusRequest.toJson({ ...req, clientIds }),
    )
    const response = GetClientsStatusResponse.fromJson((res.data as BaseResponse).body)
    if (response.status?.code !== RespCode.SUCCESS) throw new Error(response.status?.message || '节点状态查询失败')
    return response
  })
  return {
    status: { code: RespCode.SUCCESS, message: 'ok' },
    clients: Object.assign({}, ...responses.map((response) => response.clients)),
  }
}

export interface NodeOverview {
  pendingIds?: string[] | null
  total: number
  online: number
  pending: number
  unconfigured: number
  invalid: number
  unavailable: number
  upgrade: number
}

export const getNodeOverview = async (): Promise<{ clients: NodeOverview; servers: NodeOverview }> => {
  const res = await http.get(API_PATH + '/platform/overview')
  return res.data.body
}
