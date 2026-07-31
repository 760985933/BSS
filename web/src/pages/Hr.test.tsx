import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Hr from './Hr'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListLaborContracts: vi.fn(() => Promise.resolve([])),
    apiListOnboardings: vi.fn(() => Promise.resolve([])),
    apiListEmployees: vi.fn(() => Promise.resolve({ list: [], total: 0, page: 1, size: 10 })),
    apiListCandidates: vi.fn(() => Promise.resolve([])),
  }),
)

describe('Hr 页面', () => {
  it('渲染不崩溃并显示劳动合同与入职管理标签', () => {
    renderWithProviders(<Hr />)
    expect(screen.getByText('劳动合同')).toBeInTheDocument()
    expect(screen.getByText('入职管理')).toBeInTheDocument()
  })
})
