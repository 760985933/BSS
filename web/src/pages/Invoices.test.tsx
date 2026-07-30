import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, useFormWarningSpy } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Invoices from './Invoices'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiMe: vi.fn(() => Promise.resolve({ id: 'u1', name: '管理员', role: 'admin', dept: 'x' })),
    apiListInvoices: vi.fn(() =>
      Promise.resolve({
        list: [{
          id: 'i1', code: 'KP-2026-0001', contract_id: 'ct1',
          contract: { id: 'ct1', code: 'HT-1', title: '开票合同' },
          payment_record_id: null, amount_cent: 30000, status: 'draft',
          issued_at: '', remark: '', created_by: 'u1', created_at: '2026-01-01T00:00:00Z',
        }],
        total: 1,
      }),
    ),
    apiListContracts: vi.fn(() =>
      Promise.resolve({
        list: [{ id: 'ct1', code: 'HT-2026-0001', customer_id: 'c1', customer: { id: 'c1', name: '客户' }, title: '已签合同', amount_cent: 100000, sign_date: '', start_date: '', expire_date: '', status: 'signed', terminate_reason: '', owner_id: 'u1', owner: { id: 'u1', name: '管理员' }, remark: '', deals: [], created_at: '2026-01-01T00:00:00Z' }],
        total: 1, page: 1, size: 100,
      }),
    ),
    apiCreateInvoice: vi.fn(() => Promise.resolve({})),
    apiIssueInvoice: vi.fn(() => Promise.resolve({})),
    apiVoidInvoice: vi.fn(() => Promise.resolve({})),
    apiDeleteInvoice: vi.fn(() => Promise.resolve({})),
  }),
)

describe('Invoices 页面', () => {
  it('渲染不崩溃', async () => {
    renderWithProviders(<Invoices />)
    expect(await screen.findByText('开票管理（累计金额受合同额约束）')).toBeInTheDocument()
  })

  it('打开新建发票弹窗：表单已连接，无 useForm 未连接警告', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Invoices />)
    await user.click(await screen.findByText('新建发票'))
    expect(await screen.findByText('关联合同（仅显示已签约）')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })
})
