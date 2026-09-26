import type { CapabilityAsset, TavernRoom, ZeroCityPublicProfile } from '@/features/bizdecipher/api/bizdecipher'
import type { CommunityComment, CommunityPost } from '@/features/bizdecipher/api/community'

export type ZeroCityObjectKind = 'space' | 'content' | 'room' | 'asset' | 'action'
export type ZeroCityExecutionTarget = 'workbench'
export type ZeroCityRoomReplayStatus = 'completed' | 'cancelled'

export interface ZeroCityDiscussionDiscovery {
  readonly space: 'all-city'
  readonly content: readonly CommunityPost[]
}

export interface ZeroCityRoomReplay {
  readonly roomID: number
  readonly status: ZeroCityRoomReplayStatus
  readonly persistedRoom: TavernRoom
}

export interface ZeroCityIdentityHistory {
  readonly identity: ZeroCityPublicProfile
  readonly history: readonly CommunityPost[]
}

export interface ZeroCityAssetDiscovery {
  readonly asset: CapabilityAsset
  readonly entries: readonly ZeroCityExecutionEntry[]
}

export interface ZeroCityExecutionEntry {
  readonly target: ZeroCityExecutionTarget
  readonly path: string
  readonly query: Readonly<Record<string, string>>
  readonly activation: 'explicit-user-navigation'
}

export interface ZeroCityJourneyGateway {
  readonly listPosts: (query: string) => Promise<readonly CommunityPost[]>
  readonly createComment: (postID: number, body: string) => Promise<CommunityComment>
  readonly joinRoom: (roomID: number) => Promise<TavernRoom>
  readonly completeRoom: (roomID: number) => Promise<TavernRoom>
  readonly listMyRooms: () => Promise<readonly TavernRoom[]>
  readonly createRoomPost: (room: TavernRoom, body: string) => Promise<CommunityPost>
  readonly getIdentity: (identity: string | number) => Promise<ZeroCityPublicProfile>
  readonly getAsset: (assetID: number) => Promise<CapabilityAsset>
}
