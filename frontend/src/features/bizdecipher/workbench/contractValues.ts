export type UnknownRecord = Readonly<Record<string, unknown>>

export function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function readField(record: UnknownRecord, ...keys: readonly string[]): unknown {
  for (const key of keys) {
    if (key in record) return record[key]
  }
  return undefined
}

export function readString(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value.trim() : fallback
}

export function readExactDecimal(value: unknown): string | null {
  if (typeof value !== 'string') return null
  return /^(?:0|[1-9]\d{0,11})(?:\.\d{1,12})?$/.test(value) ? value : null
}

export function readBoolean(value: unknown, fallback = false): boolean {
  return typeof value === 'boolean' ? value : fallback
}

export function readUnsignedInteger(value: unknown, fallback = 0): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) return fallback
  return value
}

export function readIntegerText(value: unknown): string {
  if (typeof value === 'string' && /^\d+$/.test(value)) return value
  if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) return String(value)
  return ''
}

export function readTimestamp(value: unknown): string {
  const raw = readString(value)
  if (!raw) return ''
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? '' : date.toISOString()
}

export function readArray(value: unknown): readonly unknown[] {
  return Array.isArray(value) ? value : []
}

export function readCursor(value: unknown): string {
  if (typeof value === 'string' && /^\d+$/.test(value)) return value
  if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) return String(value)
  if (!isRecord(value)) return ''
  return readCursor(readField(value, 'seq', 'cursor', 'last_event_id'))
}

export function unwrapApiData(value: unknown): unknown {
  if (!isRecord(value)) return value
  const data = readField(value, 'data')
  return data === undefined ? value : data
}

export function readNestedRecord(record: UnknownRecord, ...keys: readonly string[]): UnknownRecord {
  const value = readField(record, ...keys)
  return isRecord(value) ? value : {}
}
