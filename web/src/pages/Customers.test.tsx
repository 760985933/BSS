import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, useFormWarningSpy } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Customers from './Customers'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListCustomers: vi.fn(() =>
      Promise.resolve({
        list: [{ id: 'c1', code: 'C-2026-0001', name: '示例客户', industry: '互联网', source: '转介绍', level: 'A', remark: '备注', owner_id: 'u1', created_at: '2026-01-01T00:00:00Z' }],
        total: 1, page: 1, size: 20,
      }),
    ),
    apiListDicts: vi.fn(() => Promise.resolve([])),
    apiListEmployees: vi.fn(() => Promise.resolve({ list: [], total: 0, page: 1, size: 100 })),
  }),
)

describe('Customers 页面', () => {
  it('渲染不崩溃', () => {
    renderWithProviders(<Customers />)
    expect(screen.getByText('客户管理')).toBeInTheDocument()
  })

  it('打开新建弹窗：表单已连接，无 useForm 未连接警告', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Customers />)
    await user.click(await screen.findByText('新建客户'))
    expect(await screen.findByText('客户名称')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('编辑客户：打开弹窗并回填，不崩溃', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Customers />)
    await user.click(await screen.findByText('编辑'))
    expect(await screen.findByText('编辑客户')).toBeInTheDocument()
    expect(screen.getByText('客户名称')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })
})
