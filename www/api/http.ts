import { X_CLIENT_REQUEST_ID } from '@/lib/consts'
import axios from 'axios'
import { v4 as uuidv4 } from 'uuid'

const instance = axios.create({ withCredentials: true, timeout: 20_000 })

instance.interceptors.request.use((request) => {
  request.headers[X_CLIENT_REQUEST_ID] = uuidv4()
  return request
})

instance.interceptors.response.use(
  (response) => {
    if (response.data.code != 200) {
      throw new Error(response.data.msg || 'Request failed')
    }
    return response
  },
  (error) => {
    if (axios.isAxiosError(error) && error.response?.status === 403 && error.response.data?.body?.mustChangePassword && typeof window !== 'undefined' && window.location.pathname !== '/change-password') {
      window.location.replace('/change-password')
    }
    if (axios.isAxiosError(error) && error.response?.data?.msg) error.message = error.response.data.msg

    if (
      axios.isAxiosError(error) &&
      error.response?.status === 401 &&
      typeof window !== 'undefined' &&
      window.location.pathname !== '/login'
    ) {
      window.location.assign('/login')
    }
    return Promise.reject(error)
  },
)

export default instance
