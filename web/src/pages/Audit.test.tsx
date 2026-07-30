import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import Audit from './Audit'
import { renderWithProviders } from '../test/render'

// 桩出组件用到的 api，避免 importOriginal 的 hoist 顺序问题
vi.mock('../api', () => ({
  apiListAuditLogs: vi.fn(() =>
    Promise.resolve({
      list: [
        {
          id: '1', entity_type: 'contract', entity_id: '10', action: 'offboard',
          operator_id: '1', before_json: '', after_json: '{"successor_id":2}',
          created_at: '2026-07-30T10:00:00Z',
        },
      ],
      total: 1, page: 1, size: 20,
    }),
  ),
  apiMe: vi.fn(() => Promise.resolve({ id: 'u1', name: '管理员', role: 'admin', dept: 'HQ' })),
  AUDIT_ACTION: {
    create: { label: '新建', color: 'blue' },
    update: { label: '修改', color: 'gold' },
    delete: { label: '删除', color: 'red' },
    transfer: { label: '转移', color: 'cyan' },
    status_change: { label: '状态变更', color: 'geekblue' },
    offboard: { label: '离职交接', color: 'purple' },
  },
}))

describe('Audit 页面', () => {
  it('渲染不崩溃并显示审计查询标题', async () => {
    renderWithProviders(<Audit />)
    expect(await screen.findByText('查询')).toBeInTheDocument()
  })

  it('展示审计记录与离职交接标签', async () => {
    renderWithProviders(<Audit />)
    expect(await screen.findByText('离职交接')).toBeInTheDocument()
    expect(screen.getByText(/合同 #10/)).toBeInTheDocument()
  })
})
