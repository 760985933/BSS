import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, useFormWarningSpy } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import CustomerDetail from './CustomerDetail'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiGetCustomer: vi.fn(() =>
      Promise.resolve({ id: 'c1', code: 'C-2026-0001', name: '示例客户', industry: '互联网', source: '转介绍', level: 'A', remark: '备注', owner_id: 'u1', created_at: '2026-01-01T00:00:00Z' }),
    ),
    apiListContacts: vi.fn(() =>
      Promise.resolve([{ id: 'ct1', customer_id: 'c1', name: '李四', phone: '13900000000', email: 'l@bss.local', position: '采购', is_primary: true, remark: '' }]),
    ),
    apiListDeals: vi.fn(() => Promise.resolve({ list: [], total: 0, page: 1, size: 50 })),
  }),
)

describe('CustomerDetail 页面', () => {
  it('渲染客户信息不崩溃', async () => {
    renderWithProviders(<CustomerDetail />, { route: '/customers/c1' })
    expect(await screen.findByText('示例客户')).toBeInTheDocument()
  })

  it('添加联系人：弹窗表单已连接，无 useForm 未连接警告', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<CustomerDetail />, { route: '/customers/c1' })
    await user.click(await screen.findByText('添加'))
    expect(await screen.findByText('添加联系人')).toBeInTheDocument()
    expect(screen.getByText('首要联系人')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('编辑联系人：打开弹窗并回填，不崩溃', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<CustomerDetail />, { route: '/customers/c1' })
    await user.click(await screen.findByText('编辑'))
    expect(await screen.findByText('编辑联系人')).toBeInTheDocument()
    expect(screen.getByText('首要联系人')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })
})
