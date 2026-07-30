import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import NotificationBell from './NotificationBell'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiUnreadCount: vi.fn(() => Promise.resolve({ count: 3 })),
    apiListNotifications: vi.fn(() =>
      Promise.resolve({
        items: [
          { id: 'n1', user_id: 'u1', type: 'contract_expiring', title: '合同 HT-1 将于 2026-08-01 到期', content: '关注续签', entity_type: 'contract', entity_id: 'c1', is_read: false, created_at: '2026-07-30T00:00:00Z' },
        ],
        total: 1, page: 1, size: 10,
      }),
    ),
    apiMarkNotificationRead: vi.fn(() => Promise.resolve({ message: 'ok' })),
    apiMarkAllRead: vi.fn(() => Promise.resolve({ message: 'ok' })),
  }),
)

describe('NotificationBell 通知中心', () => {
  it('渲染铃铛且不崩溃，未读角标展示数量', async () => {
    renderWithProviders(<NotificationBell />)
    // 未读数为 3，Badge 将渲染 3
    expect(await screen.findByText('3')).toBeInTheDocument()
  })

  it('打开下拉后展示通知列表与标记已读入口', async () => {
    const user = userEvent.setup()
    renderWithProviders(<NotificationBell />)
    // 点击铃铛打开面板
    await user.click(await screen.findByRole('img', { name: /bell/i }))
    expect(await screen.findByText(/合同 HT-1 将于/)).toBeInTheDocument()
    expect(screen.getByText('标记已读')).toBeInTheDocument()
  })
})
