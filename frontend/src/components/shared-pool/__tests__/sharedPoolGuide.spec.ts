import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import enMessages from '@/i18n/locales/en/custom'
import zhMessages from '@/i18n/locales/zh/custom'
import {
  getSharedPoolGuideSteps,
  hasSeenSharedPoolGuide,
  markSharedPoolGuideSeen,
  sharedPoolGuideStorageKey,
} from '@/features/bizdecipher/components/shared-pool/sharedPoolGuide'

describe('shared pool guide', () => {
  const t = (key: string) => key

  const translate = (messages: Record<string, unknown>, key: string): string => {
    const value = key.split('.').reduce<unknown>((current, segment) => {
      if (!current || typeof current !== 'object') return undefined
      return (current as Record<string, unknown>)[segment]
    }, messages)
    return typeof value === 'string' ? value : key
  }

  it('keeps both market and owner tours short and complete', () => {
    const market = getSharedPoolGuideSteps('market', t)
    const owner = getSharedPoolGuideSteps('owner', t)

    expect(market).toHaveLength(6)
    expect(owner).toHaveLength(5)
    expect(market.map((item) => item.popover?.title)).toEqual([
      'sharedPoolGuide.market.welcome.title',
      'sharedPoolGuide.market.browse.title',
      'sharedPoolGuide.market.member.title',
      'sharedPoolGuide.market.owner.title',
      'sharedPoolGuide.market.pricing.title',
      'sharedPoolGuide.market.safety.title',
    ])
    expect(owner.map((item) => item.popover?.title)).toContain('sharedPoolGuide.owner.earnings.title')
  })

  it('isolates first-run state by user and guide scope', () => {
    const values = new Map<string, string>()
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
    }

    expect(sharedPoolGuideStorageKey('market', 7)).not.toBe(sharedPoolGuideStorageKey('market', 8))
    expect(sharedPoolGuideStorageKey('market', 7)).not.toBe(sharedPoolGuideStorageKey('owner', 7))
    expect(hasSeenSharedPoolGuide(storage, 'market', 7)).toBe(false)

    markSharedPoolGuideSeen(storage, 'market', 7)

    expect(hasSeenSharedPoolGuide(storage, 'market', 7)).toBe(true)
    expect(hasSeenSharedPoolGuide(storage, 'owner', 7)).toBe(false)
    expect(hasSeenSharedPoolGuide(storage, 'market', 8)).toBe(false)
  })

  it('provides complete Chinese and English copy for every step', () => {
    for (const messages of [zhMessages, enMessages]) {
      const localized = (key: string) => translate(messages, key)
      const steps = [
        ...getSharedPoolGuideSteps('market', localized),
        ...getSharedPoolGuideSteps('owner', localized),
      ]

      for (const item of steps) {
        expect(item.popover?.title).not.toMatch(/^sharedPoolGuide\./)
        expect(item.popover?.description).not.toMatch(/^sharedPoolGuide\./)
      }
    }
  })

  it('uses selectors that exist in the real market and owner views', () => {
    const sources = {
      market: readFileSync(resolve(process.cwd(), 'src/features/bizdecipher/views/user/AccountSquareView.vue'), 'utf8'),
      owner: readFileSync(resolve(process.cwd(), 'src/features/bizdecipher/views/user/AccountSquareOwnerView.vue'), 'utf8'),
    }

    for (const scope of ['market', 'owner'] as const) {
      for (const item of getSharedPoolGuideSteps(scope, t)) {
        if (typeof item.element !== 'string') continue
        const selector = item.element.match(/^\[data-tour="([^"]+)"\]$/)
        expect(selector, `invalid ${scope} guide selector: ${item.element}`).not.toBeNull()
        expect(sources[scope]).toContain(`data-tour="${selector?.[1]}"`)
      }
    }
  })
})
