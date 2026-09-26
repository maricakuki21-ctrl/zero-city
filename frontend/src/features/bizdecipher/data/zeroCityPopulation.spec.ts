import { describe, expect, it } from 'vitest'
import { ZERO_CITY_CARD_MANIFEST, zeroCityCardManifestEntries } from '@/constants/zeroCityCardManifest'
import { chronicleDistricts, chronicleResidents } from './zeroCityChronicle'
import {
  zeroCityCharacterAtlas,
  zeroCityCharacterEntry,
  zeroCityCharactersInDistrict,
  zeroCityFactionProfiles,
  zeroCityMainCastKeys,
} from './zeroCityPopulation'

describe('zero city population atlas', () => {
  it('gives every collectible identity one city home and one live conflict', () => {
    expect(zeroCityCharacterAtlas).toHaveLength(zeroCityCardManifestEntries().length)
    expect(zeroCityCharacterAtlas).toHaveLength(82)
    expect(new Set(zeroCityCharacterAtlas.map(entry => entry.key)).size).toBe(zeroCityCharacterAtlas.length)
    for (const entry of zeroCityCharacterAtlas) {
      expect(ZERO_CITY_CARD_MANIFEST[entry.key]).toBeDefined()
      expect(chronicleDistricts.some(district => district.id === entry.district)).toBe(true)
      expect(entry.duty.length).toBeGreaterThanOrEqual(10)
      expect(entry.pressure.length).toBeGreaterThanOrEqual(10)
      expect(entry.duty).not.toBe(entry.pressure)
    }
  })

  it('keeps the first-volume cast aligned with resident dossiers', () => {
    expect(new Set(zeroCityMainCastKeys)).toEqual(new Set(chronicleResidents.map(resident => resident.key)))
    for (const key of zeroCityMainCastKeys) expect(zeroCityCharacterEntry(key)?.narrativeLayer).toBe('main')
  })

  it('covers every manifest faction explicitly instead of guessing from its name', () => {
    const manifestFactions = new Set(zeroCityCardManifestEntries().map(entry => entry.definition.faction))
    expect(new Set(Object.keys(zeroCityFactionProfiles))).toEqual(manifestFactions)
    expect(manifestFactions.size).toBe(55)
  })

  it('populates every district and keeps mythic and legacy identities in the archive layer', () => {
    for (const district of chronicleDistricts) expect(zeroCityCharactersInDistrict(district.id).length).toBeGreaterThan(0)
    expect(Object.fromEntries(chronicleDistricts.map(district => [district.id, zeroCityCharactersInDistrict(district.id).length]))).toEqual({
      power: 13,
      shop: 24,
      workshop: 15,
      harbour: 11,
      archive: 19,
    })
    expect(Object.fromEntries(['main', 'recurring', 'resident', 'myth', 'legacy'].map(layer => [layer, zeroCityCharacterAtlas.filter(entry => entry.narrativeLayer === layer).length]))).toEqual({
      main: 8,
      recurring: 20,
      resident: 46,
      myth: 2,
      legacy: 6,
    })
    expect(zeroCityCharacterAtlas.filter(entry => entry.narrativeLayer === 'myth').map(entry => entry.faction)).toEqual(['零点核心', '零点核心'])
    expect(zeroCityCharacterAtlas.filter(entry => entry.narrativeLayer === 'legacy').every(entry => entry.faction === '旧档案盒')).toBe(true)
  })
})
