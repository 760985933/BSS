import axios, { AxiosError } from 'axios'
import { message } from 'antd'

const TOKEN_KEY = 'bss_token'

export const getToken = () => localStorage.getItem(TOKEN_KEY)
export const setToken = (t: string) => localStorage.setItem(TOKEN_KEY, t)
export const clearToken = () => localStorage.removeItem(TOKEN_KEY)

interface Resp<T = unknown> {
  code: number
  data?: T
  message?: string
  warning?: boolean
}

const client = axios.create({ baseURL: '/api/v1', timeout: 15000 })

// 请求拦截：注入 JWT
client.interceptors.request.use((config) => {
  const t = getToken()
  if (t) config.headers.Authorization = `Bearer ${t}`
  return config
})

// 响应拦截：统一拆包 {code, data, message}；401 清会话跳登录
client.interceptors.response.use(
  (res) => {
    const body = res.data as Resp
    if (body && typeof body.code === 'number') {
      if (body.code === 0) return body.data as never
      message.error(body.message || '请求失败')
      return Promise.reject(body)
    }
    return res.data
  },
  (err: AxiosError<Resp>) => {
    if (err.response?.status === 401) {
      clearToken()
      if (!location.pathname.startsWith('/login')) {
        location.href = '/login'
      }
    }
    // 软校验 warning（如商单退出标准）不弹全局错误，交由页面弹确认框处理
    if (!err.response?.data?.warning) {
      message.error(err.response?.data?.message || '网络异常，请稍后重试')
    }
    return Promise.reject(err.response?.data || err)
  },
)

export default client
