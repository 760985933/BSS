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
        list: [{ id: 'e1', name: '张三', email: 'z@bss.local', phone: '13800000000', dept: '销售', position: '经理', role: 'sales', status: 'active' }],
        total: 1, page: 1, size: 20,
      }),
    ),
    apiListDicts: vi.fn(() => Promise.resolve([])),
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
    await user.click(await screen.findByText('编辑'))
    expect(await screen.findByText('编辑员工')).toBeInTheDocument()
    expect(screen.getByText('邮箱（登录账号）')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })
})
