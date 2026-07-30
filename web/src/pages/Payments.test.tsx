import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, useFormWarningSpy } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Payments from './Payments'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListCustomers: vi.fn(() =>
      Promise.resolve({ list: [{ id: 'c1', code: 'C-2026-0001', name: '示例客户', industry: '', source: '', level: '', remark: '', owner_id: 'u1', created_at: '2026-01-01T00:00:00Z' }], total: 1, page: 1, size: 100 }),
    ),
    apiListContracts: vi.fn(() =>
      Promise.resolve({
        list: [{
          id: 'ct1', code: 'HT-2026-0001', customer_id: 'c1', customer: { id: 'c1', name: '示例客户' },
          title: '测试合同', amount_cent: 5000, sign_date: '', start_date: '', expire_date: '',
          status: 'signed', terminate_reason: '', owner_id: 'u1', owner: { id: 'u1', name: '管理员' },
          remark: '', deals: [], created_at: '2026-01-01T00:00:00Z',
        }],
        total: 1, page: 1, size: 20,
      }),
    ),
    apiMe: vi.fn(() => Promise.resolve({ id: 'u1', name: '管理员', role: 'admin', dept: 's' })),
    apiListPlans: vi.fn(() => Promise.resolve([])),
    apiListRecords: vi.fn(() => Promise.resolve([])),
    apiPaymentSummary: vi.fn(() => Promise.resolve({ receivable_cent: 5000, received_cent: 0, balance_cent: 5000, overdue_cent: 0 })),
  }),
)

describe('Payments 页面', () => {
  it('渲染不崩溃', () => {
    renderWithProviders(<Payments />)
    expect(screen.getByText('回款管理')).toBeInTheDocument()
  })

  it('列表渲染合同行', async () => {
    renderWithProviders(<Payments />)
    expect(await screen.findByText('HT-2026-0001')).toBeInTheDocument()
    expect(screen.getByText('测试合同')).toBeInTheDocument()
  })

  it('打开回款抽屉：汇总与计划区渲染，无 useForm 未连接警告', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Payments />)
    await user.click(await screen.findAllByText('回款').then((els) => els[0]))
    expect(await screen.findByText('回款计划')).toBeInTheDocument()
    expect(screen.getByText('回款记录')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })
})
