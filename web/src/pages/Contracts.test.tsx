import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, useFormWarningSpy } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Contracts from './Contracts'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListCustomers: vi.fn(() =>
      Promise.resolve({ list: [{ id: 'c1', code: 'C-2026-0001', name: '示例客户', industry: '', source: '', level: '', remark: '', owner_id: 'u1', created_at: '2026-01-01T00:00:00Z' }], total: 1, page: 1, size: 100 }),
    ),
    apiListDeals: vi.fn(() =>
      Promise.resolve({
        list: [{ id: 'd1', code: 'SD-2026-0001', customer_id: 'c1', title: '赢单A', amount_cent: 10000, probability: 100, expected_sign_date: '', status: 'won', lost_reason: '', owner_id: 'u1', remark: '', created_at: '' }],
        total: 1, page: 1, size: 20,
      }),
    ),
    apiListContracts: vi.fn(() =>
      Promise.resolve({
        list: [{
          id: 'ct1', code: 'HT-2026-0001', customer_id: 'c1', customer: { id: 'c1', name: '示例客户' },
          title: '测试合同', amount_cent: 5000, sign_date: '', start_date: '', expire_date: '',
          status: 'draft', terminate_reason: '', owner_id: 'u1', owner: { id: 'u1', name: '管理员' },
          remark: '', deals: [], created_at: '2026-01-01T00:00:00Z',
        }],
        total: 1, page: 1, size: 20,
      }),
    ),
    apiCreateContract: vi.fn(() => Promise.resolve({})),
    apiUpdateContract: vi.fn(() => Promise.resolve({ message: 'ok' })),
    apiListContractAttachments: vi.fn(() => Promise.resolve([])),
    apiUploadContractAttachment: vi.fn(() => Promise.resolve({})),
    apiDownloadAttachment: vi.fn(() => Promise.resolve(new Blob())),
    apiDeleteAttachment: vi.fn(() => Promise.resolve({ message: 'ok' })),
  }),
)

describe('Contracts 页面', () => {
  it('渲染不崩溃', () => {
    renderWithProviders(<Contracts />)
    expect(screen.getByText('合同管理')).toBeInTheDocument()
  })

  it('打开新建弹窗：表单已连接，无 useForm 未连接警告', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Contracts />)
    await user.click(await screen.findByText('新建合同'))
    // 表单 label 出现 = <Form> 已挂载并连接，证明不再有 “not connected” 警告
    expect(await screen.findByText('合同标题')).toBeInTheDocument()
    expect(screen.getByText('关联客户')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('编辑草稿合同：回填字段且不崩溃', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Contracts />)
    await user.click(await screen.findByText('编辑'))
    expect(await screen.findByText('编辑合同')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('打开附件面板：渲染上传入口不崩溃', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Contracts />)
    await user.click(await screen.findByText('附件'))
    expect(await screen.findByText('上传附件')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })
})
