// Node 25+ exposes an experimental global localStorage accessor that returns
// undefined unless --localstorage-file is configured. Vitest 2's jsdom global
// bridge sees the accessor and does not replace it, so modules imported during
// test collection can crash before a test has a chance to install a mock.
// Keep tests deterministic and independent of the host Node version.
class MemoryStorage implements Storage {
  private readonly values = new Map<string, string>()

  get length(): number {
    return this.values.size
  }

  clear(): void {
    this.values.clear()
  }

  getItem(key: string): string | null {
    return this.values.get(String(key)) ?? null
  }

  key(index: number): string | null {
    return Array.from(this.values.keys())[index] ?? null
  }

  removeItem(key: string): void {
    this.values.delete(String(key))
  }

  setItem(key: string, value: string): void {
    this.values.set(String(key), String(value))
  }
}

const hasUsableStorage = (value: unknown): value is Storage => {
  if (!value || typeof value !== 'object') return false
  const storage = value as Partial<Storage>
  return typeof storage.getItem === 'function' &&
    typeof storage.setItem === 'function' &&
    typeof storage.removeItem === 'function' &&
    typeof storage.clear === 'function'
}

if (!hasUsableStorage(globalThis.localStorage)) {
  const storage = new MemoryStorage()
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    enumerable: true,
    value: storage
  })
  if (typeof window !== 'undefined' && !hasUsableStorage(window.localStorage)) {
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      enumerable: true,
      value: storage
    })
  }
}
