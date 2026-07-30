import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, useFormWarningSpy } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Employees from './Employees'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListEmployees: vi.fn(() =>
      Promise.resolve({
        list: [
          { id: 'e1', name: '张三', email: 'z@bss.local', phone: '13800000000', dept: '销售', position: '经理', role: 'sales', status: 'active' },
          { id: 'e2', name: '李四', email: 'l@bss.local', phone: '13900000000', dept: '销售', position: '专员', role: 'sales', status: 'active' },
        ],
        total: 2, page: 1, size: 20,
      }),
    ),
    apiListDicts: vi.fn(() => Promise.resolve([])),
    apiOffboardPreview: vi.fn(() =>
      Promise.resolve({ active: true, has_data: true, customers: 2, deals: 1, contracts: 3 }),
    ),
    apiOffboard: vi.fn(() =>
      Promise.resolve({ result: { customers: 2, deals: 1, contracts: 3 }, message: 'ok' }),
    ),
    // 管理员可见「新建员工」
    apiMe: vi.fn(() => Promise.resolve({ id: 'u1', name: '管理员', role: 'admin', dept: 'HQ' })),
  }),
)

describe('Employees 页面', () => {
  it('渲染不崩溃', () => {
    renderWithProviders(<Employees />)
    expect(screen.getByText('员工管理')).toBeInTheDocument()
  })

  it('打开新建弹窗：表单已连接，无 useForm 未连接警告', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Employees />)
    await user.click(await screen.findByText('新建员工'))
    expect(await screen.findByText('邮箱（登录账号）')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('编辑员工：打开弹窗并回填，不崩溃', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Employees />)
    const [editBtn] = await screen.findAllByText('编辑')
    await user.click(editBtn)
    expect(await screen.findByText('编辑员工')).toBeInTheDocument()
    expect(screen.getByText('邮箱（登录账号）')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('离职交接：点击停用时弹窗显示待转移数据量', async () => {
    const user = userEvent.setup()
    renderWithProviders(<Employees />)
    const [disableBtn] = await screen.findAllByText('停用')
    await user.click(disableBtn)
    await user.click(await screen.findByText('OK')) // 确认 Popconfirm（测试环境无 zh_CN locale）
    expect(await screen.findByText('离职交接')).toBeInTheDocument()
    expect(screen.getByText('客户：2 个')).toBeInTheDocument()
    expect(screen.getByText('商单：1 个')).toBeInTheDocument()
    expect(screen.getByText('合同：3 个')).toBeInTheDocument()
    // 未选交接人时确认按钮禁用（按按钮 role 取真实 <button>，避免 span 取不到 disabled）
    expect(screen.getByRole('button', { name: '确认交接并停用' })).toBeDisabled()
  })
})
