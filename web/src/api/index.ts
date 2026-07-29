import client from './client'

// ---------- 类型 ----------
export interface User {
  id: string
  name: string
  role: string
  dept: string
}

export interface Employee {
  id: string
  name: string
  email: string
  phone: string
  dept: string
  position: string
  role: string
  status: string
  must_change_pwd: boolean
}

export interface LoginResult {
  token: string
  user: Employee
}

export interface PageData<T> {
  list: T[]
  total: number
  page: number
  size: number
}

// ---------- 认证 ----------
export const apiLogin = (email: string, password: string) =>
  client.post<unknown, LoginResult>('/auth/login', { email, password })

export const apiMe = () => client.get<unknown, User>('/auth/me')

export const apiChangePassword = (old_password: string, new_password: string) =>
  client.post<unknown, { message: string }>('/auth/change-password', { old_password, new_password })

// ---------- 员工 ----------
export const apiListEmployees = (keyword?: string) =>
  client.get<unknown, PageData<Employee>>('/employees', { params: { keyword } })

export interface EmployeeInput {
  name: string
  phone: string
  dept: string
  position: string
  role: string
}

export const apiCreateEmployee = (data: EmployeeInput & { email: string }) =>
  client.post<unknown, { employee: Employee; initial_password: string; message: string }>('/employees', data)

export const apiUpdateEmployee = (id: string, data: EmployeeInput) =>
  client.put<unknown, { message: string }>(`/employees/${id}`, data)

export const apiSetEmployeeStatus = (id: string, active: boolean) =>
  client.post<unknown, { message: string }>(`/employees/${id}/status`, { active })

export const apiResetEmployeePassword = (id: string) =>
  client.post<unknown, { initial_password: string; message: string }>(`/employees/${id}/reset-password`)

// ---------- 数据字典 ----------
export interface Dict {
  id: string
  type: string
  value: string
  sort: number
}

export const apiListDicts = (type: string) =>
  client.get<unknown, Dict[]>('/dicts', { params: { type } })

export const apiAddDict = (type: string, value: string) =>
  client.post<unknown, Dict>('/dicts', { type, value })

export const apiRemoveDict = (id: string) =>
  client.delete<unknown, { message: string }>(`/dicts/${id}`)

// ---------- 客户 ----------
export interface Customer {
  id: string
  code: string
  name: string
  industry: string
  source: string
  level: string
  remark: string
  owner_id: string
  owner?: Employee
  created_at: string
}

export interface CustomerInput {
  name: string
  industry: string
  source: string
  level: string
  remark: string
}

export interface CustomerQuery {
  page?: number
  size?: number
  keyword?: string
  industry?: string
  source?: string
  level?: string
  owner_id?: string
}

export const apiListCustomers = (params: CustomerQuery) =>
  client.get<unknown, PageData<Customer>>('/customers', { params })

export const apiGetCustomer = (id: string) => client.get<unknown, Customer>(`/customers/${id}`)

export const apiCreateCustomer = (data: CustomerInput) =>
  client.post<unknown, Customer>('/customers', data)

export const apiUpdateCustomer = (id: string, data: CustomerInput) =>
  client.put<unknown, { message: string }>(`/customers/${id}`, data)

export const apiDeleteCustomer = (id: string) =>
  client.delete<unknown, { message: string }>(`/customers/${id}`)

export const apiTransferCustomer = (id: string, owner_id: string) =>
  client.post<unknown, { message: string }>(`/customers/${id}/transfer`, { owner_id })

// ---------- 联系人 ----------
export interface Contact {
  id: string
  customer_id: string
  name: string
  phone: string
  email: string
  position: string
  is_primary: boolean
  remark: string
}

export interface ContactInput {
  name: string
  phone: string
  email: string
  position: string
  is_primary: boolean
  remark: string
}

export const apiListContacts = (customerId: string) =>
  client.get<unknown, Contact[]>(`/customers/${customerId}/contacts`)

export const apiCreateContact = (customerId: string, data: ContactInput) =>
  client.post<unknown, Contact>(`/customers/${customerId}/contacts`, data)

export const apiUpdateContact = (customerId: string, contactId: string, data: ContactInput) =>
  client.put<unknown, { message: string }>(`/customers/${customerId}/contacts/${contactId}`, data)

export const apiDeleteContact = (customerId: string, contactId: string) =>
  client.delete<unknown, { message: string }>(`/customers/${customerId}/contacts/${contactId}`)

// ---------- 商单 ----------
export const DEAL_STAGES: Record<string, { label: string; color: string }> = {
  prospecting: { label: '线索', color: 'default' },
  qualifying: { label: '需求确认', color: 'cyan' },
  proposal: { label: '方案报价', color: 'blue' },
  negotiating: { label: '谈判中', color: 'geekblue' },
  won: { label: '赢单', color: 'success' },
  lost: { label: '输单', color: 'error' },
}

// 与后端 dealFlow 一致的可选目标阶段（含回退边）
export const DEAL_FLOW: Record<string, string[]> = {
  prospecting: ['qualifying', 'lost'],
  qualifying: ['proposal', 'prospecting', 'lost'],
  proposal: ['negotiating', 'qualifying', 'lost'],
  negotiating: ['won', 'proposal', 'lost'],
  won: [],
  lost: [],
}

export const LOST_REASONS = [
  { value: 'no_purchase', label: '客户不买了' },
  { value: 'competitor', label: '输给竞品' },
  { value: 'budget', label: '预算不足' },
  { value: 'qualified_out', label: '我方主动放弃' },
  { value: 'other', label: '其他' },
]

export interface Deal {
  id: string
  code: string
  customer_id: string
  customer?: Customer
  title: string
  amount_cent: number
  probability: number
  expected_sign_date: string
  status: string
  lost_reason: string
  owner_id: string
  owner?: Employee
  remark: string
  created_at: string
}

export interface DealInput {
  customer_id: string
  title: string
  amount_cent: number
  probability?: number
  expected_sign_date?: string
  remark?: string
}

export interface DealQuery {
  page?: number
  size?: number
  keyword?: string
  status?: string
  customer_id?: string
}

export interface ForecastResult {
  open_count: number
  total_cent: number
  weighted_cent: number
}

export const apiListDeals = (params: DealQuery) =>
  client.get<unknown, PageData<Deal>>('/deals', { params })

export const apiGetDeal = (id: string) => client.get<unknown, Deal>(`/deals/${id}`)

export const apiCreateDeal = (data: DealInput) => client.post<unknown, Deal>('/deals', data)

export const apiUpdateDeal = (id: string, data: DealInput) =>
  client.put<unknown, { message: string }>(`/deals/${id}`, data)

export const apiChangeDealStatus = (id: string, to: string, lost_reason?: string, force?: boolean) =>
  client.post<unknown, Deal>(`/deals/${id}/status`, { to, lost_reason, force })

export const apiDeleteDeal = (id: string) =>
  client.delete<unknown, { message: string }>(`/deals/${id}`)

export const apiDealForecast = () => client.get<unknown, ForecastResult>('/deals/forecast')

// 金额展示：分 → 元字符串
export const fenToYuan = (cent: number) => (cent / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })
