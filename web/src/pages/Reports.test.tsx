import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import Reports from './Reports'
import { renderWithProviders } from '../test/render'

// 直接桩出组件用到的 api 函数与常量，避免 importOriginal 的 hoist 顺序问题
vi.mock('../api', () => ({
  apiSignTrend: vi.fn(() =>
    Promise.resolve({
      rows: [
        { month: '2026-06', amount_cent: 10000 },
        { month: '2026-07', amount_cent: 30000 },
      ],
    }),
  ),
  apiPaymentTrend: vi.fn(() => Promise.resolve({ rows: [{ month: '2026-07', amount_cent: 8000 }] })),
  apiSalesRank: vi.fn(() =>
    Promise.resolve({
      rows: [
        { owner_id: 'u2', owner_name: '乙', won_deals: 2, signed_cent: 20000, paid_cent: 5000 },
        { owner_id: 'u1', owner_name: '甲', won_deals: 1, signed_cent: 10000, paid_cent: 3000 },
      ],
    }),
  ),
  apiFunnel: vi.fn(() =>
    Promise.resolve({
      rows: [
        { stage: 'prospecting', label: '线索', count: 1, amount_cent: 1000 },
        { stage: 'won', label: '赢单', count: 3, amount_cent: 21000 },
      ],
    }),
  ),
  apiExportReport: vi.fn(() => Promise.resolve(new Blob(['\uFEFF月份,签约金额(元)'], { type: 'text/csv' }))),
  REPORT_LABEL: {
    sign_trend: '月度签约趋势',
    payment_trend: '月度回款趋势',
    sales_rank: '销售排行',
    funnel: '客户转化漏斗',
  },
  fenToYuan: (cent: number) => (cent / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 }),
}))

describe('Reports 页面', () => {
  it('渲染不崩溃并显示报表中心标题', async () => {
    renderWithProviders(<Reports />)
    expect(await screen.findByText('报表中心（数据范围按您的权限过滤）')).toBeInTheDocument()
  })

  it('签约趋势 Tab 展示金额与导出按钮', async () => {
    renderWithProviders(<Reports />)
    expect(await screen.findByText('2026-07')).toBeInTheDocument()
    expect(screen.getByText('导出当前报表 CSV')).toBeInTheDocument()
  })
})
