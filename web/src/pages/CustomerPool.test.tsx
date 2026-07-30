import { describe, it, expect, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import CustomerPool from './CustomerPool'

const poolCustomers = [
  {
    id: 'cp1', code: 'C-2026-0001', name: '公海客户A', industry: '互联网', source: '转介绍',
    level: 'A', owner_id: '0', pool_reason: '超期未跟进自动回收',
    last_followed_at: '2026-01-01T00:00:00Z', created_at: '2026-01-01T00:00:00Z',
  },
]

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListPool: vi.fn(() =>
      Promise.resolve({ list: poolCustomers, total: 1, page: 1, size: 20 }),
    ),
    apiGetPoolSettings: vi.fn(() =>
      Promise.resolve({
        id: 1, enabled: false, max_claim_per_sales: 50,
        idle_days_no_follow: 30, idle_days_no_deal: 60, protect_days: 7,
        updated_at: '2026-01-01T00:00:00Z',
      }),
    ),
    apiPoolLogs: vi.fn(() => Promise.resolve([])),
    apiRecyclePool: vi.fn((dry: boolean) =>
      Promise.resolve(
        dry
          ? { total: 1, items: [{ customer_id: 'cp1', name: '公海客户A', reason: '超期未跟进自动回收' }] }
          : { total: 1, items: [] },
      ),
    ),
    apiClaimCustomer: vi.fn(() => Promise.resolve({ message: '领取成功' })),
  }),
)

describe('CustomerPool 公海池页面', () => {
  it('渲染不崩溃并显示标题', () => {
    renderWithProviders(<CustomerPool />)
    expect(screen.getByText('客户公海池')).toBeInTheDocument()
  })

  it('管理员可见「试算回收」「回收规则」按钮，且未启用规则时给出提示', async () => {
    renderWithProviders(<CustomerPool />)
    expect(await screen.findByText('试算回收')).toBeInTheDocument()
    expect(screen.getByText('回收规则')).toBeInTheDocument()
    expect(screen.getByText(/自动回收未启用/)).toBeInTheDocument()
  })

  it('点击「试算回收」展示回收预览', async () => {
    const user = userEvent.setup()
    renderWithProviders(<CustomerPool />)
    await user.click(await screen.findByText('试算回收'))
    expect(await screen.findByText(/试算结果：1 个客户符合回收条件/)).toBeInTheDocument()
    expect(screen.getByText('公海客户A（超期未跟进自动回收）')).toBeInTheDocument()
  })

  it('点击「流水」打开流水弹窗', async () => {
    const user = userEvent.setup()
    renderWithProviders(<CustomerPool />)
    await waitFor(() => expect(screen.getByText('公海客户A')).toBeInTheDocument())
    const flowBtns = await screen.findAllByText('流水')
    await user.click(flowBtns[0])
    expect(await screen.findByText(/公海流水「公海客户A」/)).toBeInTheDocument()
  })

  it('销售角色不显示「回收规则」入口（canManage 守卫）', async () => {
    vi.mocked((await import('../api')).apiMe).mockImplementation(
      () => Promise.resolve({ id: 'u2', name: '销售员', role: 'sales', dept: 'BD' }) as never,
    )
    renderWithProviders(<CustomerPool />)
    expect(await screen.findByText('客户公海池')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('回收规则')).not.toBeInTheDocument())
  })
})
