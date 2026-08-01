import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import PayrollPage from './Payroll'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListPayrolls: vi.fn(() => Promise.resolve([])),
    apiListEmployees: vi.fn(() => Promise.resolve({ list: [], total: 0, page: 1, size: 10 })),
  }),
)

describe('Payroll 页面', () => {
  it('渲染不崩溃并显示薪酬管理操作区', () => {
    renderWithProviders(<PayrollPage />)
    expect(screen.getByText('生成当月薪资')).toBeInTheDocument()
    expect(screen.getByText('导出工资条')).toBeInTheDocument()
  })
})
