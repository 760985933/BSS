import { vi } from 'vitest'

type AnyFn = (...args: unknown[]) => unknown

// 构造 ../api 的 mock：保留类型/常量（DEAL_STAGES 等），把各组件用到的异步函数
// 默认置为返回空数据的 vi.fn，测试内可用 overrides 覆盖为具体数据。
export async function buildApiMock(
  importOriginal: () => Promise<Record<string, unknown>>,
  overrides: Record<string, unknown> = {},
) {
  const actual = (await importOriginal()) as Record<string, unknown>
  const ok = <T,>(v: T): Promise<T> => Promise.resolve(v)

  const base: Record<string, unknown> = {
    ...actual,
    apiMe: vi.fn(() => ok({ id: 'u1', name: '管理员', role: 'admin', dept: 'HQ' })),
    apiListEmployees: vi.fn(() => ok({ list: [], total: 0, page: 1, size: 100 })),
    apiCreateEmployee: vi.fn(() => ok({ employee: {}, initial_password: 'Bss@1234' })),
    apiUpdateEmployee: vi.fn(() => ok({ message: 'ok' })),
    apiSetEmployeeStatus: vi.fn(() => ok({ message: 'ok' })),
    apiResetEmployeePassword: vi.fn(() => ok({ initial_password: 'Bss@1234' })),
    apiListDicts: vi.fn(() => ok([])),
    apiListCustomers: vi.fn(() => ok({ list: [], total: 0, page: 1, size: 20 })),
    apiGetCustomer: vi.fn(() =>
      ok({
        id: 'c1', code: 'C-2026-0001', name: '示例客户', industry: '', source: '',
        level: '', remark: '', owner_id: 'u1', created_at: '2026-01-01T00:00:00Z',
      }),
    ),
    apiListContacts: vi.fn(() => ok([])),
    apiCreateContact: vi.fn(() => ok({})),
    apiUpdateContact: vi.fn(() => ok({ message: 'ok' })),
    apiDeleteContact: vi.fn(() => ok({ message: 'ok' })),
    apiListDeals: vi.fn(() => ok({ list: [], total: 0, page: 1, size: 20 })),
    apiDealForecast: vi.fn(() => ok({ open_count: 0, total_cent: 0, weighted_cent: 0 })),
    apiCreateDeal: vi.fn(() => ok({})),
    apiUpdateDeal: vi.fn(() => ok({ message: 'ok' })),
    apiChangeDealStatus: vi.fn(() => ok({})),
    apiDeleteDeal: vi.fn(() => ok({ message: 'ok' })),
    apiCreateCustomer: vi.fn(() => ok({})),
    apiUpdateCustomer: vi.fn(() => ok({ message: 'ok' })),
    apiDeleteCustomer: vi.fn(() => ok({ message: 'ok' })),
    apiTransferCustomer: vi.fn(() => ok({ message: 'ok' })),
  }

  // 仅覆盖调用方真正需要的函数，其余保持 actual 原样
  for (const [k, v] of Object.entries(overrides)) base[k] = v as AnyFn
  return base
}
