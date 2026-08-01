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
  owner_id: string // '0' 表示无主（公海客户）
  owner?: Employee
  created_at: string
  // M3-1 公海池
  last_followed_at?: string
  claimed_at?: string
  pool_reason?: string
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

// ---------- 提醒 / 仪表盘 ----------
export interface Notification {
  id: string
  user_id: string
  type: string // contract_expiring / payment_overdue
  title: string
  content: string
  entity_type: string
  entity_id: string
  is_read: boolean
  created_at: string
}

export interface NotificationPage {
  items: Notification[]
  total: number
  page: number
  size: number
}

export const NOTIF_TYPE: Record<string, { label: string; color: string }> = {
  contract_expiring: { label: '合同到期', color: 'orange' },
  payment_overdue: { label: '回款逾期', color: 'red' },
}

export const apiListNotifications = (params: { is_read?: boolean; type?: string; page?: number; size?: number }) =>
  client.get<unknown, NotificationPage>('/notifications', { params })

export const apiUnreadCount = () => client.get<unknown, { count: number }>('/notifications/unread-count')

export const apiMarkNotificationRead = (id: string) =>
  client.post<unknown, { message: string }>(`/notifications/${id}/read`)

export const apiMarkAllRead = () => client.post<unknown, { message: string }>('/notifications/read-all')

export interface ContractLite {
  id: string
  code: string
  title: string
  customer: string
  amount_cent: number
  expire_date: string
  status: string
}

export interface PlanLite {
  id: string
  contract_code: string
  period_no: number
  due_date: string
  amount_cent: number
  paid_cent: number
  outstanding_cent: number
}

export interface DealLite {
  id: string
  code: string
  title: string
  customer: string
  amount_cent: number
  probability: number
  status: string
}

export interface DashboardData {
  cards: {
    signed_this_month_cent: number
    paid_this_month_cent: number
    open_deals: number
    overdue_amount_cent: number
  }
  expiring_contracts: ContractLite[]
  overdue_plans: PlanLite[]
  recent_won_deals: DealLite[]
}

export const apiDashboard = () => client.get<unknown, DashboardData>('/dashboard')

// ---------- 审批流（M2-1） ----------

export type ApprovalKind = 'contract_sign' | 'deal_discount'
export type ApprovalStatus = 'pending' | 'approved' | 'rejected'

export interface Approval {
  id: string
  code: string
  entity_type: 'contract' | 'deal'
  entity_id: string
  kind: ApprovalKind
  status: ApprovalStatus
  applicant_id: string
  approver_id: string
  amount_cent: number
  note: string
  reject_reason: string
  created_at: string
}

export interface ApprovalQuery {
  page?: number
  size?: number
  status?: ApprovalStatus
  kind?: ApprovalKind
  entity_type?: 'contract' | 'deal'
}

export interface ApprovalInput {
  entity_type: 'contract' | 'deal'
  entity_id: string
  kind: ApprovalKind
  amount_cent?: number
  note?: string
}

export const APPROVAL_KIND: Record<ApprovalKind, string> = {
  contract_sign: '合同签约',
  deal_discount: '商单折扣',
}
export const APPROVAL_STATUS: Record<ApprovalStatus, { label: string; color: string }> = {
  pending: { label: '待审批', color: 'processing' },
  approved: { label: '已通过', color: 'success' },
  rejected: { label: '已驳回', color: 'default' },
}

export const apiListApprovals = (q: ApprovalQuery = {}) =>
  client.get<unknown, { list: Approval[]; total: number }>('/approvals', { params: q })
export const apiGetApproval = (id: string) =>
  client.get<unknown, Approval>(`/approvals/${id}`)
export const apiCreateApproval = (input: ApprovalInput) =>
  client.post<unknown, Approval>('/approvals', input)
export const apiApproveApproval = (id: string) =>
  client.post<unknown, unknown>(`/approvals/${id}/approve`)
export const apiRejectApproval = (id: string, reason: string) =>
  client.post<unknown, unknown>(`/approvals/${id}/reject`, { reason })

// ---------- 开票管理（M2-2） ----------

export type InvoiceStatus = 'draft' | 'issued' | 'voided'

export interface Invoice {
  id: string
  code: string
  contract_id: string
  contract?: { id: string; code: string; title: string }
  payment_record_id: string | null
  amount_cent: number
  status: InvoiceStatus
  issued_at: string
  remark: string
  created_by: string
  created_at: string
}

export interface InvoiceInput {
  contract_id: string
  payment_record_id?: string | null
  amount_cent: number
  remark?: string
}

export const INVOICE_STATUS: Record<InvoiceStatus, { label: string; color: string }> = {
  draft: { label: '待开', color: 'default' },
  issued: { label: '已开', color: 'success' },
  voided: { label: '已作废', color: 'default' },
}

export const apiListInvoices = (q: { page?: number; size?: number; contract_id?: string; status?: string } = {}) =>
  client.get<unknown, { list: Invoice[]; total: number }>('/invoices', { params: q })
export const apiCreateInvoice = (data: InvoiceInput) =>
  client.post<unknown, Invoice>('/invoices', data)
export const apiIssueInvoice = (id: string) =>
  client.post<unknown, unknown>(`/invoices/${id}/issue`)
export const apiVoidInvoice = (id: string) =>
  client.post<unknown, unknown>(`/invoices/${id}/void`)
export const apiUpdateInvoice = (id: string, data: InvoiceInput) =>
  client.put<unknown, { message: string }>(`/invoices/${id}`, data)
export const apiDeleteInvoice = (id: string) =>
  client.delete<unknown, { message: string }>(`/invoices/${id}`)

// ---------- 报表中心（M2-3） ----------

export interface MonthPoint {
  month: string // YYYY-MM
  amount_cent: number
}

export interface SignTrendResult {
  rows: MonthPoint[]
}

export interface PaymentTrendResult {
  rows: MonthPoint[]
}

export interface SalesRankRow {
  owner_id: string
  owner_name: string
  won_deals: number
  signed_cent: number
  paid_cent: number
}

export interface SalesRankResult {
  rows: SalesRankRow[]
}

export interface FunnelRow {
  stage: string
  label: string
  count: number
  amount_cent: number
}

export interface FunnelResult {
  rows: FunnelRow[]
}

export type ReportType = 'sign_trend' | 'payment_trend' | 'sales_rank' | 'funnel'

export const apiSignTrend = (months = 12) =>
  client.get<unknown, SignTrendResult>('/reports/sign-trend', { params: { months } })
export const apiPaymentTrend = (months = 12) =>
  client.get<unknown, PaymentTrendResult>('/reports/payment-trend', { params: { months } })
export const apiSalesRank = () =>
  client.get<unknown, SalesRankResult>('/reports/sales-rank')
export const apiFunnel = () =>
  client.get<unknown, FunnelResult>('/reports/funnel')

// CSV 导出：以 Blob 形式返回，由页面触发下载
export const apiExportReport = (type: ReportType) =>
  client.get<unknown, Blob>('/reports/export', { params: { type }, responseType: 'blob' })

export const REPORT_LABEL: Record<ReportType, string> = {
  sign_trend: '月度签约趋势',
  payment_trend: '月度回款趋势',
  sales_rank: '销售排行',
  funnel: '客户转化漏斗',
}

// ---------- 审计查询 + 离职交接（M2-4） ----------

export interface AuditLog {
  id: string
  entity_type: string
  entity_id: string
  action: string // create/update/delete/transfer/status_change/offboard
  operator_id: string
  before_json: string
  after_json: string
  created_at: string
}

export interface AuditQuery {
  entity_type?: string
  entity_id?: string
  action?: string
  operator_id?: string
  start?: string // YYYY-MM-DD
  end?: string // YYYY-MM-DD
  page?: number
  size?: number
}

export const AUDIT_ACTION: Record<string, { label: string; color: string }> = {
  create: { label: '新建', color: 'blue' },
  update: { label: '修改', color: 'gold' },
  delete: { label: '删除', color: 'red' },
  transfer: { label: '转移', color: 'cyan' },
  status_change: { label: '状态变更', color: 'geekblue' },
  offboard: { label: '离职交接', color: 'purple' },
}

export const apiListAuditLogs = (q: AuditQuery = {}) =>
  client.get<unknown, PageData<AuditLog>>('/audit-logs', { params: q })

export interface OffboardPreview {
  active: boolean
  has_data: boolean
  customers: number
  deals: number
  contracts: number
}

export const apiOffboardPreview = (id: string) =>
  client.get<unknown, OffboardPreview>(`/employees/${id}/offboard-preview`)

export const apiOffboard = (id: string, successor_id: string) =>
  client.post<unknown, { result: { customers: number; deals: number; contracts: number }; message: string }>(
    `/employees/${id}/offboard`,
    { successor_id },
  )

// ---------- 客户公海池（M3-1） ----------
export interface PoolLog {
  id: string
  customer_id: string
  action: string // claim/release/recycle/assign
  from_owner_id: string
  to_owner_id: string
  operator_id: string
  reason: string
  created_at: string
}

export interface PoolSettings {
  id: string
  enabled: boolean
  max_claim_per_sales: number
  idle_days_no_follow: number
  idle_days_no_deal: number
  protect_days: number
  updated_at: string
}

export interface RecycleItem {
  customer_id: string
  name: string
  owner_id: string
  reason: string
}

export interface RecycleResult {
  total: number
  items: RecycleItem[]
}

export interface PoolQuery {
  page?: number
  size?: number
  keyword?: string
  industry?: string
  source?: string
  level?: string
}

export const POOL_ACTION: Record<string, { label: string; color: string }> = {
  claim: { label: '领取', color: 'green' },
  release: { label: '释放', color: 'orange' },
  recycle: { label: '回收', color: 'red' },
  assign: { label: '指派', color: 'blue' },
}

export const apiListPool = (params: PoolQuery = {}) =>
  client.get<unknown, PageData<Customer>>('/customer-pool', { params })

export const apiClaimCustomer = (id: string) =>
  client.post<unknown, { message: string }>(`/customers/${id}/claim`)

export const apiReleaseCustomer = (id: string, reason?: string) =>
  client.post<unknown, { message: string }>(`/customers/${id}/release`, { reason })

export const apiAssignFromPool = (id: string, owner_id: string) =>
  client.post<unknown, { message: string }>(`/customer-pool/${id}/assign`, { owner_id })

export const apiRecyclePool = (dryRun = false) =>
  client.post<unknown, RecycleResult>('/customer-pool/recycle' + (dryRun ? '?dry_run=1' : ''))

export const apiPoolLogs = (id: string) =>
  client.get<unknown, PoolLog[]>(`/customers/${id}/pool-logs`)

export const apiGetPoolSettings = () =>
  client.get<unknown, PoolSettings>('/customer-pool/settings')

export const apiUpdatePoolSettings = (s: Omit<PoolSettings, 'id' | 'updated_at'>) =>
  client.put<unknown, PoolSettings>('/customer-pool/settings', s)

// ---------- Excel 数据导入（M3-2） ----------
export interface ImportRowError {
  row: number
  message: string
}

export interface ImportResult {
  total: number
  created_customers: number
  created_contacts: number
  skipped: number
  errors: ImportRowError[]
}

// 上传并导入客户/联系人 Excel（multipart，字段 file）
export const apiImportCustomers = (file: File) => {
  const fd = new FormData()
  fd.append('file', file)
  return client.post<unknown, ImportResult>('/imports/customers', fd)
}

// 下载导入模板（xlsx 二进制流）
export const apiDownloadCustomerTemplate = () =>
  client.get('/imports/customers/template', { responseType: 'blob' }) as Promise<Blob>

// ---------- 项目/交付管理（M3-3） ----------
export type ProjectStatus = 'planning' | 'in_progress' | 'on_hold' | 'completed' | 'cancelled'
export type TaskKind = 'task' | 'milestone'
export type TaskStatus = 'todo' | 'doing' | 'done'

export const PROJECT_STATUS: Record<ProjectStatus, { label: string; color: string }> = {
  planning: { label: '规划中', color: 'default' },
  in_progress: { label: '进行中', color: 'processing' },
  on_hold: { label: '已暂停', color: 'warning' },
  completed: { label: '已完成', color: 'success' },
  cancelled: { label: '已取消', color: 'error' },
}

export const TASK_STATUS: Record<TaskStatus, { label: string; color: string }> = {
  todo: { label: '待办', color: 'default' },
  doing: { label: '进行中', color: 'processing' },
  done: { label: '已完成', color: 'success' },
}

export interface Project {
  id: string
  code: string
  name: string
  customer_id: string | null
  customer?: { id: string; name: string }
  owner_id: string
  owner?: Employee
  status: ProjectStatus
  start_date: string
  end_date: string
  description: string
  members?: ProjectMember[]
  tasks?: ProjectTask[]
  created_at: string
}

export interface ProjectMember {
  id: string
  project_id: string
  employee_id: string
  employee?: Employee
  role: string
  planned_days: number
  actual_days: number
}

export interface ProjectTask {
  id: string
  project_id: string
  kind: TaskKind
  title: string
  assignee_id: string | null
  assignee?: Employee
  due_date: string
  status: TaskStatus
  estimate_days: number
  sort: number
}

export interface ProjectInput {
  name: string
  customer_id?: string | null
  owner_id: string
  status?: ProjectStatus
  start_date?: string
  end_date?: string
  description?: string
}

export interface ProjectQuery {
  page?: number
  size?: number
  keyword?: string
  status?: string
  owner_id?: string
}

export const apiListProjects = (params: ProjectQuery) =>
  client.get<unknown, PageData<Project>>('/projects', { params })

export const apiGetProject = (id: string) => client.get<unknown, Project>(`/projects/${id}`)

export const apiCreateProject = (data: ProjectInput) =>
  client.post<unknown, Project>('/projects', data)

export const apiUpdateProject = (id: string, data: ProjectInput) =>
  client.put<unknown, { message: string }>(`/projects/${id}`, data)

export const apiDeleteProject = (id: string) =>
  client.delete<unknown, { message: string }>(`/projects/${id}`)

export const apiListProjectMembers = (id: string) =>
  client.get<unknown, ProjectMember[]>(`/projects/${id}/members`)

export const apiAddProjectMember = (id: string, data: { employee_id: string; role?: string; planned_days?: number; actual_days?: number }) =>
  client.post<unknown, ProjectMember>(`/projects/${id}/members`, data)

export const apiUpdateProjectMember = (id: string, mid: string, data: { role?: string; planned_days?: number; actual_days?: number }) =>
  client.put<unknown, { message: string }>(`/projects/${id}/members/${mid}`, data)

export const apiRemoveProjectMember = (id: string, mid: string) =>
  client.delete<unknown, { message: string }>(`/projects/${id}/members/${mid}`)

export const apiAddProjectTask = (id: string, data: {
  kind: TaskKind; title: string; assignee_id?: string | null; due_date?: string; status?: TaskStatus; estimate_days?: number; sort?: number
}) => client.post<unknown, ProjectTask>(`/projects/${id}/tasks`, data)

export const apiUpdateProjectTask = (id: string, tid: string, data: {
  kind: TaskKind; title: string; assignee_id?: string | null; due_date?: string; status?: TaskStatus; estimate_days?: number; sort?: number
}) => client.put<unknown, { message: string }>(`/projects/${id}/tasks/${tid}`, data)

export const apiRemoveProjectTask = (id: string, tid: string) =>
  client.delete<unknown, { message: string }>(`/projects/${id}/tasks/${tid}`)

// ---------- 通知渠道（M3-4，admin） ----------
export type NotifyChannel = 'email' | 'wecom'

export interface NotifySettings {
  email_enabled: boolean
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password: string // 读取时为掩码，原样回传表示不修改
  smtp_from: string
  smtp_tls: boolean
  wecom_enabled: boolean
  wecom_webhook: string
  types: string // 逗号分隔白名单，空=全部
  updated_at: string
}

export interface NotifyLog {
  id: string
  channel: NotifyChannel
  notification_id: string
  user_id: string
  target: string
  title: string
  status: 'success' | 'failed'
  error: string
  created_at: string
}

export const apiGetNotifySettings = () =>
  client.get<unknown, NotifySettings>('/notify-settings')

export const apiUpdateNotifySettings = (data: Partial<NotifySettings>) =>
  client.put<unknown, NotifySettings>('/notify-settings', data)

export const apiTestNotifyChannel = (channel: NotifyChannel, to?: string) =>
  client.post<unknown, { message: string }>('/notify-settings/test', { channel, to })

export const apiListNotifyLogs = (params: { channel?: string; status?: string; page?: number; size?: number }) =>
  client.get<unknown, PageData<NotifyLog>>('/notify-logs', { params })

// ---------- 客户查重合并（M2 增强，admin） ----------
export interface DuplicateGroup {
  field: 'phone' | 'email'
  value: string
  customers: Customer[]
}

export const apiFindDuplicates = () =>
  client.get<unknown, DuplicateGroup[]>('/customers/duplicates')

export const apiMergeCustomers = (primary_id: number, secondary_id: number) =>
  client.post<unknown, { message: string }>('/customers/merge', { primary_id, secondary_id })

// ---------- 商单输单分析（M4-2，admin/sales_lead） ----------
export interface LostAnalysis {
  total: number
  by_reason: { key: string; count: number }[]
  by_owner: { owner_id: number; name: string; count: number }[]
  by_month: { key: string; count: number }[]
}

export const apiLostDealAnalysis = () =>
  client.get<unknown, LostAnalysis>('/reports/lost-analysis')

// ---------- 银企对账（M4-3，admin/finance） ----------
export interface BankStatement {
  id: string
  trans_date: string
  counterparty: string
  amount_cent: number
  direction: 'income' | 'expend'
  summary: string
  payment_record_id?: string | null
}

export interface Reconciliation {
  bank_only: { id: string; trans_date: string; counterparty: string; amount_cent: number; direction: string; summary: string }[]
  company_only: { id: string; trans_date: string; amount_cent: number }[]
}

export const apiCreateBankStatements = (items: unknown[]) =>
  client.post<unknown, { created: number }>('/bank-statements', items)
export const apiListBankStatements = () =>
  client.get<unknown, BankStatement[]>('/bank-statements')
export const apiReconcile = (id: string, payment_record_id: string) =>
  client.post<unknown, { message: string }>(`/bank-statements/${id}/reconcile`, { payment_record_id: Number(payment_record_id) })
export const apiUnreconcile = (id: string) =>
  client.post<unknown, { message: string }>(`/bank-statements/${id}/unreconcile`, {})
export const apiReconciliation = () =>
  client.get<unknown, Reconciliation>('/reconciliation')

// ---------- 招聘管理（M6-S1，admin/hr） ----------
export interface JobPost {
  id: string
  code: string
  title: string
  dept: string
  headcount: number
  status: 'open' | 'closed'
  description: string
  owner_id: string
  owner?: { name: string }
}

export interface Candidate {
  id: string
  job_post_id: string
  job_post?: { title: string }
  name: string
  phone: string
  email: string
  stage: string
  source: string
  resume_url: string
  owner_id: string
  owner?: { name: string }
}

export interface JobPostInput {
  title: string
  dept?: string
  headcount?: number
  status?: 'open' | 'closed'
  description?: string
}

export interface CandidateInput {
  job_post_id?: string
  name: string
  phone?: string
  email?: string
  source?: string
  resume_url?: string
}

export interface FunnelStage {
  stage: string
  count: number
}

export const JOB_STATUS: Record<string, { label: string; color: string }> = {
  open: { label: '招聘中', color: 'green' },
  closed: { label: '已关闭', color: 'default' },
}

export const CANDIDATE_STAGE: Record<string, { label: string; color: string }> = {
  apply: { label: '投递', color: 'default' },
  screen: { label: '筛选', color: 'blue' },
  interview: { label: '面试', color: 'cyan' },
  offer: { label: 'Offer', color: 'gold' },
  hired: { label: '已入职', color: 'green' },
  rejected: { label: '已淘汰', color: 'red' },
}

export const apiListJobPosts = (params?: { keyword?: string; status?: string }) =>
  client.get<unknown, JobPost[]>('/job-posts', { params })

export const apiCreateJobPost = (data: JobPostInput) =>
  client.post<unknown, { job_post: JobPost; message: string }>('/job-posts', data)

export const apiUpdateJobPost = (id: string, data: JobPostInput) =>
  client.put<unknown, { message: string }>(`/job-posts/${id}`, data)

export const apiDeleteJobPost = (id: string) =>
  client.delete<unknown, { message: string }>(`/job-posts/${id}`)

export const apiListCandidates = (params?: { keyword?: string; job_post_id?: string; stage?: string }) =>
  client.get<unknown, Candidate[]>('/candidates', { params })

export const apiCreateCandidate = (data: CandidateInput) =>
  client.post<unknown, { candidate: Candidate; message: string }>('/candidates', data)

export const apiUpdateCandidate = (id: string, data: CandidateInput) =>
  client.put<unknown, { message: string }>(`/candidates/${id}`, data)

export const apiDeleteCandidate = (id: string) =>
  client.delete<unknown, { message: string }>(`/candidates/${id}`)

export const apiAdvanceCandidate = (id: string, stage: string, force: boolean) =>
  client.post<unknown, { message: string }>(`/candidates/${id}/advance`, { stage, force })

export const apiCandidatesFunnel = (params?: { job_post_id?: string }) =>
  client.get<unknown, FunnelStage[]>('/candidates/funnel', { params })

// ---------- 劳动合同 + 入职管理（M6-S2，admin/hr） ----------
export interface LaborContract {
  id: string
  code: string
  employee_id: string
  employee?: { id: string; name: string }
  type: string
  start_date?: string
  end_date?: string
  sign_date?: string
  probation_months: number
  status: string
  terminate_reason?: string
  owner_id: string
  owner?: { id: string; name: string }
}

export interface Onboarding {
  id: string
  code: string
  employee_id: string
  employee?: { id: string; name: string }
  candidate_id?: string
  candidate?: { id: string; name: string }
  step_profile: string
  step_equip: string
  step_training: string
  step_probation: string
  status: string
  owner_id: string
}

export interface LaborContractInput {
  employee_id: string
  type: string
  start_date: string
  end_date: string
  sign_date: string
  probation_months: number
}

export interface OnboardingInput {
  employee_id: string
  candidate_id?: string
  step_profile: string
  step_equip: string
  step_training: string
  step_probation: string
}

export const LC_STATUS: Record<string, { label: string; color: string }> = {
  draft: { label: '草稿', color: 'default' },
  active: { label: '生效中', color: 'green' },
  expired: { label: '已到期', color: 'red' },
  renewed: { label: '已续签', color: 'blue' },
  terminated: { label: '已解除', color: 'volcano' },
}

export const LC_TYPE: Record<string, string> = {
  fixed: '固定期限',
  nonfixed: '无固定期限',
  internship: '实习',
  parttime: '兼职',
}

export const OB_STEP: Record<string, { label: string; color: string }> = {
  pending: { label: '待办', color: 'default' },
  done: { label: '完成', color: 'green' },
}

export const OB_STATUS: Record<string, { label: string; color: string }> = {
  in_progress: { label: '进行中', color: 'processing' },
  completed: { label: '已完成', color: 'green' },
}

export const apiListLaborContracts = (params?: { keyword?: string; status?: string; employee_id?: string }) =>
  client.get<unknown, LaborContract[]>('/labor-contracts', { params })

export const apiCreateLaborContract = (data: LaborContractInput) =>
  client.post<unknown, LaborContract>('/labor-contracts', data)

export const apiUpdateLaborContract = (id: string, data: LaborContractInput) =>
  client.put<unknown, LaborContract>(`/labor-contracts/${id}`, data)

export const apiDeleteLaborContract = (id: string) =>
  client.delete<unknown, { message: string }>(`/labor-contracts/${id}`)

export const apiTransitionContract = (id: string, data: { to: string; reason: string; force: boolean }) =>
  client.post<unknown, LaborContract>(`/labor-contracts/${id}/transition`, data)

export const apiListOnboardings = (params?: { keyword?: string; status?: string; employee_id?: string }) =>
  client.get<unknown, Onboarding[]>('/onboardings', { params })

export const apiCreateOnboarding = (data: OnboardingInput) =>
  client.post<unknown, Onboarding>('/onboardings', data)

export const apiUpdateOnboarding = (id: string, data: OnboardingInput) =>
  client.put<unknown, Onboarding>(`/onboardings/${id}`, data)

export const apiDeleteOnboarding = (id: string) =>
  client.delete<unknown, { message: string }>(`/onboardings/${id}`)

// ---------- 考勤排班 + 请假 + 考勤记录（M6-S3，admin/hr） ----------
export interface AttendanceSchedule {
  id: string
  employee_id: string
  employee?: { id: string; name: string }
  weekday: number // 1=周一..7=周日
  start_time: string
  end_time: string
  shift_type: string
}

export interface LeaveRequest {
  id: string
  code: string
  employee_id: string
  employee?: { id: string; name: string }
  type: string
  start_date?: string
  end_date?: string
  reason?: string
  status: string
  approver_id: string
  approver?: { id: string; name: string }
  approved_at?: string
  reject_reason?: string
}

export interface Attendance {
  id: string
  employee_id: string
  employee?: { id: string; name: string }
  date: string
  schedule_id?: string
  schedule?: AttendanceSchedule
  status: string
  leave_type?: string
  remark?: string
}

export interface ScheduleInput {
  employee_id: string
  weekday: number
  start_time: string
  end_time: string
  shift_type: string
}

export interface LeaveRequestInput {
  employee_id: string
  type: string
  start_date: string
  end_date: string
  reason?: string
}

export interface AttendanceInput {
  employee_id: string
  date: string
  schedule_id?: string
  status: string
  leave_type?: string
  remark?: string
}

export const SCHEDULE_SHIFT: Record<string, string> = {
  regular: '常日班',
  night: '夜班',
  shift: '倒班',
}
export const WEEKDAY_LABEL: Record<number, string> = {
  1: '周一', 2: '周二', 3: '周三', 4: '周四', 5: '周五', 6: '周六', 7: '周日',
}
export const LEAVE_TYPE: Record<string, string> = {
  annual: '年假',
  sick: '病假',
  personal: '事假',
  marriage: '婚假',
  maternity: '产假/陪产假',
  bereavement: '丧假',
}
export const LEAVE_STATUS: Record<string, { label: string; color: string }> = {
  pending: { label: '待审批', color: 'processing' },
  approved: { label: '已通过', color: 'success' },
  rejected: { label: '已驳回', color: 'default' },
}
export const ATT_STATUS: Record<string, { label: string; color: string }> = {
  normal: { label: '正常', color: 'success' },
  late: { label: '迟到', color: 'gold' },
  early: { label: '早退', color: 'orange' },
  absent: { label: '缺勤', color: 'red' },
  leave: { label: '请假', color: 'blue' },
  holiday: { label: '节假日', color: 'purple' },
}

export const apiListSchedules = (params?: { employee_id?: string }) =>
  client.get<unknown, AttendanceSchedule[]>('/schedules', { params })
export const apiCreateSchedule = (data: ScheduleInput) =>
  client.post<unknown, AttendanceSchedule>('/schedules', data)
export const apiUpdateSchedule = (id: string, data: ScheduleInput) =>
  client.put<unknown, AttendanceSchedule>(`/schedules/${id}`, data)
export const apiDeleteSchedule = (id: string) =>
  client.delete<unknown, { message: string }>(`/schedules/${id}`)

export const apiListLeaveRequests = (params?: { employee_id?: string; status?: string; type?: string }) =>
  client.get<unknown, LeaveRequest[]>('/leave-requests', { params })
export const apiCreateLeaveRequest = (data: LeaveRequestInput) =>
  client.post<unknown, LeaveRequest>('/leave-requests', data)
export const apiDeleteLeaveRequest = (id: string) =>
  client.delete<unknown, { message: string }>(`/leave-requests/${id}`)
export const apiDecideLeaveRequest = (id: string, approve: boolean, reason?: string) =>
  client.post<unknown, LeaveRequest>(`/leave-requests/${id}/decide`, { approve, reason })

export const apiListAttendances = (params?: { employee_id?: string; date?: string; status?: string; from?: string; to?: string }) =>
  client.get<unknown, Attendance[]>('/attendances', { params })
export const apiUpsertAttendance = (data: AttendanceInput) =>
  client.post<unknown, Attendance>('/attendances', data)
export const apiGenerateAttendance = (date: string) =>
  client.post<unknown, { created: number }>('/attendances/generate', { date })
export const apiDeleteAttendance = (id: string) =>
  client.delete<unknown, { message: string }>(`/attendances/${id}`)
