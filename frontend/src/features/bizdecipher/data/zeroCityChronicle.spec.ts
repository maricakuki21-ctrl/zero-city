import { existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { ZERO_CITY_CARD_MANIFEST } from '@/constants/zeroCityCardManifest'
import { chronicleCardLink, chronicleDistricts, chronicleEras, chronicleEvents, chroniclePortrait, chronicleResident, chronicleResidents, eventsForResident, filterChronicleEvents } from './zeroCityChronicle'

describe('city chronicle canon', () => {
  it('uses existing collectible identities and actual bundled art', () => {
    expect(chronicleResidents).toHaveLength(8)
    for (const resident of chronicleResidents) {
      expect(ZERO_CITY_CARD_MANIFEST[resident.key]).toBeDefined()
      expect(existsSync(resolve(process.cwd(), 'public', chroniclePortrait(resident.key).slice(1)))).toBe(true)
      expect(chronicleCardLink(resident.key)).toBe(`/zero-city/cards?card=${resident.key}`)
      expect(resident.wish.length).toBeGreaterThan(12)
      expect(resident.fear.length).toBeGreaterThan(12)
      expect(resident.secret.length).toBeGreaterThan(12)
    }
  })

  it('has unique story, district, era and resident identifiers', () => {
    for (const ids of [chronicleEvents.map(x => x.id), chronicleDistricts.map(x => x.id), chronicleEras.map(x => x.id), chronicleResidents.map(x => x.key)]) {
      expect(new Set(ids).size).toBe(ids.length)
    }
  })

  it('never leaves characters, relations or scenes dangling', () => {
    for (const resident of chronicleResidents) {
      expect(chronicleDistricts.some(district => district.id === resident.district)).toBe(true)
      expect(chronicleResident(resident.relationship.key)).toBeDefined()
      expect(eventsForResident(resident.key).length).toBeGreaterThan(0)
    }
    for (const event of chronicleEvents) {
      expect(chronicleEras.some(era => era.id === event.era)).toBe(true)
      expect(chronicleDistricts.some(district => district.id === event.district)).toBe(true)
      expect(event.residents.every(key => Boolean(chronicleResident(key)))).toBe(true)
      expect(event.paragraphs).toHaveLength(3)
      expect(event.date).toMatch(/^零历/)
    }
  })

  it('covers all eras and districts with concrete stories', () => {
    for (const era of chronicleEras) expect(chronicleEvents.some(event => event.era === era.id)).toBe(true)
    for (const district of chronicleDistricts) expect(chronicleEvents.some(event => event.district === district.id)).toBe(true)
  })

  it('searches story prose, places, names and canonical factions', () => {
    expect(filterChronicleEvents('铆钉').map(x => x.id)).toContain('a-button-left-in-place')
    expect(filterChronicleEvents('蓝时海岸').map(x => x.id)).toEqual(['the-answer-across-water'])
    expect(filterChronicleEvents('低电量小人').length).toBeGreaterThan(1)
    expect(filterChronicleEvents(' 暮色地图局 ').length).toBeGreaterThan(0)
  })

  it('combines era with search without falling back to unrelated events', () => {
    expect(filterChronicleEvents('', 'bridge')).toHaveLength(2)
    expect(filterChronicleEvents('电台', 'before')).toEqual([])
    expect(filterChronicleEvents('不存在的故事')).toEqual([])
    expect(filterChronicleEvents('   ')).toHaveLength(chronicleEvents.length)
  })

  it('does not treat unknown query values as valid characters', () => {
    expect(chronicleResident('<script>')).toBeUndefined()
    expect(chronicleResident(undefined)).toBeUndefined()
  })
})
