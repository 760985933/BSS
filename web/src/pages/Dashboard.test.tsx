import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Dashboard from './Dashboard'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiDashboard: vi.fn(() =>
      Promise.resolve({
        cards: {
          signed_this_month_cent: 100000,
          paid_this_month_cent: 30000,
          open_deals: 5,
          overdue_amount_cent: 12000,
        },
        expiring_contracts: [
          { id: 'c1', code: 'HT-1', title: '合同A', customer: '客户甲', amount_cent: 5000, expire_date: '2026-08-01', status: 'signed' },
        ],
        overdue_plans: [
          { id: 'p1', contract_code: 'HT-1', period_no: 1, due_date: '2026-07-01', amount_cent: 20000, paid_cent: 8000, outstanding_cent: 12000 },
        ],
        recent_won_deals: [
          { id: 'd1', code: 'SD-1', title: '赢单A', customer: '客户甲', amount_cent: 9000, probability: 100, status: 'won' },
        ],
      }),
    ),
  }),
)

describe('Dashboard 页面', () => {
  it('渲染不崩溃，4 张卡片标题出现', async () => {
    renderWithProviders(<Dashboard />)
    expect(await screen.findByText('本月签约金额')).toBeInTheDocument()
    expect(screen.getByText('本月回款金额')).toBeInTheDocument()
    expect(screen.getByText('进行中商单')).toBeInTheDocument()
    expect(screen.getByText('逾期回款金额')).toBeInTheDocument()
  })

  it('三个列表均渲染且含样例数据', async () => {
    renderWithProviders(<Dashboard />)
    expect(await screen.findByText('即将到期合同')).toBeInTheDocument()
    expect(screen.getByText('逾期回款')).toBeInTheDocument()
    expect(screen.getByText('近期赢单')).toBeInTheDocument()
    expect(screen.getAllByText('HT-1').length).toBeGreaterThan(0)
    expect(screen.getByText('SD-1')).toBeInTheDocument()
  })
})
