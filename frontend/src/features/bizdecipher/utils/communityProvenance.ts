interface CommunityPostOrigin {
  kind?: string
  district?: string
  channel?: string
}

export function isResidentProposal(post: CommunityPostOrigin): boolean {
  return post.district === 'governance' && post.channel === 'votes' && post.kind !== 'announcement'
}

export function isOfficialCommunityPost(post: CommunityPostOrigin, officiallyConfirmed: boolean): boolean {
  return post.kind === 'announcement' || officiallyConfirmed
}
