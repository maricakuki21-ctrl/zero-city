type LocaleMessageObject = Record<string, unknown>

// Preserve the statically known keys of every locale module while the runtime
 // merge still accepts arbitrary nested message objects. Returning a plain
 // Record<string, unknown> makes every nested property unknown, which in turn
 // breaks locale modules that intentionally extend another locale.
type UnionToIntersection<T> = (T extends unknown ? (value: T) => void : never) extends (
  value: infer Intersection,
) => void
  ? Intersection
  : never

type MergedLocaleMessages<Sources extends readonly LocaleMessageObject[]> = UnionToIntersection<Sources[number]>

function isMessageObject(value: unknown): value is LocaleMessageObject {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function mergeInto(target: LocaleMessageObject, source: LocaleMessageObject): LocaleMessageObject {
  for (const [key, value] of Object.entries(source)) {
    if (isMessageObject(value)) {
      const current = isMessageObject(target[key]) ? target[key] : {}
      target[key] = mergeInto({ ...current }, value)
    } else {
      target[key] = value
    }
  }

  return target
}

/**
 * Deeply composes locale modules. Later sources override translated leaves while
 * preserving new keys introduced by the official modular locale.
 */
export function mergeLocaleMessages<const Sources extends readonly LocaleMessageObject[]>(
  ...sources: Sources
): MergedLocaleMessages<Sources> {
  return sources.reduce<LocaleMessageObject>((messages, source) => mergeInto(messages, source), {}) as MergedLocaleMessages<Sources>
}
