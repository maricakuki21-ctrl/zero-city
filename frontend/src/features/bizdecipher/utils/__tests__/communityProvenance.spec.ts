import { describe, expect, it } from 'vitest'
import { isOfficialCommunityPost, isResidentProposal } from '../communityProvenance'

describe('community provenance', () => {
  it('does not turn a player proposal into an official announcement based on district', () => {
    const post = { kind: 'feedback', district: 'governance', channel: 'votes' }
    expect(isResidentProposal(post)).toBe(true)
    expect(isOfficialCommunityPost(post, false)).toBe(false)
  })
  it('keeps official announcements and actual official confirmations discoverable', () => {
    const official = { kind: 'announcement', district: 'governance', channel: 'votes' }
    expect(isResidentProposal(official)).toBe(false)
    expect(isOfficialCommunityPost(official, false)).toBe(true)
    expect(isOfficialCommunityPost({ kind: 'feedback' }, true)).toBe(true)
  })
})
