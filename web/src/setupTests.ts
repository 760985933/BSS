import '@testing-library/jest-dom/vitest'
import { vi } from 'vitest'

// ---------- jsdom 补齐：AntD 运行所需但 jsdom 未实现的 API ----------
if (!window.matchMedia) {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }) as unknown as MediaQueryList
}

class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = ResizeObserver
;(globalThis as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = ResizeObserver

if (!HTMLElement.prototype.scrollIntoView) {
  HTMLElement.prototype.scrollIntoView = vi.fn()
}
