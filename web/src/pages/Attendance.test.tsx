import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Attendance from './Attendance'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListSchedules: vi.fn(() => Promise.resolve([])),
    apiListLeaveRequests: vi.fn(() => Promise.resolve([])),
    apiListAttendances: vi.fn(() => Promise.resolve([])),
    apiListEmployees: vi.fn(() => Promise.resolve({ list: [], total: 0, page: 1, size: 10 })),
  }),
)

describe('Attendance 页面', () => {
  it('渲染不崩溃并显示排班、请假、考勤记录标签', () => {
    renderWithProviders(<Attendance />)
    expect(screen.getByText('排班')).toBeInTheDocument()
    expect(screen.getByText('请假')).toBeInTheDocument()
    expect(screen.getByText('考勤记录')).toBeInTheDocument()
  })
})
