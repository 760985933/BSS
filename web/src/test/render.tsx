import { ReactElement } from 'react'
import { render, RenderResult } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { App as AntApp } from 'antd'
import { vi, expect } from 'vitest'

export function makeClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  })
}

// 统一包装：react-query + router + AntD App（提供 message/modal 上下文）
export function renderWithProviders(
  ui: ReactElement,
  options: { route?: string; client?: QueryClient } = {},
): RenderResult & { client: QueryClient } {
  const client = options.client ?? makeClient()
  const route = options.route ?? '/'
  const utils = render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[route]}>
        <AntApp>{ui}</AntApp>
      </MemoryRouter>
    </QueryClientProvider>,
  )
  return { ...utils, client }
}

// 捕获 console.warn/error 中 rc-field-form 的 “not connected to any Form element” 警告，
// 用于锁死「useForm 实例未连接 Form」类回归（常见于 Modal destroyOnClose + 关闭态调用表单方法）。
export function useFormWarningSpy() {
  const msgs: string[] = []
  const w = vi.spyOn(console, 'warn').mockImplementation((...a) => msgs.push(a.map(String).join(' ')))
  const e = vi.spyOn(console, 'error').mockImplementation((...a) => msgs.push(a.map(String).join(' ')))
  return {
    assertNoUseFormWarning: () => {
      const hit = msgs.filter((m) => m.includes('not connected to any Form element'))
      expect(hit, `不应出现 useForm 未连接警告，实际：${hit.join(' | ')}`).toEqual([])
    },
    restore: () => { w.mockRestore(); e.mockRestore() },
  }
}
