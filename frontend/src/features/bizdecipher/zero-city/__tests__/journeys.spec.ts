import { describe, expect, it } from 'vitest'
import type { CapabilityAsset, TavernRoom, ZeroCityPublicProfile } from '@/features/bizdecipher/api/bizdecipher'
import type { CommunityComment, CommunityPost } from '@/features/bizdecipher/api/community'
import { type ZeroCityJourneyGateway } from '../contracts'
import { zeroCityModuleDescriptor } from '../moduleDescriptor'
import { ZeroCityJourneys } from '../service'

function post(id: number, title = '讨论'): CommunityPost {
  return {
    id,
    user_id: 7,
    author: 'resident',
    kind: 'support',
    title,
    body: '内容',
    tags: ['tag'],
    private: false,
    status: 'open',
    pinned: false,
    catches: 0,
    replies: 0,
    views: 0,
    created_at: '2026-09-02T00:00:00Z',
    updated_at: '2026-09-02T00:00:00Z',
  }
}

function room(id: number, status: TavernRoom['status'] = 'lobby'): TavernRoom {
  return {
    id,
    script_id: 13,
    owner_id: 7,
    owner: 'host',
    script_title: 'Script',
    title: 'Room',
    status,
    visibility: 'public',
    host_mode: 'human_host',
    billing_mode: 'free',
    entry_credit_cost: 0,
    entry_balance_cost: 0,
    max_players: 6,
    current_players: 2,
    current_user_joined: true,
    current_phase: 'closing',
    room_config: {},
    ended_at: status === 'completed' ? '2026-09-02T00:01:00Z' : null,
    created_at: '2026-09-02T00:00:00Z',
    updated_at: '2026-09-02T00:01:00Z',
  }
}

function asset(id: number): CapabilityAsset {
  return {
    id,
    user_id: 7,
    author: 'creator',
    title: 'Asset',
    slug: 'asset',
    summary: 'summary',
    description: 'description',
    asset_type: 'workflow',
    status: 'listed',
    tags: [],
    scenario_tags: [],
    integration_tags: [],
    cover_url: '',
    screenshot_urls: [],
    video_url: '',
    demo_url: '',
    doc_url: '',
    source_url: '',
    template_url: '',
    primary_action_type: 'view_workflow',
    pricing_type: 'free',
    contact_enabled: false,
    is_featured: false,
    featured_weight: 0,
    view_count: 0,
    like_count: 0,
    favorite_count: 0,
    download_count: 0,
    use_count: 0,
    liked_by_me: false,
    favorited_by_me: false,
    downloaded_by_me: false,
    viewed_today: false,
    comment_count: 0,
    rating_avg: 0,
    rating_count: 0,
    review_note: '',
    created_at: '2026-09-02T00:00:00Z',
    updated_at: '2026-09-02T00:00:00Z',
  }
}

function identity(posts: readonly CommunityPost[]): ZeroCityPublicProfile {
  return {
    profile: {
      user_id: 7,
      handle: 'resident',
      display_name: 'Resident',
      avatar_url: '',
      bio: '',
      created_at: '2026-09-02T00:00:00Z',
      updated_at: '2026-09-02T00:00:00Z',
    },
    stats: { followers: 0, following: 0, community_posts: posts.length, shared_pools: 0, collectible_cards: 0 },
    follow_state: { is_following: false, followers: 0, following: 0 },
    shared_pools: [],
    posts,
  }
}

function gatewayFixture(): { readonly gateway: ZeroCityJourneyGateway; readonly calls: string[] } {
  const calls: string[] = []
  const completed = room(41, 'completed')
  const gateway: ZeroCityJourneyGateway = {
    async listPosts() { calls.push('listPosts'); return [post(1, 'model notes')] },
    async createComment(postID, body): Promise<CommunityComment> {
      calls.push(`comment:${postID}:${body}`)
      return { id: 3, post_id: postID, user_id: 9, author: 'reply', body, helper_role: '', official: false, status: 'visible', created_at: '2026-09-02T00:00:00Z', updated_at: '2026-09-02T00:00:00Z' }
    },
    async joinRoom(roomID) { calls.push(`join:${roomID}`); return room(roomID, 'running') },
    async completeRoom(roomID) { calls.push(`complete:${roomID}`); return completed },
    async listMyRooms() { calls.push('listMyRooms'); return [completed] },
    async createRoomPost(sourceRoom, body) { calls.push(`roomPost:${sourceRoom.id}:${body}`); return post(8, 'replay') },
    async getIdentity() { calls.push('identity'); return identity([post(6, 'history')]) },
    async getAsset(assetID) { calls.push(`asset:${assetID}`); return asset(assetID) },
  }
  return { gateway, calls }
}

describe('ZeroCityJourneys', () => {
  it('finds a discussion and persists a reply through the community boundary', async () => {
    const fixture = gatewayFixture()
    const journeys = new ZeroCityJourneys(fixture.gateway)

    const discovery = await journeys.discoverDiscussion('model')
    const reply = await journeys.replyToDiscussion(discovery.content[0]?.id ?? 0, 'answer')

    expect(discovery.space).toBe('all-city')
    expect(discovery.content).toHaveLength(1)
    expect(reply.post_id).toBe(1)
    expect(fixture.calls).toEqual(['listPosts', 'comment:1:answer'])
  })

  it('joins, completes, persists and optionally converts a room replay without charging', async () => {
    const fixture = gatewayFixture()
    const journeys = new ZeroCityJourneys(fixture.gateway)

    const joined = await journeys.joinRoom(41)
    const replay = await journeys.endRoomAndPersistReplay(41)
    const converted = await journeys.convertRoomReplayToPost(replay.persistedRoom, 'room summary')

    expect(joined.status).toBe('running')
    expect(replay.status).toBe('completed')
    expect(converted.id).toBe(8)
    expect(fixture.calls).toEqual(['join:41', 'complete:41', 'listMyRooms', 'listMyRooms', 'roomPost:41:room summary'])
  })

  it('inspects identity history without deriving runtime or financial facts', async () => {
    const fixture = gatewayFixture()
    const journeys = new ZeroCityJourneys(fixture.gateway)

    const history = await journeys.inspectIdentityHistory(7)

    expect(history.identity.profile.user_id).toBe(7)
    expect(history.history.map((item) => item.id)).toEqual([6])
    expect(fixture.calls).toEqual(['identity'])
  })

  it('discovers an asset and exposes the actual Workbench route with its required context', async () => {
    const fixture = gatewayFixture()
    const journeys = new ZeroCityJourneys(fixture.gateway)

    const discovery = await journeys.discoverAsset(23)

    expect(discovery.asset.id).toBe(23)
    expect(discovery.entries).toEqual([
      { target: 'workbench', path: '/operator', query: { asset: '23', entry: 'zero-city-asset' }, activation: 'explicit-user-navigation' },
    ])
    expect(fixture.calls).toEqual(['asset:23'])
  })

  it('declares distinct Space Content Room Asset and Action objects for the approved Workbench wiring', () => {
    expect(zeroCityModuleDescriptor.objectKinds).toEqual(['space', 'content', 'room', 'asset', 'action'])
    expect(zeroCityModuleDescriptor.routeWiring).toEqual([
      {
        target: 'workbench',
        routeName: 'Operator',
        path: '/operator',
        owner: 'task-17-workbench-relay-wiring',
        requiredQuery: ['asset', 'entry'],
      },
    ])
  })
})
