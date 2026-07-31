import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../test/render'
import { buildApiMock } from '../test/mockApi'
import Recruitment from './Recruitment'

vi.mock('../api', async (importOriginal) =>
  buildApiMock(importOriginal as () => Promise<Record<string, unknown>>, {
    apiListJobPosts: vi.fn(() => Promise.resolve([])),
    apiListCandidates: vi.fn(() => Promise.resolve([])),
    apiCandidatesFunnel: vi.fn(() => Promise.resolve([])),
  }),
)

describe('Recruitment 页面', () => {
  it('渲染不崩溃并显示招聘管理标题', () => {
    renderWithProviders(<Recruitment />)
    expect(screen.getByText('招聘管理')).toBeInTheDocument()
  })
})
