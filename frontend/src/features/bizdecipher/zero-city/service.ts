import {
  completeTavernRoom,
  getCapabilityAsset,
  getZeroCityPublicProfile,
  joinTavernRoom,
  listMyTavernRooms,
  type TavernRoom,
} from '@/features/bizdecipher/api/bizdecipher'
import { communityAPI, type CommunityComment, type CommunityPost } from '@/features/bizdecipher/api/community'
import {
  type ZeroCityAssetDiscovery,
  type ZeroCityDiscussionDiscovery,
  type ZeroCityIdentityHistory,
  type ZeroCityJourneyGateway,
  type ZeroCityRoomReplay,
} from './contracts'
import { executionEntryForAsset } from './moduleDescriptor'

const completedRoomStatuses = new Set(['completed', 'cancelled'])

function roomReplayStatus(room: TavernRoom): ZeroCityRoomReplay['status'] {
  if (room.status === 'completed') return 'completed'
  if (room.status === 'cancelled') return 'cancelled'
  throw new Error(`Room ${room.id} is not replayable after status ${room.status}`)
}

function persistedReplay(roomID: number, rooms: readonly TavernRoom[]): ZeroCityRoomReplay {
  const room = rooms.find((candidate) => candidate.id === roomID)
  if (!room || !completedRoomStatuses.has(room.status)) {
    throw new Error(`Room ${roomID} is not persisted as a completed replay`)
  }
  return {
    roomID,
    status: roomReplayStatus(room),
    persistedRoom: room,
  }
}
export const liveZeroCityJourneyGateway: ZeroCityJourneyGateway = {
  async listPosts(query) {
    const response = await communityAPI.listPosts({ limit: 50 })
    const normalizedQuery = query.trim().toLocaleLowerCase()
    if (!normalizedQuery) return response.items
    return response.items.filter((post) => `${post.title}\n${post.body}\n${post.tags.join(' ')}`.toLocaleLowerCase().includes(normalizedQuery))
  },
  createComment: (postID, body) => communityAPI.createComment(postID, { body }),
  joinRoom: joinTavernRoom,
  completeRoom: completeTavernRoom,
  listMyRooms: () => listMyTavernRooms({ limit: 50 }),
  createRoomPost: (room, body) => communityAPI.createPost({
    kind: 'feedback',
    title: room.title,
    body,
    tags: ['tavern-replay'],
    district: 'tavern',
    channel: 'chat-hall',
    source_type: 'tavern_room',
    source_id: String(room.id),
    scenario: 'delivery_collaboration',
    action_type: 'share_signal',
  }),
  getIdentity: getZeroCityPublicProfile,
  getAsset: getCapabilityAsset,
}

export class ZeroCityJourneys {
  constructor(private readonly gateway: ZeroCityJourneyGateway = liveZeroCityJourneyGateway) {}

  async discoverDiscussion(query: string): Promise<ZeroCityDiscussionDiscovery> {
    return {
      space: 'all-city',
      content: await this.gateway.listPosts(query),
    }
  }

  replyToDiscussion(postID: number, body: string): Promise<CommunityComment> {
    return this.gateway.createComment(postID, body)
  }

  joinRoom(roomID: number): Promise<TavernRoom> {
    return this.gateway.joinRoom(roomID)
  }

  async endRoomAndPersistReplay(roomID: number): Promise<ZeroCityRoomReplay> {
    const completed = await this.gateway.completeRoom(roomID)
    return persistedReplay(completed.id, await this.gateway.listMyRooms())
  }

  async convertRoomReplayToPost(room: TavernRoom, body: string): Promise<CommunityPost> {
    const replay = persistedReplay(room.id, await this.gateway.listMyRooms())
    return this.gateway.createRoomPost(replay.persistedRoom, body)
  }

  async inspectIdentityHistory(identity: string | number): Promise<ZeroCityIdentityHistory> {
    const identityRecord = await this.gateway.getIdentity(identity)
    return {
      identity: identityRecord,
      history: identityRecord.posts,
    }
  }

  async discoverAsset(assetID: number): Promise<ZeroCityAssetDiscovery> {
    const asset = await this.gateway.getAsset(assetID)
    return {
      asset,
      entries: [executionEntryForAsset(asset, 'workbench')],
    }
  }
}
