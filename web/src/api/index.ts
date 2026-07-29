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
