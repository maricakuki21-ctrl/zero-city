export function isDraftRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function isStringList(value: unknown): value is string[] {
  return Array.isArray(value) && value.every(item => typeof item === 'string')
}

export function localDraftSerializer<T>(accepts: (value: unknown) => value is T) {
  return {
    read(raw: string): T {
      const value: unknown = JSON.parse(raw)
      if (!accepts(value)) throw new Error('本地草稿格式不兼容，未载入此记录。')
      return value
    },
    write(value: T): string { return JSON.stringify(value) },
  }
}
