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

// ---------- 合同 ----------
export const CONTRACT_STATUS: Record<string, { label: string; color: string }> = {
  draft: { label: '草稿', color: 'default' },
  pending: { label: '待签约', color: 'gold' },
  signed: { label: '已签约', color: 'blue' },
  performing: { label: '履行中', color: 'cyan' },
  completed: { label: '已完成', color: 'success' },
  cancelled: { label: '已取消', color: 'default' },
  terminated: { label: '已终止', color: 'error' },
  expired: { label: '已过期', color: 'orange' },
}

// 与后端 contractFlow 一致的可选目标状态（含回退边）
export const CONTRACT_FLOW: Record<string, string[]> = {
  draft: ['pending', 'cancelled'],
  pending: ['signed', 'draft', 'cancelled'],
  signed: ['performing', 'terminated', 'expired'],
  performing: ['completed', 'terminated', 'expired'],
  completed: [],
  cancelled: [],
  terminated: [],
  expired: [],
}

export interface Contract {
  id: string
  code: string
  customer_id: string
  customer?: Customer
  title: string
  amount_cent: number
  sign_date: string
  start_date: string
  expire_date: string
  status: string
  terminate_reason: string
  owner_id: string
  owner?: Employee
  remark: string
  deals?: Deal[]
  created_at: string
}

export interface ContractInput {
  customer_id: string
  title: string
  amount_cent: number
  sign_date?: string
  start_date?: string
  expire_date?: string
  remark?: string
  deal_ids?: number[]
}

export interface ContractQuery {
  page?: number
  size?: number
  keyword?: string
  status?: string
  customer_id?: string
}

export const apiListContracts = (params: ContractQuery) =>
  client.get<unknown, PageData<Contract>>('/contracts', { params })

export const apiGetContract = (id: string) => client.get<unknown, Contract>(`/contracts/${id}`)

export const apiCreateContract = (data: ContractInput) =>
  client.post<unknown, Contract>('/contracts', data)

export const apiUpdateContract = (id: string, data: ContractInput) =>
  client.put<unknown, { message: string }>(`/contracts/${id}`, data)

export const apiChangeContractStatus = (id: string, to: string, terminate_reason?: string) =>
  client.post<unknown, Contract>(`/contracts/${id}/status`, { to, terminate_reason })

export const apiDeleteContract = (id: string) =>
  client.delete<unknown, { message: string }>(`/contracts/${id}`)

export const apiReplaceContractDeals = (id: string, deal_ids: number[]) =>
  client.put<unknown, { message: string }>(`/contracts/${id}/deals`, { deal_ids })

// ---------- 附件 ----------
export interface Attachment {
  id: string
  entity_type: string
  entity_id: string
  file_name: string
  file_size: number
  mime: string
  uploaded_by: string
  created_at: string
}

export const apiListContractAttachments = (id: string) =>
  client.get<unknown, Attachment[]>(`/contracts/${id}/attachments`)

export const apiUploadContractAttachment = (id: string, file: File) => {
  const fd = new FormData()
  fd.append('file', file)
  return client.post<unknown, Attachment>(`/contracts/${id}/attachments`, fd)
}

export const apiDownloadAttachment = (id: string) =>
  client.get<unknown, Blob>(`/attachments/${id}/download`, { responseType: 'blob' })

export const apiDeleteAttachment = (id: string) =>
  client.delete<unknown, { message: string }>(`/attachments/${id}`)

// ---------- 回款 ----------
export const PLAN_STATUS: Record<string, { label: string; color: string }> = {
  pending: { label: '待收', color: 'default' },
  partial: { label: '部分核销', color: 'processing' },
  paid: { label: '已收', color: 'success' },
}

export const PAY_METHODS = [
  { value: 'bank', label: '银行转账' },
  { value: 'cash', label: '现金' },
  { value: 'check', label: '支票' },
  { value: 'other', label: '其他' },
]

export interface PaymentPlan {
  id: string
  contract_id: string
  period_no: number
  due_date: string
  amount_cent: number
  status: string
  is_overdue: boolean
  created_at: string
}

export interface PaymentRecord {
  id: string
  contract_id: string
  plan_id: string | null
  amount_cent: number
  paid_at: string
  method: string
  remark: string
  created_by: string
  created_at: string
}

export interface PaymentSummary {
  receivable_cent: number
  received_cent: number
  balance_cent: number
  overdue_cent: number
}

export interface PlanInput {
  period_no: number
  due_date: string
  amount_cent: number
}

export interface RecordInput {
  plan_id: string | null
  amount_cent: number
  paid_at: string
  method?: string
  remark?: string
}

export const apiListPlans = (contractId: string) =>
  client.get<unknown, PaymentPlan[]>(`/contracts/${contractId}/plans`)

export const apiCreatePlan = (contractId: string, data: PlanInput) =>
  client.post<unknown, PaymentPlan>(`/contracts/${contractId}/plans`, data)

export const apiUpdatePlan = (contractId: string, planId: string, data: PlanInput) =>
  client.put<unknown, { message: string }>(`/contracts/${contractId}/plans/${planId}`, data)

export const apiDeletePlan = (contractId: string, planId: string) =>
  client.delete<unknown, { message: string }>(`/contracts/${contractId}/plans/${planId}`)

export const apiListRecords = (contractId: string) =>
  client.get<unknown, PaymentRecord[]>(`/contracts/${contractId}/records`)

export const apiCreateRecords = (contractId: string, records: RecordInput[]) =>
  client.post<unknown, { message: string }>(`/contracts/${contractId}/records`, { records })

export const apiDeleteRecord = (contractId: string, recordId: string) =>
  client.delete<unknown, { message: string }>(`/contracts/${contractId}/records/${recordId}`)

export const apiPaymentSummary = (contractId: string) =>
  client.get<unknown, PaymentSummary>(`/contracts/${contractId}/payment-summary`)
