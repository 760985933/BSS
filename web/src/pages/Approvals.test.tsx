import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, useFormWarningSpy } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Approvals from './Approvals'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiMe: vi.fn(() => Promise.resolve({ id: 'u1', name: '管理员', role: 'admin', dept: 'x' })),
    apiListApprovals: vi.fn(() =>
      Promise.resolve({
        list: [{
          id: 'a1', code: 'SP-2026-0001', entity_type: 'contract', entity_id: 'ct1',
          kind: 'contract_sign', status: 'pending', applicant_id: 'u1', approver_id: '',
          amount_cent: 0, note: '', reject_reason: '', created_at: '2026-01-01T00:00:00Z',
        }],
        total: 1,
      }),
    ),
    apiListContracts: vi.fn(() =>
      Promise.resolve({
        list: [{ id: 'ct1', code: 'HT-2026-0001', customer_id: 'c1', customer: { id: 'c1', name: '示例客户' }, title: '待签合同', amount_cent: 5000, sign_date: '', start_date: '', expire_date: '', status: 'pending', terminate_reason: '', owner_id: 'u1', owner: { id: 'u1', name: '管理员' }, remark: '', deals: [], created_at: '2026-01-01T00:00:00Z' }],
        total: 1, page: 1, size: 100,
      }),
    ),
    apiListDeals: vi.fn(() =>
      Promise.resolve({
        list: [{ id: 'd1', code: 'SD-2026-0001', customer_id: 'c1', title: '谈判中商单', amount_cent: 10000, probability: 80, expected_sign_date: '', status: 'negotiating', lost_reason: '', owner_id: 'u1', remark: '', created_at: '' }],
        total: 1, page: 1, size: 100,
      }),
    ),
    apiCreateApproval: vi.fn(() => Promise.resolve({})),
    apiApproveApproval: vi.fn(() => Promise.resolve({})),
    apiRejectApproval: vi.fn(() => Promise.resolve({})),
  }),
)

describe('Approvals 页面', () => {
  it('渲染不崩溃', async () => {
    renderWithProviders(<Approvals />)
    expect(await screen.findByText('提交审批')).toBeInTheDocument()
  })

  it('打开提交审批弹窗：表单已连接，无 useForm 未连接警告', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Approvals />)
    await user.click(await screen.findByText('提交审批'))
    expect(await screen.findByText('审批类型')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })
})
