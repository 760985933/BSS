import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, useFormWarningSpy } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Deals from './Deals'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListCustomers: vi.fn(() =>
      Promise.resolve({
        list: [{ id: 'c1', code: 'C-2026-0001', name: '示例客户', industry: '', source: '', level: '', remark: '', owner_id: 'u1', created_at: '2026-01-01T00:00:00Z' }],
        total: 1, page: 1, size: 100,
      }),
    ),
    apiDealForecast: vi.fn(() => Promise.resolve({ open_count: 1, total_cent: 10000, weighted_cent: 1000 })),
    apiListDeals: vi.fn(() =>
      Promise.resolve({
        list: [
          { id: 'd1', code: 'D-2026-0001', customer_id: 'c1', title: '开放商单', amount_cent: 10000, probability: 10, expected_sign_date: '2026-03-01', status: 'prospecting', lost_reason: '', owner_id: 'u1', remark: '', created_at: '2026-01-01T00:00:00Z' },
          { id: 'd2', code: 'D-2026-0002', customer_id: 'c1', title: '已赢单', amount_cent: 20000, probability: 100, expected_sign_date: '2026-02-01', status: 'won', lost_reason: '', owner_id: 'u1', remark: 'ok', created_at: '2026-01-01T00:00:00Z' },
        ],
        total: 2, page: 1, size: 20,
      }),
    ),
  }),
)

// 捕获 console.warn/error 中是否出现 rc-field-form 的 “not connected” 警告
// （useFormWarningSpy 已抽到 ../test/render）

describe('Deals 页面', () => {
  it('渲染不崩溃', () => {
    renderWithProviders(<Deals />)
    expect(screen.getByText('商单管理')).toBeInTheDocument()
  })

  it('打开新建弹窗：表单已连接，无 useForm 未连接警告', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Deals />)
    await user.click(await screen.findByText('新建商单'))
    // 表单 label 出现 = <Form> 已挂载并连接，证明不再有 “not connected” 警告
    expect(await screen.findByText('商单标题')).toBeInTheDocument()
    expect(screen.getByText('关联客户')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('编辑开放商单：回填字段且不崩溃', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Deals />)
    const editBtns = await screen.findAllByText('编辑')
    await user.click(editBtns[0])
    expect(await screen.findByText('编辑商单')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('编辑已关闭商单（备注）：setFieldsValue 不崩溃', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Deals />)
    await user.click(await screen.findByText('备注'))
    expect(await screen.findByText('编辑备注（商单已关闭）')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })

  it('closed(null) 路径：新建弹窗内存取 editing 为 null 不抛错', async () => {
    const user = userEvent.setup()
    const spy = useFormWarningSpy()
    renderWithProviders(<Deals />)
    await user.click(await screen.findByText('新建商单'))
    // 新建态 editing=null，表单规则 required: !closed(editing) 不应抛错；
    // 弹窗内表单正常挂载（关联客户字段存在）即证明
    expect(await screen.findByText('关联客户')).toBeInTheDocument()
    spy.assertNoUseFormWarning()
    spy.restore()
  })
})
