<template>
  <AppLayout>
    <div class="community-page zero-city-page space-y-6">
      <div class="zero-city-app-shell">
        <div class="zero-city-main space-y-6">
          <section v-if="!isCityHome" class="zero-city-workspace" :class="activeWorkspaceTone">
            <template v-if="isPersonalWorkspace">
              <ZeroCityPersonalHub
                :user="authStore.user"
                :history="zeroCityHistory"
                :history-error="myPostsLoadError"
                :posts="myPosts"
                :loading="isLoadingMyPosts"
                @refresh="loadMyCommunityPosts"
                @profile="router.push('/settings/account')"
                @cards="router.push('/zero-city/cards')"
                @assets="router.push('/assets')"
              />
            </template>

            <ZeroCityPollHall
              v-if="activeSurface?.kind === 'activity' && activeCommunityChannel === 'votes'"
              :key="`${activeCommunityDistrict}:${activeCommunityChannel}`"
              :focus-poll-id="Number(route.query.poll) || undefined"
            />
            <ZeroCityActivityBoard
              v-else-if="activeSurface?.kind === 'activity' && activeCommunityChannel !== 'badges' && activeCommunityChannel !== 'rules'"
              :key="`${activeCommunityDistrict}:${activeCommunityChannel}`"
              :kind="(activeCommunityChannel as 'votes' | 'badges' | 'rules')"
              :title="activeChannelTitle"
              :posts="workspacePosts"
              :loading="isLoadingPosts"
              :error="postLoadError"
              @create="jumpToComposer"
              @retry="loadCommunityPosts"
              @cards="router.push('/zero-city/cards')"
            />
            <ZeroCityGovernancePanel
              v-else-if="activeSurface?.kind === 'activity' && (activeCommunityChannel === 'badges' || activeCommunityChannel === 'rules')"
              :kind="activeCommunityChannel"
            />
            <ZeroCityForumBoard
              v-else-if="activeSurface"
              :key="`${activeCommunityDistrict}:${activeCommunityChannel}`"
              :title="activeChannelTitle"
              :district="activeWorkspaceTitle"
              :posts="workspacePosts"
              :loading="isLoadingPosts"
              :error="postLoadError"
              :excerpts="activeSurface.kind === 'plaza'"
              :channels="activeSurface.kind === 'plaza' ? activeWorkspaceChannels : undefined"
              :active-channel="activeCommunityChannel"
              :can-create="composerAllowed"
              :participation-note="forumParticipationNote"
              @create="jumpToComposer"
              @retry="loadCommunityPosts"
              @open="loadForumComments"
              @profile="openZeroCityProfile"
              @channel="activateDistrict('tavern', $event)"
            >
              <template #detail="{ post }">
                <div class="city-forum-meta">
                  <button v-if="post.user_id" type="button" @click="openZeroCityProfile(post.user_id)">{{ post.card }}</button>
                  <span v-else>{{ post.card }}</span>
                  <time :datetime="post.created_at">{{ post.time }}</time>
                  <span>{{ post.views }} 浏览</span>
                  <span v-for="tag in post.tags" :key="tag" class="city-forum-tag">{{ tag }}</span>
                </div>
                <p class="city-forum-body">{{ post.body || post.excerpt }}</p>
                <div v-if="communityRuntimeMeta(post).length" class="runtime-meta">
                  <span v-for="item in communityRuntimeMeta(post)" :key="item">{{ item }}</span>
                </div>
                <div v-if="communitySignalBadges(post).length || communityRuntimeBadges(post).length" class="mt-3 flex flex-wrap gap-2">
                  <span v-for="badge in communitySignalBadges(post)" :key="badge" class="signal-badge">{{ badge }}</span>
                  <span v-for="badge in communityRuntimeBadges(post)" :key="badge.label" class="runtime-badge" :class="`runtime-badge-${badge.tone}`">{{ badge.label }}</span>
                </div>
                <div v-if="postActionButtons(post).length" class="runtime-actions mt-3">
                  <button v-for="action in postActionButtons(post)" :key="action.key" class="runtime-action-btn" type="button" :disabled="operatingPostId === post.id" @click="action.handler()">
                    {{ operatingPostId === post.id ? '处理中...' : action.label }}
                  </button>
                </div>
                <section class="city-forum-replies" aria-label="帖子回复">
                  <h2>回复 <span>{{ post.replies }}</span></h2>
                  <p v-if="post.id && forumCommentsStatus[post.id] === 'loading'" role="status">正在加载回复…</p>
                  <div v-else-if="post.id && forumCommentsStatus[post.id] === 'error'" role="alert">
                    回复暂时没有加载成功。
                    <button class="city-forum-back" type="button" @click="loadForumComments(post)">重试</button>
                  </div>
                  <p v-else-if="!post.comments?.length" class="city-forum-meta">还没有回复。</p>
                  <article v-for="comment in post.comments" :key="comment.id" class="city-forum-reply">
                    <div class="city-forum-meta">
                      <strong>{{ comment.author }}</strong>
                      <time v-if="comment.created_at" :datetime="comment.created_at">{{ formatCommunityTime(comment.created_at) }}</time>
                      <span v-for="badge in commentRuntimeBadges(comment)" :key="badge.label" class="comment-badge" :class="`comment-badge-${badge.tone}`">{{ badge.label }}</span>
                    </div>
                    <p>{{ comment.body }}</p>
                  </article>
                  <p v-if="post.comments?.length && post.replies > post.comments.length" class="city-forum-meta">已加载 {{ post.comments.length }} 条回复，共 {{ post.replies }} 条。</p>
                  <form v-if="post.id" class="city-forum-reply-form" @submit.prevent="submitComment(post)">
                    <label class="sr-only" :for="`city-forum-reply-${post.id}`">写回复</label>
                    <textarea :id="`city-forum-reply-${post.id}`" v-model="commentDrafts[post.id]" :placeholder="composerAllowed ? '写下你的回复…' : '当前频道回复需要参与资格'" :disabled="submittingCommentId === post.id || !composerAllowed" />
                    <button class="city-forum-primary" type="submit" :disabled="!composerAllowed || submittingCommentId === post.id || forumCommentsStatus[post.id] === 'loading' || !commentDrafts[post.id]?.trim()">
                      {{ submittingCommentId === post.id ? '发送中…' : '发送回复' }}
                    </button>
                  </form>
                </section>
              </template>
            </ZeroCityForumBoard>
          </section>

          <ZeroCityColumns v-if="isCityHome && route.query.view === 'columns'" />
          <section v-if="isCityHome && route.query.view !== 'columns'" class="zero-city-forum-home">
            <header class="zero-city-forum-header">
              <div class="zero-city-forum-title">
                <div class="forum-overline"><span class="forum-live-dot" aria-hidden="true"></span><span>零号城 / 全城动态</span><span class="forum-open-label">城门已开</span></div>
                <h1>欢迎回到零号城</h1>
                <p>带着问题来，带着灵感走。也许下一段故事，就从你的回应开始。</p>
              </div>
              <div class="zero-city-forum-actions">
                <button class="community-btn community-btn-secondary" type="button" @click="router.push('/operator')">继续创作</button>
                <button class="community-btn community-btn-primary" type="button" @click="jumpToComposer">说点什么</button>
              </div>
            </header>

            <PlayerAnnouncements />

            <div class="zero-city-forum-search">
              <label class="sr-only" for="zero-city-forum-search-input">搜索零号城动态</label>
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="m21 21-4.35-4.35m1.35-5.65a7 7 0 1 1-14 0 7 7 0 0 1 14 0Z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              </svg>
              <input id="zero-city-forum-search-input" v-model.trim="homeSearch" type="search" placeholder="搜索问题、经验、模型、API 或协作线索" />
              <span v-if="homeSearch">{{ homeFeedPosts.length }} 条结果</span>
              <button v-if="homeSearch" class="forum-search-clear" type="button" aria-label="清除搜索" @click="homeSearch = ''">×</button>
            </div>

            <div class="zero-city-forum-layout">
              <main id="community-sections" class="zero-city-forum-feed">
                <TabsRoot :model-value="homeFeedMode" @update:model-value="setHomeFeedMode">
                  <section class="forum-feed-panel" aria-labelledby="forum-feed-title">
                  <header class="forum-feed-header">
                    <div>
                      <p class="community-kicker">城市内容流</p>
                      <div class="forum-feed-heading-row"><h2 id="forum-feed-title">正在发生</h2><span class="forum-feed-count">{{ homeFeedPosts.length }}</span></div>
                      <p class="forum-feed-subtitle">{{ activeFeedModeMeta.note }}</p>
                    </div>
                    <button class="forum-rules-toggle" type="button" :aria-expanded="showFeedRules" @click="showFeedRules = !showFeedRules">
                      {{ showFeedRules ? '收起说明' : '为什么会看到这些' }}
                    </button>
                  </header>

                  <TabsList class="forum-mode-tabs" aria-label="动态排序模式">
                    <TabsTrigger
                      v-for="mode in homeFeedModes"
                      :key="mode.key"
                      :value="mode.key"
                      :class="{ active: homeFeedMode === mode.key }"
                    >
                      {{ mode.label }}
                    </TabsTrigger>
                  </TabsList>

                  <TabsContent v-for="mode in homeFeedModes" :key="`feed-${mode.key}`" :value="mode.key" class="forum-feed-content">
                    <template v-if="mode.key === homeFeedMode">
                  <div v-if="showFeedRules" class="forum-rules-note">
                    <strong>这里不只显示热度最高的内容。</strong>
                    <span>和你有关、正在讨论，以及还在等回应的话题，都会出现在这里。</span>
                  </div>

                  <div v-if="isLoadingPosts" class="zero-city-feed-state">正在读取全城动态...</div>
                  <div v-else-if="postLoadError" class="zero-city-feed-state">
                    <strong>全城动态暂时没有加载成功</strong>
                    <p>请稍后刷新重试，已发布的内容不会丢失。</p>
                    <button class="community-btn community-btn-primary" type="button" @click="loadCommunityPosts">重新读取</button>
                  </div>
                  <div v-else-if="homeFeedPosts.length === 0" class="zero-city-feed-state">
                    <strong>{{ homeSearch ? '没有找到匹配内容' : homeFeedMode === 'unanswered' ? '当前没有等待第一条回复的问题' : homeFeedMode === 'following' ? '你还没有关注中的动态' : '零号城正在等待第一条真实动态' }}</strong>
                    <p>{{ homeSearch ? '换一个关键词，或者到答疑互助发布这个问题。' : homeFeedMode === 'unanswered' ? '去最新动态聊聊，或者留下你正在研究的问题。' : homeFeedMode === 'following' ? '关注作者或城区后，这里会成为你的个人城市入口。' : '发布一个问题、经验或协作想法，开始第一段城市讨论。' }}</p>
                    <button class="community-btn community-btn-primary" type="button" @click="homeSearch ? activateDistrict('workshop', 'help-desk') : jumpToComposer()">
                      {{ homeSearch ? '去提问' : '发布第一条动态' }}
                    </button>
                  </div>
                  <div v-else class="forum-thread-list">
                    <article v-for="post in homeFeedPosts" :key="`home-${post.id || post.title}-${post.time}`" class="forum-thread-row" :class="{ 'is-expanded': isFeedPostExpanded(post) }">
                      <div class="forum-thread-mark" aria-hidden="true"><span>{{ postKindLabel(post) }}</span><i :class="`thread-dot thread-dot-${postStatusTone(post)}`"></i></div>
                      <div class="forum-thread-main">
                        <div class="forum-thread-topline">
                          <div class="forum-thread-context"><span class="forum-thread-section">{{ post.section }}</span><span v-if="post.channel">#{{ post.channel }}</span><span v-if="postStatusLabel(post)" class="forum-thread-status">{{ postStatusLabel(post) }}</span></div>
                          <time>{{ post.time }}</time>
                        </div>
                        <button class="forum-thread-title" type="button" @click="toggleFeedPost(post)"><span>{{ post.title }}</span><span class="forum-thread-open-hint">{{ isFeedPostExpanded(post) ? '收起' : '查看详情' }}</span></button>
                        <p>{{ post.excerpt }}</p>
                        <div v-if="communitySignalBadges(post).length || communityRuntimeBadges(post).length" class="forum-thread-signals">
                          <span v-for="badge in communitySignalBadges(post)" :key="badge">{{ badge }}</span>
                          <span v-for="badge in communityRuntimeBadges(post)" :key="badge.label" :class="`signal-${badge.tone}`">{{ badge.label }}</span>
                        </div>
                        <div v-if="isFeedPostExpanded(post)" class="forum-thread-detail">
                          <p>{{ post.body || post.excerpt }}</p>
                          <div v-if="post.comments?.length" class="forum-thread-comments">
                            <div v-for="comment in post.comments.slice(0, 3)" :key="comment.id"><strong>{{ comment.author }}</strong><span>{{ comment.body }}</span></div>
                          </div>
                          <form v-if="post.id" class="forum-thread-reply" @submit.prevent="submitComment(post)">
                            <label class="sr-only" :for="`forum-reply-${post.id}`">回复这条动态</label>
                            <input :id="`forum-reply-${post.id}`" v-model="commentDrafts[post.id]" placeholder="写一句回复，接住这条线索" />
                            <button type="submit" :disabled="submittingCommentId === post.id">{{ submittingCommentId === post.id ? '发送中' : '回复' }}</button>
                          </form>
                        </div>
                        <div class="forum-thread-byline"><span class="forum-author-mark">{{ post.card || '居民' }}</span><span>{{ actionLabel(post.action_type) || '参与讨论' }}</span><button v-if="post.user_id" type="button" @click="openZeroCityProfile(post.user_id)">查看居民主页</button></div>
                        <div v-if="isFeedPostExpanded(post)" class="forum-thread-feedback" aria-label="推荐反馈">
                          <span>不想再看到什么？</span>
                          <button type="button" @click="setFeedFeedback(post, '不感兴趣')">不感兴趣</button>
                          <button type="button" @click="setFeedFeedback(post, '少看此城区')">少看此城区</button>
                          <button type="button" @click="setFeedFeedback(post, '减少此类内容')">减少此类内容</button>
                          <button type="button" @click="setFeedFeedback(post, '不推荐作者')">不推荐作者</button>
                          <button type="button" @click="setFeedFeedback(post, '举报或隐藏')">举报或隐藏</button>
                        </div>
                        <div v-if="post.id && feedFeedback[post.id]" class="forum-thread-feedback-result">{{ feedFeedback[post.id] }} · 收到了，之后会少打扰你。</div>
                      </div>
                      <div class="forum-thread-stats" aria-label="动态统计"><div><strong>{{ post.replies }}</strong><span>回复</span></div><div><strong>{{ post.catches }}</strong><span>收藏</span></div><div><strong>{{ post.views }}</strong><span>浏览</span></div></div>
                      <div class="forum-thread-reason"><span>为什么在这里</span><strong>{{ recommendationReason(post) }}</strong></div>
                    </article>
                  </div>
                    </template>
                  </TabsContent>
                  </section>
                </TabsRoot>
              </main>

              <aside class="zero-city-forum-aside">
                <section class="forum-side-panel">
                  <div class="forum-side-heading"><div><p class="community-kicker">城里不只有帖子</p><h2>找到一起做事的人</h2></div></div>
                  <p>在聊天室打个招呼，读一位创作者的专栏，或把你的第一个问题留下来。</p>
                  <div class="forum-side-actions">
                    <button type="button" @click="router.replace({ query: { ...route.query, chat: String(Date.now()) } })">打开聊天室</button>
                    <button type="button" @click="router.push('/community?view=columns')">逛逛专栏</button>
                  </div>
                </section>
                <ZeroCityCityPulse
                  compact
                  :definitions="visibleRankingDefinitions"
                  :rows-by-key="rankingRowsByKey"
                  @open-event="router.push('/tavern')"
                  @open-plaza="activateDistrict('tavern', 'chat-hall')"
                  @select="handleRankingSelect"
                  @open="handleRankingOpen"
                />
                <section class="forum-side-panel">
                  <div class="forum-side-heading"><div><p class="community-kicker">精选讨论</p><h2>值得留下的内容</h2></div><button type="button" @click="activateDistrict('workshop', 'help-desk')">查看全部</button></div>
                  <div v-if="standardAnswerPosts.length" class="forum-side-list">
                    <button v-for="post in standardAnswerPosts" :key="`answer-${post.id || post.title}`" type="button" @click="activateDistrict('workshop', 'help-desk')"><strong>{{ post.title }}</strong><small>{{ featuredReason(post) }}</small></button>
                  </div>
                  <p v-else class="forum-side-empty">好问题和好回答会出现在这里。</p>
                </section>

                <section class="forum-side-panel forum-gate-panel">
                  <div class="forum-side-heading"><div><p class="community-kicker">协作入口</p><h2>协作交流</h2></div><span class="forum-gate-lock">浏览</span></div>
                  <p>在这里展示作品、发布需求、记录交付。</p>
                  <button type="button" @click="activateDistrict('market')">查看集市 <span aria-hidden="true">→</span></button>
                </section>

                <section class="forum-side-panel forum-card-panel">
                  <div class="forum-side-heading"><div><p class="community-kicker">个人装扮</p><h2>让别人记住你</h2></div><span class="forum-card-mark">卡册</span></div>
                  <p>头像、背景、名牌和一句话，都可以放到这里。</p>
                  <div class="forum-side-actions"><button type="button" @click="router.push('/zero-city/cards')">打开卡册</button><button type="button" @click="activatePersonalHub">我的零号城</button></div>
                </section>
              </aside>
            </div>
          </section>

      <section v-if="legacyHomeSectionsVisible" class="community-command zero-city-command overflow-hidden rounded-[2rem] border p-5 lg:p-6">
        <div class="relative z-[1] grid gap-5 xl:grid-cols-[1.08fr_0.92fr]">
          <div class="command-primary zero-city-console space-y-4">
            <div class="flex flex-wrap items-center gap-2">
              <p class="command-kicker"><span class="command-live-dot mr-2"></span>Zero City Console</p>
              <span v-if="hasSourceFilter" class="command-source-chip">{{ activeSubjectChip }}</span>
            </div>
            <h1 class="command-title">在 AI 蛮荒里，找到可以继续前进的线索</h1>
            <p class="command-description">
              在零号城提问、分享经验、交换模型情报，也可以参与 BizDecipher 的产品共建。可靠的回答和协作进展会持续沉淀下来。
            </p>
            <div class="flex flex-wrap gap-2">
              <button class="community-btn community-btn-primary" type="button" @click="activateChannel('qa')">去提问</button>
              <button class="community-btn community-btn-secondary" type="button" @click="jumpToComposer">发布动态</button>
              <button class="community-btn community-btn-ghost" type="button" @click="router.push('/zero-city/cards')">我的卡册</button>
              <button class="community-btn community-btn-ghost" type="button" @click="activatePersonalHub">我的零号城</button>
            </div>
            <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <article v-for="metric in communityCommandMetrics" :key="metric.title" class="command-metric">
                <span>{{ metric.label }}</span>
                <strong>{{ metric.value }}</strong>
                <small>{{ metric.title }}</small>
              </article>
            </div>
          </div>

          <aside class="command-operator city-operator-card">
            <div class="city-operator-main">
              <div class="city-operator-copy">
                <span class="operator-status">今日值班</span>
                <h2>{{ cityDuty.title }}</h2>
                <p>{{ cityDuty.line }}</p>
              </div>
              <div class="city-operator-portrait">
                <img :src="cityDuty.src" :alt="cityDuty.name" />
              </div>
            </div>
            <div class="city-duty-panel">
              <strong>{{ cityDuty.name }}</strong>
              <span>{{ cityDuty.prompt }}</span>
            </div>
            <div class="mt-4 flex flex-wrap gap-2">
              <button v-for="suggestion in cityDuty.suggestions" :key="suggestion" class="city-suggestion-chip" type="button" @click="prefillComposer(suggestion)">
                {{ suggestion }}
              </button>
            </div>
          </aside>
        </div>

        <div class="zero-city-map zero-city-channel-map relative z-[1] mt-5">
          <button
            v-for="channel in primaryChannels"
            :key="channel.key"
            class="city-district-card primary-channel-card"
            :class="channel.tone"
            type="button"
            @click="channel.handler()"
          >
            <div class="city-district-head">
              <span>{{ channel.icon }}</span>
              <div>
                <h2>{{ channel.title }}</h2>
                <p>{{ channel.description }}</p>
              </div>
            </div>
            <small class="city-visibility-chip">{{ channel.note }}</small>
          </button>
        </div>
      </section>

      <section v-if="legacyHomeSectionsVisible" class="zero-city-homeboard grid gap-4 xl:grid-cols-[1.08fr_0.92fr]">
        <article class="zero-city-home-panel zero-city-home-feed p-5 lg:p-6">
          <div class="zero-city-home-head">
            <div>
              <p class="community-kicker">City Feed</p>
              <h2>全城动态</h2>
              <p>公开帖子、回复、需求、交付、反馈和资源信号从这里进入，再流向对应城区。</p>
            </div>
            <button class="community-btn community-btn-ghost" type="button" @click="jumpToSections">查看动态流</button>
          </div>
          <div v-if="latestHomePosts.length" class="zero-city-home-list mt-4">
            <button v-for="post in latestHomePosts" :key="`latest-${post.id || post.title}`" type="button" @click="jumpToSections">
              <span>{{ post.section }}</span>
              <strong>{{ post.title }}</strong>
              <small>{{ post.replies }} 回复 · {{ post.views }} 浏览 · {{ post.time }}</small>
            </button>
          </div>
          <div v-else class="zero-city-home-empty mt-4">
            <strong>还没有真实全城动态</strong>
            <p>第一条问题、经验或协作线索发出后，会出现在首页。</p>
          </div>
        </article>

        <div class="zero-city-home-stack grid gap-4">
          <article id="zero-city-hot" class="zero-city-home-panel p-5">
            <div class="zero-city-home-head compact">
              <div>
                <p class="community-kicker">Hot</p>
                <h2>热门帖子</h2>
              </div>
              <span>{{ hotHomePosts.length ? `${hotHomePosts.length} 条` : '无假热度' }}</span>
            </div>
            <div v-if="hotHomePosts.length" class="zero-city-compact-list mt-4">
              <button v-for="post in hotHomePosts" :key="`hot-${post.id || post.title}`" type="button" @click="jumpToSections">
                <strong>{{ post.title }}</strong>
                <small>{{ post.catches }} 捕获 · {{ post.replies }} 回复 · {{ post.views }} 浏览</small>
              </button>
            </div>
            <p v-else class="zero-city-home-note mt-4">有真实回复、收藏、采纳、官方确认后才进入热门。</p>
          </article>

          <article id="zero-city-featured" class="zero-city-home-panel p-5">
            <div class="zero-city-home-head compact">
              <div>
                <p class="community-kicker">Archive</p>
                <h2>精选沉淀</h2>
              </div>
              <span>{{ featuredHomePosts.length ? `${featuredHomePosts.length} 条` : '待沉淀' }}</span>
            </div>
            <div v-if="featuredHomePosts.length" class="zero-city-compact-list mt-4">
              <button v-for="post in featuredHomePosts" :key="`featured-${post.id || post.title}`" type="button" @click="jumpToSections">
                <strong>{{ post.title }}</strong>
                <small>{{ featuredReason(post) }}</small>
              </button>
            </div>
            <p v-else class="zero-city-home-note mt-4">被采纳、官方确认、设为精华或进入资产候选后，才会出现在这里。</p>
          </article>
        </div>
      </section>

      <section v-if="legacyHomeSectionsVisible" class="zero-city-homeboard grid gap-4 xl:grid-cols-[0.92fr_1.08fr]">
        <article class="zero-city-home-panel p-5 lg:p-6">
          <div class="zero-city-home-head compact">
            <div>
              <p class="community-kicker">My Zero City</p>
              <h2>我的进度</h2>
              <p>先用真实帖子、采纳、官方确认和资产候选做进度预览，后续再接 XP / 积分 / 勋章账本。</p>
            </div>
            <button class="community-btn community-btn-ghost" type="button" @click="activatePersonalHub">我的零号城</button>
          </div>
          <div class="zero-city-progress-grid mt-4">
            <article v-for="item in zeroCityProgressCards" :key="item.label">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
              <small>{{ item.note }}</small>
            </article>
          </div>
        </article>

        <article class="zero-city-home-panel p-5 lg:p-6">
          <div class="zero-city-home-head compact">
            <div>
              <p class="community-kicker">Daily Quests</p>
              <h2>今日可做</h2>
              <p>轻行为给参与感，可信行为才进入积分、声誉和市场权限。</p>
            </div>
          </div>
          <div class="zero-city-task-grid mt-4">
            <button v-for="task in zeroCityDailyTasks" :key="task.title" type="button" @click="task.handler()">
              <span>{{ task.reward }}</span>
              <strong>{{ task.title }}</strong>
              <small>{{ task.note }}</small>
            </button>
          </div>
        </article>
      </section>

      <section v-if="false && isCityHome" class="community-health-grid grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <article v-for="signal in communityHealthSignals" :key="signal.title" class="decision-signal" :class="signal.tone">
          <div>
            <strong>{{ signal.title }}</strong>
            <p>{{ signal.note }}</p>
          </div>
          <span>{{ signal.value }}</span>
        </article>
      </section>

      <section v-if="legacyHomeSectionsVisible" id="community-sections-legacy" class="grid gap-4 xl:grid-cols-[0.95fr_1.05fr]">
        <div class="panel p-5 lg:p-6">
          <div class="flex items-end justify-between gap-4">
            <div>
              <p class="community-kicker">零号城频道</p>
              <h2 class="community-section-title mt-2 text-xl font-black tracking-[0]">选择你要参与的内容</h2>
            </div>
            <span class="community-chip rounded-full px-3 py-1 text-xs font-black">5 个频道</span>
          </div>
          <div class="mt-5 grid gap-3 sm:grid-cols-2">
            <button v-for="channel in primaryChannels" :key="channel.key" class="district-card rounded-[1.25rem] p-4 text-left" type="button" @click="channel.handler()">
              <div class="flex items-start justify-between gap-3">
                <span class="district-mark">{{ channel.icon }}</span>
                <span class="community-chip rounded-full px-2.5 py-1 text-xs font-black">{{ channel.note }}</span>
              </div>
              <h3 class="community-card-title mt-4 text-sm font-black">{{ channel.title }}</h3>
              <p class="community-card-copy mt-2 text-xs leading-5">{{ channel.description }}</p>
            </button>
          </div>
        </div>

        <div class="panel overflow-hidden">
          <div class="community-section-head border-b px-5 py-4 lg:px-6">
            <h2 class="community-section-small-title text-base font-black">动态与协作进展</h2>
            <p class="community-card-copy mt-1 text-xs leading-5">查看居民发布的问题、经验、模型情报和产品建议。</p>
          </div>
          <div class="community-section-head post-filter-bar border-b px-5 py-3 lg:px-6">
            <button
              v-for="filter in postFilters"
              :key="filter.key"
              class="post-filter"
              :class="{ 'post-filter-active': activePostFilter === filter.key }"
              type="button"
              @click="activePostFilter = filter.key"
            >
              {{ filter.label }}
            </button>
          </div>
          <div class="community-divide divide-y">
            <div v-if="isLoadingPosts" class="community-muted-line p-5 text-sm font-bold">正在读取真实社区帖子...</div>
            <div v-else-if="postLoadError" class="community-empty-state p-6 lg:p-8">
              <span class="empty-state-kicker">Read failed</span>
              <h3>社区内容读取失败</h3>
              <p>动态暂时没有加载成功，请稍后刷新再试。</p>
              <div class="mt-4 flex flex-wrap gap-2">
                <button class="community-btn community-btn-primary" type="button" @click="loadCommunityPosts">重新读取</button>
                <button class="community-btn community-btn-secondary" type="button" @click="jumpToFeedback">提交产品反馈</button>
              </div>
            </div>
            <div v-else-if="filteredPosts.length === 0" class="community-empty-state p-6 lg:p-8">
              <span class="empty-state-kicker">等待第一条分享</span>
              <h3>零号城还没有动态</h3>
              <p>发布第一个问题、经验、模型情报或产品建议，其他居民就能在这里继续讨论。</p>
              <div class="mt-4 flex flex-wrap gap-2">
                <button class="community-btn community-btn-primary" type="button" @click="jumpToComposer">发布第一条真实记录</button>
                <button class="community-btn community-btn-secondary" type="button" @click="jumpToFeedback">提交产品反馈</button>
              </div>
            </div>
            <template v-else>
            <article v-for="post in filteredPosts" :key="`${post.id || post.title}-${post.time}`" class="feed-row p-5 lg:px-6">
              <div class="flex flex-wrap items-center gap-2">
                <span class="community-chip community-chip-accent rounded-full px-2.5 py-1 text-xs font-black">{{ post.section }}</span>
                <span class="community-chip rounded-full px-2.5 py-1 text-xs font-black">{{ post.card }}</span>
                <span class="community-time text-xs font-bold">{{ post.time }}</span>
              </div>
              <h3 class="community-section-small-title mt-3 text-base font-black">{{ post.title }}</h3>
              <div v-if="communitySignalBadges(post).length || communityRuntimeBadges(post).length" class="mt-3 flex flex-wrap gap-2">
                <span v-for="badge in communitySignalBadges(post)" :key="badge" class="signal-badge">{{ badge }}</span>
                <span
                  v-for="badge in communityRuntimeBadges(post)"
                  :key="badge.label"
                  class="runtime-badge"
                  :class="`runtime-badge-${badge.tone}`"
                >
                  {{ badge.label }}
                </span>
              </div>
              <p class="community-card-copy mt-2 whitespace-pre-wrap text-sm leading-6">{{ post.excerpt }}</p>
              <div v-if="communityRuntimeMeta(post).length" class="runtime-meta mt-3">
                <span v-for="item in communityRuntimeMeta(post)" :key="item">{{ item }}</span>
              </div>
              <div class="community-meta mt-4 flex flex-wrap items-center gap-3 text-xs font-black">
                <span>{{ post.catches }} {{ t('community.feed.catches') }}</span>
                <span>{{ post.replies }} {{ t('community.feed.replies') }}</span>
                <span>{{ post.views }} {{ t('community.feed.views') }}</span>
                <button v-if="post.user_id" class="community-link" type="button" @click="openZeroCityProfile(post.user_id)">零号城主页</button>
                <button v-if="post.source_type === 'shared_pool' && post.source_id" class="community-link community-link-secondary" type="button" @click="openSharedPoolDiscussion(post.source_id)">查看主体讨论</button>
              </div>
              <div v-if="postActionButtons(post).length" class="runtime-actions mt-3">
                <button
                  v-for="action in postActionButtons(post)"
                  :key="action.key"
                  class="runtime-action-btn"
                  type="button"
                  :disabled="operatingPostId === post.id"
                  @click="action.handler()"
                >
                  {{ operatingPostId === post.id ? '处理中...' : action.label }}
                </button>
              </div>
              <div v-if="post.comments?.length" class="community-comments-block mt-4 space-y-2 rounded-2xl p-3">
                <div v-for="comment in post.comments" :key="comment.id" class="community-comment-item rounded-xl p-3">
                  <div class="flex items-center justify-between gap-3">
                    <b class="text-xs">{{ comment.author }}</b>
                    <div class="flex flex-wrap justify-end gap-1.5">
                      <span v-for="badge in commentRuntimeBadges(comment)" :key="badge.label" class="comment-badge" :class="`comment-badge-${badge.tone}`">{{ badge.label }}</span>
                    </div>
                  </div>
                  <p class="community-card-copy mt-1 whitespace-pre-wrap text-xs leading-5">{{ comment.body }}</p>
                  <div v-if="commentActionButtons(post, comment).length" class="runtime-actions mt-2">
                    <button
                      v-for="action in commentActionButtons(post, comment)"
                      :key="action.key"
                      class="runtime-action-btn runtime-action-btn-small"
                      type="button"
                      :disabled="operatingCommentId === comment.id"
                      @click="action.handler()"
                    >
                      {{ operatingCommentId === comment.id ? '处理中...' : action.label }}
                    </button>
                  </div>
                </div>
              </div>
              <form v-if="post.id" class="mt-3 flex gap-2" @submit.prevent="submitComment(post)">
                <input v-model="commentDrafts[post.id]" class="community-input !rounded-full !py-2 text-xs" placeholder="回复/接一下这个问题..." />
                <button class="community-btn community-btn-primary !min-h-0 !px-4 !py-2 text-xs" type="submit" :disabled="submittingCommentId === post.id">
                  {{ submittingCommentId === post.id ? '...' : '回复' }}
                </button>
              </form>
            </article>
            </template>
          </div>
        </div>
      </section>

      <section v-if="false && isCityHome" class="community-connection-grid grid gap-4 xl:grid-cols-[0.92fr_1.08fr]">
        <article class="decision-panel zero-city-module-panel p-5">
          <div class="section-head compact-head">
            <div>
              <p class="command-kicker">City workflows</p>
              <h2>零号城协作路径</h2>
            </div>
            <span>{{ hasSourceFilter ? activeSubjectChip : '开放协作层' }}</span>
          </div>
          <div class="mt-4 grid gap-3 sm:grid-cols-2">
            <button v-for="module in zeroCityModules" :key="module.title" class="zero-city-module-card" type="button" @click="module.handler()">
              <span>{{ module.kicker }}</span>
              <strong>{{ module.title }}</strong>
              <small>{{ module.note }}</small>
            </button>
          </div>
        </article>

        <article id="zero-city-tavern" class="decision-panel tavern-panel p-5">
          <div class="tavern-panel-head">
            <div>
              <p class="command-kicker">Night district</p>
              <h2>夜间酒馆收进零号城</h2>
              <p>酒馆不是全站平级主产品，而是零号城里的故事区。剧本、房间、复盘和坏剧本信号只有能沉淀关系、资产、积分消费和创作者信用，才继续保留。</p>
            </div>
            <button class="tavern-open-badge" type="button" @click="jumpToTavern">进入夜间酒馆</button>
          </div>
          <div class="mt-4 grid gap-3 md:grid-cols-3">
            <article v-for="tier in tavernScriptTiers" :key="tier.title" class="tavern-tier-card">
              <span>{{ tier.badge }}</span>
              <strong>{{ tier.title }}</strong>
              <p>{{ tier.note }}</p>
            </article>
          </div>
        </article>
      </section>

      <section v-if="false && isCityHome" id="community-token-intel" class="community-intel-panel rounded-[2rem] border p-5 lg:p-6">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <p class="command-kicker">{{ t('community.tokenIntel.kicker') }}</p>
            <h2 class="mt-3 text-2xl font-black tracking-[0]">{{ t('community.tokenIntel.title') }}</h2>
            <p class="mt-2 max-w-2xl text-sm font-bold leading-7">{{ t('community.tokenIntel.description') }}</p>
          </div>
          <button class="community-btn community-btn-ghost" type="button" @click="activateDistrict('workshop', 'news-radar')">
            发资源情报
          </button>
        </div>

        <div class="mt-5 flex flex-wrap gap-2">
          <button
            v-for="board in tokenBoards"
            :key="board.key"
            class="token-tab"
            :class="{ 'token-tab-active': activeTokenBoard === board.key }"
            type="button"
            @click="activeTokenBoard = board.key"
          >
            {{ board.title }}
          </button>
        </div>

        <div class="mt-5 grid gap-4 lg:grid-cols-[0.82fr_1.18fr]">
          <article v-for="board in visibleTokenBoards" :key="board.title" class="token-board rounded-[1.45rem] border p-4">
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="text-xs font-black">{{ board.mascot }}</p>
                <h3 class="mt-2 text-base font-black">{{ board.title }}</h3>
              </div>
              <span class="rounded-full px-2.5 py-1 text-xs font-black">{{ board.badge }}</span>
            </div>
            <div class="mt-4 space-y-3">
              <div v-for="row in board.items" :key="row.name" class="intel-row rounded-2xl p-3">
                <div class="flex items-center justify-between gap-3">
                  <b class="text-sm">{{ row.name }}</b>
                  <span class="text-xs font-black">{{ row.value }}</span>
                </div>
                <p class="mt-1 text-xs leading-5">{{ row.note }}</p>
              </div>
            </div>
          </article>

          <aside class="intel-rules rounded-[1.45rem] border p-4">
            <p class="text-xs font-black uppercase tracking-[0.2em]">Token Power</p>
            <h3 class="mt-2 text-lg font-black">周榜第一免单 · 封顶 $200</h3>
            <p class="mt-2 text-sm font-bold leading-7">
              只统计真实付费有效消耗，免费额度、退款、异常刷量不计入。{{ tokenPowerPeriodLabel }}
            </p>
            <div v-if="tokenPowerRows().length" class="mt-4 space-y-2">
              <div v-for="row in tokenPowerRows()" :key="row.user_id" class="intel-row rounded-2xl border p-3">
                <div class="flex items-center justify-between gap-3">
                  <b class="text-sm">#{{ row.rank }} {{ row.display_name }}</b>
                  <span class="text-xs font-black">{{ formatMoney(row.effective_spend) }}</span>
                </div>
                <p class="mt-1 text-xs leading-5">
                  {{ row.request_count }} 次有效调用 · {{ row.token_count }} tokens · 预计返 {{ formatMoney(row.reward_amount) }}
                </p>
              </div>
            </div>
            <div class="mt-4 grid gap-3 sm:grid-cols-3">
              <div v-for="rule in tokenRules" :key="rule.title" class="intel-row rounded-2xl border p-3">
                <p class="text-sm font-black">{{ rule.title }}</p>
                <p class="mt-1 text-xs leading-5">{{ rule.description }}</p>
              </div>
            </div>
          </aside>
        </div>
      </section>

      <section v-if="isCityHome" id="community-archive" class="panel archive-entry p-5 lg:p-6">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div class="min-w-0">
            <p class="community-kicker">Zero City Archive</p>
            <h2 class="community-section-title mt-2 text-xl font-black tracking-[0]">零号城档案馆</h2>
            <p class="community-card-copy mt-2 max-w-3xl text-sm leading-7">从第一盏灯到第一张休假表。读一段城史，去看看那些收藏卡里的角色住在哪里。</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="item in archiveEntryLinks"
              :key="item.key"
              class="archive-entry-link"
              :class="{ 'archive-entry-link-active': activeArchiveEntry === item.key }"
              type="button"
              @click="selectArchiveEntry(item.key)"
            >
              {{ item.label }}
            </button>
          </div>
        </div>
        <div class="archive-preview mt-4">
          <div>
            <span>{{ activeArchiveEntryMeta.label }}</span>
            <h3>{{ activeArchiveEntryMeta.title }}</h3>
            <p>{{ activeArchiveEntryMeta.description }}</p>
          </div>
          <button class="community-btn community-btn-ghost" type="button" @click="openArchiveEntry(activeArchiveEntryMeta.key)">
            {{ activeArchiveEntryMeta.action }}
          </button>
        </div>
      </section>

      <section v-if="false && isCityHome" class="grid gap-4 lg:grid-cols-[0.92fr_1.08fr]">
        <div class="panel p-5 lg:p-6">
          <div class="community-callout rounded-[1.5rem] p-5">
            <p class="community-card-title text-sm font-black">发布到零号城</p>
          </div>
        </div>

        <div class="panel p-5 lg:p-6">
          <p class="community-kicker">Visible Signals</p>
          <h2 class="community-section-title mt-2 text-xl font-black tracking-[0]">核心状态信号</h2>
          <div class="mt-4 grid gap-3 md:grid-cols-2">
            <article v-for="rule in runtimeRuleCards" :key="rule.title" class="runtime-rule-card" :class="`runtime-rule-card-${rule.tone}`">
              <div>
                <p>{{ rule.title }}</p>
                <strong>{{ rule.value }}</strong>
              </div>
              <span>{{ rule.note }}</span>
            </article>
          </div>
          <div class="community-section-head mt-5 border-t pt-5">
            <div class="flex items-center justify-between gap-3">
              <p class="community-card-title text-sm font-black">志愿角色</p>
              <span class="community-chip community-chip-accent rounded-full px-2.5 py-1 text-xs font-black">协作入口</span>
            </div>
            <div class="mt-3 grid gap-3 md:grid-cols-2">
              <article v-for="role in helperRoles" :key="role.title" class="helper-card rounded-[1.25rem] p-4">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <p class="community-card-title text-sm font-black">{{ role.title }}</p>
                    <p class="community-card-copy mt-1 text-xs leading-5">{{ role.description }}</p>
                  </div>
                  <span class="community-chip community-chip-accent rounded-full px-2.5 py-1 text-xs font-black">{{ role.badge }}</span>
                </div>
                <button class="community-link mt-3 text-xs font-black" type="button" @click="handleVolunteer(role.title)">
                  {{ t('community.support.volunteerButton') }}
                </button>
              </article>
            </div>
          </div>
        </div>
      </section>

      <section v-if="false && isCityHome" id="community-feedback" class="grid gap-4 xl:grid-cols-[1fr_1fr]">
        <div class="panel p-5 lg:p-6">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p class="community-kicker">Feedback</p>
              <h2 class="community-section-title mt-2 text-xl font-black tracking-[0]">反馈通道</h2>
              <p class="community-card-copy mt-2 text-sm leading-7">这里是页面底部兜底：产品问题、支付异常、API 故障、体验反馈和治理建议都可以提交，但不抢社群首屏。</p>
            </div>
            <span class="community-chip community-chip-accent rounded-full px-3 py-1 text-xs font-black">底部兜底</span>
          </div>

          <div class="mt-5 space-y-4">
            <div class="flex flex-wrap gap-2">
              <button
                v-for="category in feedbackCategories"
                :key="category.key"
                class="quick-tag"
                :class="{ 'quick-tag-active': selectedFeedbackCategory === category.key }"
                type="button"
                @click="selectedFeedbackCategory = category.key"
              >
                {{ category.label }}
              </button>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="severity in feedbackSeverities"
                :key="severity.key"
                class="severity-chip"
                :class="{ 'severity-chip-active': selectedFeedbackSeverity === severity.key }"
                type="button"
                @click="selectedFeedbackSeverity = severity.key"
              >
                {{ severity.label }}
              </button>
            </div>
            <input v-model="feedbackTitle" class="community-input" :placeholder="t('community.feedback.titlePlaceholder')" />
            <textarea v-model="feedbackBody" class="community-input min-h-[132px] resize-none" :placeholder="t('community.feedback.bodyPlaceholder')"></textarea>
            <label class="community-checkbox-row flex items-start gap-3 rounded-2xl p-3 text-xs font-bold leading-5">
              <input v-model="feedbackPrivate" class="mt-1" type="checkbox" />
              <span>{{ t('community.feedback.privateLabel') }}</span>
            </label>
            <button class="community-btn community-btn-primary w-full" type="button" @click="handleFeedbackSubmit">
              {{ t('community.feedback.submit') }}
            </button>
            <p class="community-time text-center text-xs leading-5">{{ t('community.feedback.hint') }}</p>
          </div>
        </div>

        <div class="panel overflow-hidden">
          <div class="community-section-head border-b px-5 py-4 lg:px-6">
            <h2 class="community-section-small-title text-base font-black">{{ t('community.feedback.latestTitle') }}</h2>
            <p class="community-card-copy mt-1 text-xs leading-5">{{ t('community.feedback.latestDescription') }}</p>
          </div>
          <div v-if="feedbackPosts.length" class="community-divide divide-y">
            <article v-for="item in feedbackPosts" :key="`${item.postId || item.title}-${item.time}`" class="p-5 lg:px-6">
              <div class="flex flex-wrap items-center gap-2">
                <span class="community-chip community-chip-accent rounded-full px-2.5 py-1 text-xs font-black">{{ item.category }}</span>
                <span class="community-chip rounded-full px-2.5 py-1 text-xs font-black">{{ item.severity }}</span>
                <span v-if="item.private" class="community-chip community-chip-strong rounded-full px-2.5 py-1 text-xs font-black">{{ t('community.feedback.privateBadge') }}</span>
                <span class="feedback-status-badge" :class="`feedback-status-${feedbackStatusTone(item)}`">{{ feedbackStatusLabel(item) }}</span>
                <span class="community-time text-xs font-bold">{{ item.time }}</span>
              </div>
              <h3 class="community-section-small-title mt-3 text-base font-black">{{ item.title }}</h3>
              <p class="community-card-copy mt-2 text-sm leading-6">{{ item.body }}</p>
              <div class="feedback-followup mt-3">
                <span>{{ feedbackVisibilityLabel(item) }}</span>
                <span>{{ feedbackEvidenceLabel(item) }}</span>
                <span>{{ feedbackNextStepLabel(item) }}</span>
              </div>
              <div class="mt-3 flex flex-wrap gap-2">
                <button class="runtime-action-btn runtime-action-btn-small" type="button" @click="openFeedbackHandling(item)">
                  {{ t('community.feedback.openHandling') }}
                </button>
                <button v-if="!item.private" class="runtime-action-btn runtime-action-btn-small" type="button" @click="prefillFeedbackReply(item)">
                  {{ t('community.feedback.replyAction') }}
                </button>
              </div>
            </article>
          </div>
          <div v-else class="community-muted-line p-6 text-sm leading-7">
            {{ t('community.feedback.empty') }}
          </div>
        </div>
      </section>
        </div>
      </div>

      <ZeroCityRankingDetailsDialog
        v-model:open="rankingDetailsOpen"
        :definition="rankingDetailsDefinition"
        :rows="rankingDetailsRows"
        data-scope="基于当前可见动态"
        @select="handleRankingDetailsSelect"
        @related="openRankingRelatedDistrict"
      />

      <DialogRoot :open="composerOpen" @update:open="setComposerOpen">
        <DialogPortal>
          <DialogOverlay class="zero-city-composer-overlay" />
          <DialogContent class="zero-city-composer-drawer" aria-describedby="zero-city-composer-description">
              <header class="zero-city-composer-head">
                <div>
                  <p class="community-kicker">发布到零号城</p>
                  <DialogTitle as="h2">{{ composerDestinationTitle }}</DialogTitle>
                  <DialogDescription as="p">{{ composerDestinationDescription }}</DialogDescription>
                </div>
                <DialogClose class="zero-city-composer-close" aria-label="关闭发布面板">×</DialogClose>
              </header>

              <div class="zero-city-composer-context">
                <span>{{ composerDestinationKicker }}</span>
                <strong>{{ composerDestinationTitle }}</strong>
                <small>{{ participationLabel }} · {{ composerAllowed ? '可参与当前频道' : '当前频道仅可阅读' }}</small>
              </div>

              <div class="zero-city-composer-form">
                <div v-if="participationError || !composerAllowed" class="participation-notice" role="status">
                  <p>{{ participationLoading ? '正在读取参与资格…' : participationError || '此频道需要相应参与资格，闲聊广场向所有已登录居民开放。' }}</p>
                  <button v-if="participationError" type="button" @click="refreshParticipation">重新加载</button>
                  <button v-else-if="!participationLoading" type="button" @click="activateDistrict('tavern', 'chat-hall')">去闲聊广场</button>
                </div>
                <input v-model="composerTitle" class="community-input" :placeholder="composerTitlePlaceholder" autofocus />
                <textarea v-model="composerBody" class="community-input min-h-[180px] resize-none" :placeholder="composerBodyPlaceholder"></textarea>
                <div class="flex flex-wrap gap-2">
                  <button
                    v-for="(tag, index) in quickTags"
                    :key="tag"
                    class="quick-tag"
                    :class="{ 'quick-tag-active': selectedTagIndex === index }"
                    type="button"
                    @click="selectComposerTag(index)"
                  >
                    {{ tag }}
                  </button>
                </div>
                <button class="community-btn community-btn-primary w-full" type="button" :disabled="isSubmittingPost || !composerAllowed" @click="handleComposerSubmit">
                  {{ isSubmittingPost ? '发布中...' : `发布到 ${composerDestinationTitle}` }}
                </button>
                <p class="community-time text-center text-xs leading-5">写清背景、证据、期望结果或可提供的帮助，方便其他居民继续协作。</p>
              </div>

              <section class="zero-city-composer-routes">
                <div>
                  <p class="community-card-title text-sm font-black">需要正式业务表单？</p>
                  <p class="community-card-copy mt-1 text-xs leading-5">能力资产、酒馆剧本和共享池创建分别进入对应业务模块，不在动态发布器里堆长表单。</p>
                </div>
                <div class="zero-city-composer-route-grid">
                  <button type="button" @click="openFormalCreate('/assets?tab=mine&action=create')">
                    <strong>提交能力资产</strong>
                    <small>作品、工作流、Agent 与可信证据</small>
                  </button>
                  <button type="button" @click="openFormalCreate('/tavern?action=create-script')">
                    <strong>投稿酒馆剧本</strong>
                    <small>剧本资产、试玩与房间体验</small>
                  </button>
                  <button type="button" @click="openFormalCreate('/account-square/owner?action=create')">
                    <strong>创建共享池</strong>
                    <small>账号供给、模型、席位与规则</small>
                  </button>
                </div>
              </section>
          </DialogContent>
        </DialogPortal>
      </DialogRoot>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  TabsContent,
  TabsList,
  TabsRoot,
  TabsTrigger,
} from 'reka-ui'
import AppLayout from '@/components/layout/AppLayout.vue'
import ZeroCityColumns from '@/features/bizdecipher/components/community/ZeroCityColumns.vue'
import ZeroCityCityPulse from '@/features/bizdecipher/components/community/ZeroCityCityPulse.vue'
import ZeroCityActivityBoard from '@/features/bizdecipher/components/community/ZeroCityActivityBoard.vue'
import ZeroCityPollHall from '@/features/bizdecipher/components/community/ZeroCityPollHall.vue'
import PlayerAnnouncements from '@/features/bizdecipher/components/community/PlayerAnnouncements.vue'
import { isOfficialCommunityPost, isResidentProposal } from '@/features/bizdecipher/utils/communityProvenance'
import ZeroCityGovernancePanel from '@/features/bizdecipher/components/community/ZeroCityGovernancePanel.vue'
import ZeroCityForumBoard from '@/features/bizdecipher/components/community/ZeroCityForumBoard.vue'
import ZeroCityPersonalHub from '@/features/bizdecipher/components/community/ZeroCityPersonalHub.vue'
import ZeroCityRankingDetailsDialog from '@/features/bizdecipher/components/community/ZeroCityRankingDetailsDialog.vue'
import { communityAPI, type TokenPowerLeaderboard } from '@/features/bizdecipher/api/community'
import { useCommunityParticipation } from '@/features/bizdecipher/components/community/useCommunityParticipation'
import { extractActionableApiErrorMessage } from '@/utils/apiError'
import { communityPreviewPosts } from '@/features/bizdecipher/data/communityPreview'
import { getZeroCitySurface } from '@/features/bizdecipher/data/zeroCityExperience'
import { type ZeroCityWorkspaceKey } from '@/features/bizdecipher/data/zeroCityModules'
import {
  buildZeroCityRankingRows,
  defaultZeroCityRankingConfig,
  loadZeroCityRankingConfig,
  normalizeRankingConfig,
  type ZeroCityRankingConfig,
  type ZeroCityRankingDefinition,
  type ZeroCityRankingPost,
  type ZeroCityRankingRow,
} from '@/features/bizdecipher/data/zeroCityRankings'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { isLocalPreviewAuth } from '@/utils/localPreview'
import { getTodayMascotDuty, zeroCityMascotSrc } from '@/features/bizdecipher/constants/zeroCityMascots'

const { t, tm } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

type TokenBoardKey = 'saving' | 'stable' | 'alert' | 'contribute'
type CommunityPostKind = 'card' | 'token' | 'support' | 'pool' | 'feedback' | 'announcement'
type CommunityScenario = 'resource_decision' | 'incident_support' | 'feedback_triage' | 'demand_match' | 'capability_showcase' | 'delivery_collaboration' | 'usage_intel' | 'announcement' | 'general'
type CommunityActionType = 'discuss' | 'ask_help' | 'report' | 'share_signal' | 'recommend' | 'offer' | 'request' | 'accept' | 'deliver' | 'review' | 'announce'
type PostFilterKey = 'all' | 'qa' | 'workflow' | 'demand' | 'asset' | 'resource' | 'product' | 'support' | 'feedback'
type ZeroCityDistrictKey = Exclude<ZeroCityWorkspaceKey, 'home' | 'mine'>
type HomeFeedMode = 'recommended' | 'latest' | 'unanswered' | 'hot' | 'technical' | 'following' | 'tavern' | 'official'
type ZeroCityChannel = {
  key: string
  label: string
  filter: PostFilterKey
  description: string
  kind?: CommunityPostKind
  scenario?: CommunityScenario
  actionType?: CommunityActionType
  handler: () => void
}
type ZeroCityDistrict = {
  key: ZeroCityDistrictKey
  icon: string
  title: string
  rule: string
  description: string
  tone: string
  visibility?: string
  handler: () => void
  channels: ZeroCityChannel[]
}
type FeedbackCategoryKey = 'api' | 'payment' | 'pool' | 'idea' | 'governance'
type FeedbackSeverityKey = 'normal' | 'urgent' | 'private'
type ArchiveEntryKey = 'world' | 'chronicle' | 'mascot' | 'rules'
type ArchiveEntryLink = { key: ArchiveEntryKey; label: string; title: string; description: string; action: string }
type RuntimeBadgeTone = 'good' | 'gold' | 'neutral' | 'risk'
type RuntimeBadge = { label: string; tone: RuntimeBadgeTone }
type RuntimeActionButton = { key: string; label: string; handler: () => void }
type LocaleMessage = string | number | boolean | null | LocaleMessage[] | { [key: string]: LocaleMessage }

type CommunityPostComment = {
  id: number
  author: string
  body: string
  helper_role?: string
  official?: boolean
  status?: string
  created_at?: string
}

type CommunityPost = {
  id?: number
  user_id?: number
  kind: CommunityPostKind
  section: string
  card: string
  time: string
  title: string
  excerpt: string
  body?: string
  tags?: string[]
  district?: string
  channel?: string
  source_type?: string
  source_id?: string
  scenario?: CommunityScenario | string
  subject_type?: string
  subject_id?: string
  subject_title?: string
  action_type?: CommunityActionType | string
  evidence?: unknown[]
  trust_signals?: Record<string, unknown>
  status?: string
  pinned?: boolean
  private?: boolean
  catches: number
  replies: number
  views: number
  comments?: CommunityPostComment[]
  created_at?: string
}

type CommentDrafts = Record<number, string>

type FeedbackItem = {
  category: string
  severity: string
  private: boolean
  time: string
  title: string
  body: string
  status?: string
  postId?: number
  evidenceCount?: number
  officialAnswered?: boolean
  acceptedAnswer?: boolean
}

const activeTokenBoard = ref<TokenBoardKey>('saving')
const activeDistrictKey = ref<ZeroCityWorkspaceKey>('home')
const activeChannelKey = ref('')
const activePostFilter = ref<PostFilterKey>('all')
const homeSearch = ref('')
const homeFeedMode = ref<HomeFeedMode>('recommended')
const showFeedRules = ref(false)
const expandedFeedPostIds = ref<number[]>([])
const feedFeedback = ref<Record<number, string>>({})
const selectedTagIndex = ref(0)
const activeArchiveEntry = ref<ArchiveEntryKey>('world')
const composerTitle = ref('')
const composerBody = ref('')
const composerOpen = ref(false)
const remotePosts = ref<CommunityPost[]>([])
const myPosts = ref<CommunityPost[]>([])
const isLoadingMyPosts = ref(false)
const myPostsLoadError = ref(false)
let myPostsRequestID = 0
const isLoadingPosts = ref(false)
const postLoadError = ref(false)
const isSubmittingPost = ref(false)
const commentDrafts = ref<CommentDrafts>({})
const forumCommentsStatus = ref<Record<number, 'loading' | 'ready' | 'error'>>({})
const submittingCommentId = ref<number | null>(null)
const operatingPostId = ref<number | null>(null)
const operatingCommentId = ref<number | null>(null)
const tokenPower = ref<TokenPowerLeaderboard | null>(null)
const isLoadingTokenPower = ref(false)
const rankingConfig = ref<ZeroCityRankingConfig>(normalizeRankingConfig(defaultZeroCityRankingConfig))
const rankingDetailsOpen = ref(false)
const rankingDetailsDefinition = ref<ZeroCityRankingDefinition>()

const selectedFeedbackCategory = ref<FeedbackCategoryKey>('api')
const selectedFeedbackSeverity = ref<FeedbackSeverityKey>('normal')
const feedbackTitle = ref('')
const feedbackBody = ref('')
const feedbackPrivate = ref(false)
const todayMascotDuty = getTodayMascotDuty()

const cityDuty = computed(() => ({
  name: todayMascotDuty.name,
  title: todayMascotDuty.title,
  line: todayMascotDuty.line,
  prompt: '把今天的线索交给值班台，零号城会分流到社群、资源、资产或反馈通道。',
  suggestions: todayMascotDuty.suggestions,
  src: zeroCityMascotSrc(todayMascotDuty.key)
}))

function firstQueryValue(value: unknown): string {
  if (Array.isArray(value)) return value.length > 0 ? String(value[0] ?? '') : ''
  return value == null ? '' : String(value)
}

function buildCommunityQuery(extra: Record<string, string | undefined> = {}) {
  const query: Record<string, string> = {}
  for (const key of ['previewAuth', 'source_type', 'source_id', 'subject_type', 'subject_id', 'kind']) {
    const value = firstQueryValue(route.query[key])
    if (value) query[key] = value
  }
  for (const [key, value] of Object.entries(extra)) {
    if (value) query[key] = value
  }
  return query
}

function syncWorkspaceFromRoute() {
  const workspace = firstQueryValue(route.query.workspace)
  if (workspace === 'mine') {
    activeDistrictKey.value = 'mine'
    activeChannelKey.value = ''
    activePostFilter.value = 'all'
    return
  }

  const districtKey = firstQueryValue(route.query.district) as ZeroCityWorkspaceKey
  const channelKey = firstQueryValue(route.query.channel)
  const district = zeroCityDistricts.value.find((item) => item.key === districtKey)
  if (!district) {
    activeDistrictKey.value = 'home'
    activeChannelKey.value = ''
    activePostFilter.value = 'all'
    return
  }

  const channel = district.channels.find((item) => item.key === channelKey) || district.channels[0]
  activeDistrictKey.value = district.key
  activeChannelKey.value = channel?.key || ''
  activePostFilter.value = channel?.filter || 'all'
}

function postMatchesActiveSubject(post: CommunityPost): boolean {
  if (!hasSourceFilter.value) return true
  return (post.source_type === activeSourceType.value && post.source_id === activeSourceId.value) ||
    (post.subject_type === activeSubjectType.value && post.subject_id === activeSubjectId.value)
}

const communityCommandMetrics = computed(() => {
  const scoped = hasSourceFilter.value ? posts.value.filter(postMatchesActiveSubject) : posts.value
  const evidenceTotal = scoped.reduce((sum, post) => sum + evidenceCount(post), 0)
  const acceptedTotal = scoped.filter(hasAcceptedAnswer).length
  const officialTotal = scoped.filter(hasOfficialConfirmation).length
  const resourceTotal = scoped.filter((post) => postMatchesFilter(post, 'resource')).length
  return [
    { label: 'Threads', value: scoped.length, title: '协作线索' },
    { label: 'Evidence', value: evidenceTotal, title: '证据挂点' },
    { label: 'Accepted', value: acceptedTotal, title: '采纳答案' },
    { label: 'Resource', value: resourceTotal + officialTotal, title: '资源与确认' }
  ]
})

const communityHealthSignals = computed(() => {
  const scoped = hasSourceFilter.value ? posts.value.filter(postMatchesActiveSubject) : posts.value
  const openQuestions = scoped.filter((post) => postMatchesFilter(post, 'qa') && !hasAcceptedAnswer(post)).length
  const resolvedTotal = scoped.filter((post) => hasPostRuntimeState(post, ['resolved', 'closed', 'confirmed']) || runtimeFlag(post, ['resolved'])).length
  const assetCandidates = scoped.filter((post) => runtimeFlag(post, ['saved_as_asset', 'asset_candidate']) || postMatchesFilter(post, 'asset')).length
  const tokenLeaders = tokenPowerRows().length
  return [
    { title: '问答闭环', note: openQuestions ? '仍有问题等待居民接力' : '等待第一条真实问答', value: openQuestions ? `${openQuestions} 待接` : '0', tone: 'signal-calm' },
    { title: '解决信号', note: '已解决、官方确认或已关闭的协作线索', value: resolvedTotal, tone: resolvedTotal ? 'signal-good' : 'signal-calm' },
    { title: '资产候选', note: '可进入能力资产馆的工作流、作品和案例', value: assetCandidates, tone: assetCandidates ? 'signal-hot' : 'signal-calm' },
    { title: '资源情报', note: '模型、工具、成本和 token 信号留在资源区', value: tokenLeaders ? `${tokenLeaders} 人` : '规则预览', tone: tokenLeaders ? 'signal-hot' : 'signal-calm' }
  ]
})

const zeroCityModules = computed(() => [
  { kicker: 'Community', title: '社群分区', note: '问答、经验、需求、能力、资源情报和产品讨论。', handler: () => activateDistrict('workshop') },
  { kicker: 'Assets', title: '能力资产', note: '工作流、Agent、案例和服务能力进入资产馆。', handler: () => router.push('/assets') },
  { kicker: 'Night', title: '夜间酒馆', note: '零号城夜间故事区，承接剧本、房间、复盘和积分消费。', handler: () => activateDistrict('tavern') },
  { kicker: 'Archive', title: '档案与身份', note: '世界解释、小人角色和居民身份沉淀到档案馆。', handler: () => activateDistrict('governance', 'rules') }
])

const primaryChannels = computed(() => [
  {
    key: 'dynamic',
    icon: '新',
    title: '动态',
    description: '浏览全城最新发布和已有协作进展。',
    note: '全部内容',
    tone: 'district-home',
    handler: () => activateCityHome()
  },
  {
    key: 'qa',
    icon: '问',
    title: '问答',
    description: '模型选择、报错、提示词和工具使用问题。',
    note: '可采纳回答',
    tone: 'district-workshop',
    handler: () => activateDistrict('workshop', 'help-desk')
  },
  {
    key: 'experience',
    icon: '验',
    title: '经验',
    description: '分享工作流、Agent、插件和踩坑复盘。',
    note: '可复用经验',
    tone: 'district-workshop',
    handler: () => activateDistrict('workshop', 'tech-share')
  },
  {
    key: 'intel',
    icon: '讯',
    title: '模型情报',
    description: '交流模型、API、成本、稳定性和可用渠道变化。',
    note: '资源线索',
    tone: 'district-market',
    handler: () => activateDistrict('workshop', 'news-radar')
  },
  {
    key: 'product',
    icon: '共',
    title: '产品共建',
    description: '参与 BizDecipher 功能建议、路线讨论和公开回应。',
    note: '产品讨论',
    tone: 'district-governance',
    handler: () => activateDistrict('governance', 'votes')
  }
])

const zeroCityDistricts = computed<ZeroCityDistrict[]>(() => [
  {
    key: 'tavern',
    icon: '🍻',
    title: '闲聊广场',
    rule: '慢一点，聊一会儿。',
    description: '晚上想找人说话、看看新剧本，或者只是坐一会儿，都可以来这里。',
    tone: 'district-tavern',
    handler: () => activateDistrict('tavern'),
    channels: [
      { key: 'chat-hall', label: '闲聊广场', filter: 'all', description: '公开帖子、话题与持续回复；不是实时房间。', kind: 'card', scenario: 'general', actionType: 'discuss', handler: () => activateDistrict('tavern', 'chat-hall') },
      { key: 'life-break', label: '生活摸鱼', filter: 'all', description: '生活状态、摸鱼记录、轻松话题和随手互助。', kind: 'card', scenario: 'general', actionType: 'discuss', handler: () => activateDistrict('tavern', 'life-break') },
      { key: 'deals', label: '薅羊毛与福利', filter: 'resource', description: '把值得分享的福利和资源放到这里。', kind: 'token', scenario: 'usage_intel', actionType: 'share_signal', handler: () => activateDistrict('tavern', 'deals') }
    ]
  },
  {
    key: 'workshop',
    icon: '🔧',
    title: '技术工坊',
    rule: '分享、提问、复盘。把技术经验沉淀成可复用答案。',
    description: '技术工坊只服务技术问题、工具工作流、新闻雷达和答疑互助，让帖子直接成为可检索的经验资产。',
    tone: 'district-workshop',
    handler: () => activateDistrict('workshop'),
    channels: [
      { key: 'tech-share', label: '教程与复盘', filter: 'workflow', description: '按步骤记录工具链、Agent、插件和可复用流程。', kind: 'card', scenario: 'delivery_collaboration', actionType: 'recommend', handler: () => activateDistrict('workshop', 'tech-share') },
      { key: 'news-radar', label: '情报雷达', filter: 'resource', description: '模型、API与工具变化需要来源、日期与可用范围。', kind: 'token', scenario: 'usage_intel', actionType: 'share_signal', handler: () => activateDistrict('workshop', 'news-radar') },
      { key: 'help-desk', label: '技术问答', filter: 'qa', description: '带环境与复现信息提问，用采纳结论收束。', kind: 'support', scenario: 'general', actionType: 'ask_help', handler: () => activateDistrict('workshop', 'help-desk') }
    ]
  },
  {
    key: 'market',
    icon: '🔒',
    title: '协作交流',
    rule: '把事情做成，再讲故事。',
    description: '有人带着作品来，也有人带着一件想做成的事来。',
    tone: 'district-market',
    visibility: '协作区',
    handler: () => activateDistrict('market'),
    channels: [
      { key: 'capability-showcase', label: '能力展示', filter: 'asset', description: '把自己做过的东西拿出来看看。', kind: 'card', scenario: 'capability_showcase', actionType: 'offer', handler: () => activateDistrict('market', 'capability-showcase') },
      { key: 'demand-posting', label: '需求发布', filter: 'demand', description: '说清一件想做的事，等合适的人接过来。', kind: 'card', scenario: 'demand_match', actionType: 'request', handler: () => activateDistrict('market', 'demand-posting') },
      { key: 'delivery-certification', label: '交付与认证', filter: 'asset', description: '把做完的事留给后来的人。', kind: 'card', scenario: 'delivery_collaboration', actionType: 'deliver', handler: () => activateDistrict('market', 'delivery-certification') }
    ]
  },
  {
    key: 'governance',
    icon: '⚖️',
    title: '城市活动',
    rule: '一座城也需要有人商量。',
    description: '这里放着大家真正想一起决定的事。',
    tone: 'district-governance',
    handler: () => activateDistrict('governance'),
    channels: [
      { key: 'votes', label: '投票大厅', filter: 'product', description: '产品路线、社区规则和功能优先级的公开讨论。', kind: 'feedback', scenario: 'announcement', actionType: 'discuss', handler: () => activateDistrict('governance', 'votes') },
      { key: 'badges', label: '勋章墙', filter: 'product', description: '围绕勋章、身份、贡献记录和权益的讨论。', kind: 'card', scenario: 'general', actionType: 'discuss', handler: () => activateDistrict('governance', 'badges') },
      { key: 'rules', label: '规则公示', filter: 'product', description: '城里的约定、变动和公共消息都放在这里。', kind: 'announcement', scenario: 'announcement', actionType: 'announce', handler: () => activateDistrict('governance', 'rules') }
    ]
  }
])

const isCityHome = computed(() => activeDistrictKey.value === 'home')
const isPersonalWorkspace = computed(() => activeDistrictKey.value === 'mine')
const legacyHomeSectionsVisible = computed(() => false)
const homeFeedModes: Array<{ key: HomeFeedMode; label: string; note: string }> = [
  { key: 'recommended', label: '城里推荐', note: '来自城里的真实讨论、经验和创作' },
  { key: 'latest', label: '最新', note: '按发布时间浏览' },
  { key: 'unanswered', label: '等你回应', note: '还没有收到回复的提问；你的第一句回应可能很重要' },
  { key: 'hot', label: '热议', note: '大家正在聊的事' },
  { key: 'technical', label: '技术精选', note: '值得留下的经验' },
  { key: 'following', label: '关注', note: '你关注的作者与城区' },
  { key: 'tavern', label: '酒馆', note: '水聊、活动与房间' },
  { key: 'official', label: '官方', note: '城里刚发生的更新' }
]
const activeFeedModeMeta = computed(() => homeFeedModes.find((mode) => mode.key === homeFeedMode.value) || homeFeedModes[0])
const activeDistrict = computed(() => zeroCityDistricts.value.find((district) => district.key === activeDistrictKey.value))
const activeSurface = computed(() => getZeroCitySurface(activeDistrictKey.value))
const activeWorkspaceChannels = computed(() => activeDistrict.value?.channels || [])
const activeChannelMeta = computed(() => activeWorkspaceChannels.value.find((channel) => channel.key === activeChannelKey.value) || activeWorkspaceChannels.value[0])
const activeWorkspaceTitle = computed(() => isPersonalWorkspace.value ? '我的零号城' : activeDistrict.value?.title || '全城动态')
const activeWorkspaceDescription = computed(() => isPersonalWorkspace.value ? '个人资料、作品、勋章和你在城里留下的回应，都收在这里。' : activeDistrict.value?.description || '全城动态')
const activeWorkspaceTone = computed(() => isPersonalWorkspace.value ? 'district-mine' : activeDistrict.value?.tone || '')
const activeChannelTitle = computed(() => activeChannelMeta.value?.label || activeWorkspaceTitle.value)
const activeChannelDescription = computed(() => activeChannelMeta.value?.description || activeWorkspaceDescription.value)
const activeCommunityDistrict = computed(() => activeDistrict.value?.key || '')
const activeCommunityChannel = computed(() => activeChannelMeta.value?.key || '')
const { canPost: canParticipate, label: participationLabel, loading: participationLoading, error: participationError, refresh: refreshParticipation } = useCommunityParticipation()
const composerAllowed = computed(() => canParticipate(
  isCityHome.value || isPersonalWorkspace.value ? '' : activeCommunityDistrict.value,
  isCityHome.value || isPersonalWorkspace.value ? '' : activeCommunityChannel.value,
))
const forumParticipationNote = computed(() => {
  if (participationLoading.value) return '正在读取参与资格…'
  if (participationError.value) return participationError.value
  if (composerAllowed.value) return ''
  if (activeCommunityDistrict.value === 'governance' && activeCommunityChannel.value === 'rules') {
    return '规则公示由管理员发布，所有居民均可阅读。'
  }
  return `${participationLabel.value} · 当前频道可阅读，发帖和回复需要 L1 参与资格。闲聊广场向所有居民开放。`
})
const composerDestinationKicker = computed(() => isCityHome.value ? '全城动态' : activeWorkspaceTitle.value)
const composerDestinationTitle = computed(() => isCityHome.value ? '全城动态' : `#${activeChannelTitle.value}`)
const composerDestinationDescription = computed(() => isCityHome.value
  ? '发布问题、经验、模型情报或产品讨论；需要正式业务表单时请进入对应模块。'
  : activeChannelDescription.value)
const composerTitlePlaceholder = computed(() => isCityHome.value
  ? t('community.composer.titlePlaceholder')
  : `给 #${activeChannelTitle.value} 写一个清楚标题`)
const composerBodyPlaceholder = computed(() => isCityHome.value
  ? t('community.composer.bodyPlaceholder')
  : `${activeChannelDescription.value}\n\n补充背景、希望有人帮什么，或你能带来什么。`)

const tavernScriptTiers = computed(() => [
  { badge: 'Asset', title: '剧本资产', note: '好剧本要能沉淀作者、试玩记录、复盘证据和后续收益。' },
  { badge: 'Room', title: '房间关系', note: '公开招募、加入、开始和完成服务居民关系，不抢主导航。' },
  { badge: 'Trust', title: '治理信号', note: '坏剧本、主持失败和违规诱导必须进入社群证据链。' }
])

function translateStringArray(key: string): string[] {
  const value = tm(key) as LocaleMessage
  return Array.isArray(value) ? value.map(String) : []
}

const tokenBoards = computed(() => [
  {
    key: 'saving' as const,
    title: t('community.tokenIntel.saving.title'),
    mascot: t('community.tokenIntel.saving.mascot'),
    badge: t('community.tokenIntel.saving.badge'),
    items: [
      { name: t('community.tokenIntel.saving.items.first.name'), value: t('community.tokenIntel.saving.items.first.value'), note: t('community.tokenIntel.saving.items.first.note') },
      { name: t('community.tokenIntel.saving.items.second.name'), value: t('community.tokenIntel.saving.items.second.value'), note: t('community.tokenIntel.saving.items.second.note') },
    ],
  },
  {
    key: 'stable' as const,
    title: t('community.tokenIntel.stable.title'),
    mascot: t('community.tokenIntel.stable.mascot'),
    badge: t('community.tokenIntel.stable.badge'),
    items: [
      { name: t('community.tokenIntel.stable.items.first.name'), value: t('community.tokenIntel.stable.items.first.value'), note: t('community.tokenIntel.stable.items.first.note') },
      { name: t('community.tokenIntel.stable.items.second.name'), value: t('community.tokenIntel.stable.items.second.value'), note: t('community.tokenIntel.stable.items.second.note') },
    ],
  },
  {
    key: 'alert' as const,
    title: t('community.tokenIntel.alert.title'),
    mascot: t('community.tokenIntel.alert.mascot'),
    badge: t('community.tokenIntel.alert.badge'),
    items: [
      { name: t('community.tokenIntel.alert.items.first.name'), value: t('community.tokenIntel.alert.items.first.value'), note: t('community.tokenIntel.alert.items.first.note') },
      { name: t('community.tokenIntel.alert.items.second.name'), value: t('community.tokenIntel.alert.items.second.value'), note: t('community.tokenIntel.alert.items.second.note') },
    ],
  },
  {
    key: 'contribute' as const,
    title: t('community.tokenIntel.contribute.title'),
    mascot: t('community.tokenIntel.contribute.mascot'),
    badge: t('community.tokenIntel.contribute.badge'),
    items: [
      { name: t('community.tokenIntel.contribute.items.first.name'), value: t('community.tokenIntel.contribute.items.first.value'), note: t('community.tokenIntel.contribute.items.first.note') },
      { name: t('community.tokenIntel.contribute.items.second.name'), value: t('community.tokenIntel.contribute.items.second.value'), note: t('community.tokenIntel.contribute.items.second.note') },
    ],
  },
])

const visibleTokenBoards = computed(() =>
  tokenBoards.value.filter((board) => board.key === activeTokenBoard.value)
)

const tokenPowerPeriodLabel = computed(() => {
  if (isLoadingTokenPower.value) return '正在读取榜单...'
  const leaderboard = tokenPower.value
  return leaderboard
    ? `${leaderboard.week_start} - ${leaderboard.week_end}`
    : '当前暂无榜单数据。'
})

const tokenRules = computed(() => {
  const remoteRules = tokenPower.value?.rules || []
  if (remoteRules.length) {
    return remoteRules.map((rule, index) => ({
      title: index === 0 ? '有效付费' : index === 1 ? '反刷复核' : '榜一免单',
      description: rule
    }))
  }
  return [
    { title: t('community.tokenIntel.rules.privacy.title'), description: t('community.tokenIntel.rules.privacy.description') },
    { title: t('community.tokenIntel.rules.utility.title'), description: t('community.tokenIntel.rules.utility.description') },
    { title: t('community.tokenIntel.rules.noSpend.title'), description: t('community.tokenIntel.rules.noSpend.description') },
  ]
})

const postFilters = computed(() => [
  { key: 'all' as const, label: '动态' },
  { key: 'qa' as const, label: '问答' },
  { key: 'workflow' as const, label: '经验' },
  { key: 'resource' as const, label: '模型情报' },
  { key: 'product' as const, label: '产品共建' }
])

const posts = computed<CommunityPost[]>(() => {
  const seen = new Set<string>()
  const merged: CommunityPost[] = []
  for (const post of remotePosts.value) {
    const key = post.id ? `id:${post.id}` : `${post.title}:${post.time}`
    if (seen.has(key)) continue
    seen.add(key)
    merged.push(post)
  }
  return merged
})

const filteredPosts = computed(() => {
  const scopedPosts = hasSourceFilter.value
    ? posts.value.filter(postMatchesActiveSubject)
    : posts.value

  if (hasSourceFilter.value || activePostFilter.value === 'all') {
    return rankCommunityPosts(scopedPosts)
  }

  return rankCommunityPosts(scopedPosts.filter((post) => postMatchesFilter(post, activePostFilter.value)))
})

const workspacePosts = computed(() => {
  if (isCityHome.value || isPersonalWorkspace.value) return []
  return rankCommunityPosts(posts.value.filter((post) => {
    if (post.district && post.district !== activeCommunityDistrict.value) return false
    if (post.channel && post.channel !== activeCommunityChannel.value) return false
    return postMatchesFilter(post, activeChannelMeta.value?.filter || 'all')
  }))
})

const feedbackPosts = computed<FeedbackItem[]>(() =>
  rankCommunityPosts(posts.value.filter((post) => postMatchesFilter(post, 'feedback')))
    .map((post) => ({
      category: post.tags?.[0] || post.section || 'Feedback',
      severity: String(post.trust_signals?.severity || post.tags?.[1] || normalizedStatus(post.status) || 'normal'),
      private: Boolean(post.private),
      time: post.time,
      title: post.title,
      body: post.body || post.excerpt,
      status: post.status,
      postId: post.id,
      evidenceCount: evidenceCount(post),
      officialAnswered: hasOfficialConfirmation(post),
      acceptedAnswer: hasAcceptedAnswer(post)
    }))
)

const archiveEntryFallback: ArchiveEntryLink = { key: 'world', label: '城市生活', title: '留灯的人，也需要一个住处', description: '微光街、未打烊巷、重试工坊、蓝时海岸与余晖档案街。这里是零号城世界观，不是真实用户动态。', action: '走进城区' }
const archiveEntryLinks: ArchiveEntryLink[] = [
  archiveEntryFallback,
  { key: 'chronicle', label: '编年史', title: '第一卷：留灯的人', description: '地图少了一条路，值班表少了一页，桥那边还有人拒绝回来。六篇连续故事从断光前夜追到开城日。', action: '阅读编年史' },
  { key: 'mascot', label: '居民故事', title: '不救场的日子，他们在做什么', description: '八位已有收藏卡角色的住所、日常、愿望与熟人。走进故事，也可以回到他们各自的卡面。', action: '认识居民' },
  { key: 'rules', label: '社群规则', title: '城市故事与社群规则，各有归处', description: '这里查看已经公示的社群规则。世界观故事不会改变真实账户、权限或资产权益。', action: '查看规则公示' }
]

const activeArchiveEntryMeta = computed<ArchiveEntryLink>(() => archiveEntryLinks.find((item) => item.key === activeArchiveEntry.value) || archiveEntryFallback)

const quickTags = computed(() => translateStringArray('community.composer.quickTags'))
const selectedQuickTag = computed(() => quickTags.value[selectedTagIndex.value] || quickTags.value[0] || '')

const feedbackCategories = computed(() => [
  { key: 'api' as const, label: t('community.feedback.categories.api') },
  { key: 'payment' as const, label: t('community.feedback.categories.payment') },
  { key: 'pool' as const, label: t('community.feedback.categories.pool') },
  { key: 'idea' as const, label: t('community.feedback.categories.idea') },
  { key: 'governance' as const, label: t('community.feedback.categories.governance') },
])

const feedbackSeverities = computed(() => [
  { key: 'normal' as const, label: t('community.feedback.severity.normal') },
  { key: 'urgent' as const, label: t('community.feedback.severity.urgent') },
  { key: 'private' as const, label: t('community.feedback.severity.private') },
])

const helperRoles = computed(() => [
  { title: t('community.support.roles.navigator.title'), description: t('community.support.roles.navigator.description'), badge: t('community.support.roles.navigator.badge') },
  { title: t('community.support.roles.triage.title'), description: t('community.support.roles.triage.description'), badge: t('community.support.roles.triage.badge') },
  { title: t('community.support.roles.pool.title'), description: t('community.support.roles.pool.description'), badge: t('community.support.roles.pool.badge') },
])

const runtimeRuleCards = computed(() => {
  const scoped = hasSourceFilter.value ? posts.value.filter(postMatchesActiveSubject) : posts.value
  const answeredCount = scoped.filter((post) => hasPostRuntimeState(post, ['answered', 'accepted'])).length
  const acceptedCount = scoped.filter(hasAcceptedAnswer).length
  const officialCount = scoped.filter(hasOfficialConfirmation).length
  const resolvedCount = scoped.filter((post) => hasPostRuntimeState(post, ['resolved', 'closed', 'confirmed']) || runtimeFlag(post, ['resolved'])).length
  const featuredCount = scoped.filter((post) => post.pinned || hasPostRuntimeState(post, ['featured']) || runtimeFlag(post, ['featured'])).length
  const reputationTotal = scoped.reduce((sum, post) => sum + runtimeNumber(post, ['reputation', 'contribution', 'contribution_score']), 0)
  return [
    { title: '答案闭环', value: `${answeredCount}/${acceptedCount}`, note: '已回答 / 已采纳', tone: acceptedCount ? 'good' : 'neutral' },
    { title: '官方确认', value: `${officialCount}`, note: '管理员、池主或官方回复', tone: officialCount ? 'gold' : 'neutral' },
    { title: '问题收口', value: `${resolvedCount}`, note: '已解决、已关闭或已确认', tone: resolvedCount ? 'good' : 'neutral' },
    { title: '精华与声誉', value: `${featuredCount} / +${reputationTotal}`, note: '精华线索 / 贡献声誉', tone: featuredCount || reputationTotal ? 'gold' : 'neutral' },
  ]
})

function formatCommunityTime(value?: string): string {
  if (!value) return t('community.composer.localTime')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const diff = Date.now() - date.getTime()
  if (diff < 60_000) return t('community.composer.localTime')
  if (diff < 3_600_000) return `${Math.max(1, Math.floor(diff / 60_000))} 分钟前`
  if (diff < 86_400_000) return `${Math.max(1, Math.floor(diff / 3_600_000))} 小时前`
  return date.toLocaleDateString()
}

const activeSourceType = computed(() => String(route.query.source_type || route.query.subject_type || ''))
const activeSourceId = computed(() => String(route.query.source_id || route.query.subject_id || ''))
const activeSubjectType = computed(() => String(route.query.subject_type || route.query.source_type || ''))
const activeSubjectId = computed(() => String(route.query.subject_id || route.query.source_id || ''))
const hasSourceFilter = computed(() => activeSourceType.value !== '' && activeSourceId.value !== '')
const activeSubjectChip = computed(() => {
  if (!hasSourceFilter.value) return ''
  const matchedPost = posts.value.find((post) => postMatchesActiveSubject(post) && (post.subject_title || post.card || post.title))
  if (matchedPost?.subject_title) return matchedPost.subject_title
  if (matchedPost?.card && !/^#?\d+$/.test(matchedPost.card)) return matchedPost.card
  if (activeSubjectType.value === 'shared_pool') return `资源主体 ${activeSubjectId.value}`
  return `${activeSubjectType.value || '主体'} ${activeSubjectId.value}`
})
const currentUserId = computed(() => Number(authStore.user?.id || 0))
const canModerateCommunity = computed(() => Boolean(authStore.isAdmin))

const scopedHomePosts = computed(() => {
  const scoped = hasSourceFilter.value ? posts.value.filter(postMatchesActiveSubject) : posts.value
  return rankCommunityPosts(scoped)
})

const visibleRankingDefinitions = computed<ZeroCityRankingDefinition[]>(() =>
  rankingConfig.value.items
    .filter((item) => item.enabled)
    .sort((left, right) => left.order - right.order),
)

const rankingRowsByKey = computed<Readonly<Record<string, readonly ZeroCityRankingRow[]>>>(() => {
  const rankingPosts: ZeroCityRankingPost[] = scopedHomePosts.value.map((post) => ({
    id: post.id,
    userId: post.user_id,
    author: post.card || '居民',
    card: post.card,
    createdAt: post.created_at,
    kind: post.kind,
    district: post.district,
    scenario: post.scenario,
    status: post.status,
    tags: post.tags,
    catches: post.catches,
    replies: post.replies,
    views: post.views,
    evidenceCount: evidenceCount(post),
    official: hasOfficialConfirmation(post),
    accepted: hasAcceptedAnswer(post),
  }))

  const rows = Object.fromEntries(
    visibleRankingDefinitions.value.map((definition) => [
      definition.key,
      buildZeroCityRankingRows(definition, rankingPosts),
    ]),
  )

  const spendingDefinition = visibleRankingDefinitions.value.find((definition) => definition.source === 'spending')
  if (spendingDefinition) {
    rows[spendingDefinition.key] = tokenPowerRows()
      .slice(0, spendingDefinition.limit)
      .map((row) => ({
        rank: row.rank,
        label: row.display_name,
        value: formatMoney(row.effective_spend),
        secondary: `${row.request_count} 次有效付费调用 · Token仅作明细`,
        userId: row.user_id,
      }))
  }

  return rows
})
const rankingDetailsRows = computed<readonly ZeroCityRankingRow[]>(() =>
  rankingDetailsDefinition.value ? rankingRowsByKey.value[rankingDetailsDefinition.value.key] || [] : [],
)

function postSearchText(post: CommunityPost): string {
  return [post.title, post.excerpt, post.body, post.section, post.card, post.district, post.channel, post.scenario, post.subject_title, ...(post.tags || [])]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
}

function postMomentum(post: CommunityPost): number {
  return post.replies * 5 + post.catches * 4 + post.views * 0.08 + evidenceCount(post) * 8 + (hasAcceptedAnswer(post) ? 24 : 0) + (hasOfficialConfirmation(post) ? 28 : 0)
}

function isTechnicalPost(post: CommunityPost): boolean {
  return post.district === 'workshop' || post.kind === 'token' || post.kind === 'support' || post.scenario === 'usage_intel' || post.scenario === 'delivery_collaboration' || post.scenario === 'incident_support'
}

function isTavernPost(post: CommunityPost): boolean {
  return post.district === 'tavern' || /酒馆|生活|水聊|福利/.test(`${post.section} ${post.channel || ''} ${post.title}`)
}

function isOfficialPost(post: CommunityPost): boolean {
  return isOfficialCommunityPost(post, hasOfficialConfirmation(post))
}

function isFollowedPost(post: CommunityPost): boolean {
  return runtimeFlag(post, ['following', 'followed', 'is_following']) || (currentUserId.value > 0 && Number(post.user_id || 0) === currentUserId.value)
}

function clamp01(value: number): number {
  return Math.max(0, Math.min(1, value))
}

function homeRecommendationScore(post: CommunityPost): number {
  const relevance = clamp01(0.38 + (isFollowedPost(post) ? 0.34 : 0) + (postMatchesActiveSubject(post) ? 0.12 : 0) + (post.tags?.length ? 0.04 : 0))
  const quality = clamp01((hasAcceptedAnswer(post) ? 0.34 : 0) + (hasOfficialConfirmation(post) ? 0.22 : 0) + Math.min(evidenceCount(post), 4) * 0.1 + Math.min(post.catches, 20) * 0.008 + (runtimeFlag(post, ['saved_as_asset', 'asset_reused']) ? 0.16 : 0))
  const trust = clamp01((hasOfficialConfirmation(post) ? 0.36 : 0) + (evidenceCount(post) > 0 ? 0.28 : 0) + (runtimeNumber(post, ['reputation', 'contribution', 'contribution_score']) > 0 ? 0.18 : 0) + (post.status === 'confirmed' ? 0.18 : 0))
  const freshness = clamp01(postFreshnessScore(post) / 100)
  const exploration = clamp01(runtimeFlag(post, ['new_author', 'explore', 'underexposed']) ? 0.9 : (post.user_id ? 0.42 : 0.2))
  const diversity = isTechnicalPost(post) || isTavernPost(post) || isOfficialPost(post) ? 0.72 : 0.48
  const repetition = clamp01(runtimeNumber(post, ['repeat_exposure', 'impression_count']) / 8)
  const negative = clamp01(runtimeNumber(post, ['negative_feedback', 'reports', 'hides']) / 5)
  const bait = clamp01(runtimeNumber(post, ['bait_penalty', 'controversy_penalty']) / 5 + (runtimeFlag(post, ['ragebait', 'clickbait']) ? 0.5 : 0))
  return 0.30 * relevance + 0.20 * quality + 0.15 * trust + 0.15 * freshness + 0.10 * exploration + 0.10 * diversity - 0.10 * repetition - 0.10 * negative - 0.10 * bait
}

function rankHomeFeedPosts(items: CommunityPost[]): CommunityPost[] {
  const ranked = items
    .map((post, index) => ({ post, index, score: homeRecommendationScore(post) }))
    .sort((left, right) => right.score - left.score || left.index - right.index)
  const authors = new Map<string, number>()
  const districts = new Map<string, number>()
  const deferred: CommunityPost[] = []
  const selected: CommunityPost[] = []

  for (const item of ranked) {
    const author = String(item.post.user_id || item.post.card || 'resident')
    const district = String(item.post.district || item.post.section || 'city')
    const authorCap = selected.length < 10 ? 2 : 3
    const districtCap = selected.length < 10 ? 4 : 6
    if ((authors.get(author) || 0) >= authorCap || (districts.get(district) || 0) >= districtCap) {
      deferred.push(item.post)
      continue
    }
    selected.push(item.post)
    authors.set(author, (authors.get(author) || 0) + 1)
    districts.set(district, (districts.get(district) || 0) + 1)
  }

  return [...selected, ...deferred]
}

const homeFeedPosts = computed(() => {
  const keyword = homeSearch.value.trim().toLowerCase()
  let result = keyword
    ? scopedHomePosts.value.filter((post) => postSearchText(post).includes(keyword))
    : [...scopedHomePosts.value]

  if (homeFeedMode.value === 'latest') return result.sort((left, right) => postFreshnessScore(right) - postFreshnessScore(left))
  if (homeFeedMode.value === 'unanswered') {
    return result.filter(post => post.replies === 0 && !hasAcceptedAnswer(post)
      && (post.kind === 'support' || post.channel === 'help-desk'))
      .sort((left, right) => postFreshnessScore(right) - postFreshnessScore(left))
  }
  if (homeFeedMode.value === 'hot') return result.sort((left, right) => postMomentum(right) - postMomentum(left))
  if (homeFeedMode.value === 'technical') return rankHomeFeedPosts(result.filter(isTechnicalPost))
  if (homeFeedMode.value === 'following') {
    return rankHomeFeedPosts(result.filter(isFollowedPost))
  }
  if (homeFeedMode.value === 'tavern') return rankHomeFeedPosts(result.filter(isTavernPost))
  if (homeFeedMode.value === 'official') return result.filter(isOfficialPost).sort((left, right) => postFreshnessScore(right) - postFreshnessScore(left))

  return rankHomeFeedPosts(result)
})

const standardAnswerPosts = computed(() =>
  scopedHomePosts.value
    .filter((post) => hasAcceptedAnswer(post) || hasOfficialConfirmation(post) || hasPostRuntimeState(post, ['confirmed', 'featured']))
    .slice(0, 4)
)

const latestHomePosts = computed(() => scopedHomePosts.value.slice(0, 4))

const hotHomePosts = computed(() =>
  scopedHomePosts.value
    .filter((post) => post.replies > 0 || post.catches > 0 || post.views > 0 || hasAcceptedAnswer(post) || hasOfficialConfirmation(post) || evidenceCount(post) > 0)
    .slice(0, 3)
)

const featuredHomePosts = computed(() =>
  scopedHomePosts.value
    .filter((post) => post.pinned || hasAcceptedAnswer(post) || hasOfficialConfirmation(post) || runtimeFlag(post, ['saved_as_asset', 'asset_candidate']) || hasPostRuntimeState(post, ['featured', 'confirmed']))
    .slice(0, 3)
)

const zeroCityHistory = computed(() => {
  const ownPosts = myPosts.value
  const accepted = ownPosts.filter(hasAcceptedAnswer).length
  const confirmed = ownPosts.filter(hasOfficialConfirmation).length
  const assets = ownPosts.filter((post) => postMatchesFilter(post, 'asset') || runtimeFlag(post, ['saved_as_asset', 'asset_candidate'])).length
  return { posts: ownPosts.length, accepted, confirmed, assets }
})

const zeroCityProgressCards = computed(() => [
  { label: '我的帖子', value: zeroCityHistory.value.posts, note: '已从个人社区历史读取' },
  { label: '已采纳', value: zeroCityHistory.value.accepted, note: '有采纳信号的真实记录' },
  { label: '已确认', value: zeroCityHistory.value.confirmed, note: '有官方确认信号的真实记录' },
  { label: '资产线索', value: zeroCityHistory.value.assets, note: '需回到原始资产服务继续确认' },
])

const zeroCityDailyTasks = computed(() => [
  { reward: '+10 XP', title: '发一条真实动态', note: '问题、经验、需求、资源线索都可以先入城。', handler: () => activateDistrict('workshop', 'help-desk') },
  { reward: '+15 XP', title: '接一个答疑互助', note: '高质量回复被采纳后才进入可信积分。', handler: () => activateDistrict('workshop', 'help-desk') },
  { reward: '+25 声誉', title: '补一条证据链', note: '能力展示、需求、交付都必须用证据说话。', handler: () => activateDistrict('market', 'capability-showcase') },
  { reward: '解锁', title: '完善我的零号城', note: '资料、资产和勋章会影响可信市场权限。', handler: activatePersonalHub }
])

function postMatchesFilter(post: CommunityPost, filter: PostFilterKey): boolean {
  if (filter === 'all') return true
  const haystack = [post.section, post.card, post.title, post.excerpt, post.scenario, post.action_type, post.kind, ...(post.tags || [])]
    .join(' ')
    .toLowerCase()
  switch (filter) {
    case 'qa': return post.scenario === 'general' || post.kind === 'support' || /问答|经验|q&a|问题|答案|踩坑/.test(haystack)
    case 'workflow': return post.scenario === 'delivery_collaboration' || /工作流|workflow|工具|模板|agent|插件|流程/.test(haystack)
    case 'demand': return post.scenario === 'demand_match' || post.action_type === 'request' || /需求|任务|协作|委托|task/.test(haystack)
    case 'asset': return post.scenario === 'capability_showcase' || /能力|作品|案例|展示|asset|showcase/.test(haystack)
    case 'resource': return post.scenario === 'resource_decision' || post.scenario === 'usage_intel' || post.kind === 'pool' || /资源|模型|api|通道|成本|稳定/.test(haystack)
    case 'product': return post.scenario === 'announcement' || /产品|路线|公告|规则|建议|讨论/.test(haystack)
    case 'support': return post.scenario === 'incident_support' || post.kind === 'support'
    case 'feedback': return post.scenario === 'feedback_triage' || post.kind === 'feedback'
    default: return false
  }
}

function mapRemotePost(post: import('@/features/bizdecipher/api/community').CommunityPost): CommunityPost {
  const firstTag = post.tags?.[0] || post.kind
  return {
    id: post.id,
    user_id: post.user_id,
    kind: post.kind === 'announcement' ? 'feedback' : post.kind,
    section: firstTag || post.kind,
    card: post.author || 'Decoder',
    time: formatCommunityTime(post.created_at),
    title: post.title,
    excerpt: post.body,
    body: post.body,
    tags: post.tags || [],
    district: post.district,
    channel: post.channel,
    source_type: post.source_type,
    source_id: post.source_id,
    scenario: post.scenario,
    subject_type: post.subject_type,
    subject_id: post.subject_id,
    subject_title: post.subject_title,
    action_type: post.action_type,
    evidence: Array.isArray(post.evidence) ? post.evidence : [],
    trust_signals: post.trust_signals || {},
    status: post.status,
    pinned: post.pinned,
    catches: post.catches || 0,
    replies: post.replies || 0,
    views: post.views || 0,
    comments: post.comments || [],
    created_at: post.created_at,
  }
}

function mapPreviewPosts(): CommunityPost[] {
  const ages = [1, 12, 28, 41, 60, 120, 1440, 180]
  return communityPreviewPosts.map((post, index) => ({
    ...post,
    created_at: new Date(Date.now() - (ages[index] || 1) * 60 * 1000).toISOString(),
  }))
}

let postLoadSequence = 0
async function loadCommunityPosts() {
  const sequence = ++postLoadSequence
  isLoadingPosts.value = true
  try {
    const response = await communityAPI.listPosts({
      kind: 'all',
      district: isCityHome.value || isPersonalWorkspace.value ? undefined : activeCommunityDistrict.value,
      channel: isCityHome.value || isPersonalWorkspace.value ? undefined : activeCommunityChannel.value,
      source_type: activeSourceType.value,
      source_id: activeSourceId.value,
      subject_type: activeSubjectType.value,
      subject_id: activeSubjectId.value,
      limit: 80
    })
    if (sequence !== postLoadSequence) return
    const mappedPosts = response.items.map(mapRemotePost)
    // The explicit localhost preview bypass is a visual review surface, not a
    // production empty state. Keep it populated when a local API returns an
    // otherwise-valid but empty fixture response.
    remotePosts.value = isLocalPreviewAuth() && mappedPosts.length === 0 ? mapPreviewPosts() : mappedPosts
    postLoadError.value = false
  } catch {
    if (sequence !== postLoadSequence) return
    if (isLocalPreviewAuth()) {
      remotePosts.value = mapPreviewPosts()
      postLoadError.value = false
    } else {
      remotePosts.value = []
      postLoadError.value = true
    }
  } finally {
    if (sequence === postLoadSequence) isLoadingPosts.value = false
  }
}

async function loadForumComments(post: CommunityPost) {
  if (!post.id || forumCommentsStatus.value[post.id] === 'loading') return
  const id = post.id
  forumCommentsStatus.value[id] = 'loading'
  try {
    const response = await communityAPI.listComments(id, 100)
    const current = remotePosts.value.find(item => item.id === id)
    if (current) current.comments = response.items.slice().sort((a, b) => a.id - b.id)
    forumCommentsStatus.value[id] = 'ready'
  } catch {
    forumCommentsStatus.value[id] = 'error'
  }
}

async function loadMyCommunityPosts() {
  const requestID = ++myPostsRequestID
  const userID = currentUserId.value
  if (currentUserId.value <= 0) {
    myPosts.value = []
    myPostsLoadError.value = false
    isLoadingMyPosts.value = false
    return
  }

  isLoadingMyPosts.value = true
  myPostsLoadError.value = false
  try {
    const response = await communityAPI.listMyPosts('all', 200)
    if (requestID !== myPostsRequestID || currentUserId.value !== userID) return
    myPosts.value = response.items.map(mapRemotePost)
  } catch {
    if (requestID !== myPostsRequestID || currentUserId.value !== userID) return
    myPosts.value = []
    myPostsLoadError.value = true
  } finally {
    if (requestID === myPostsRequestID) isLoadingMyPosts.value = false
  }
}

async function loadTokenPower() {
  isLoadingTokenPower.value = true
  try {
    tokenPower.value = await communityAPI.getTokenPower(10)
  } catch {
    tokenPower.value = null
  } finally {
    isLoadingTokenPower.value = false
  }
}

async function loadRankingConfig() {
  try {
    const loaded = normalizeRankingConfig(await loadZeroCityRankingConfig())
    if (loaded.items.length) rankingConfig.value = loaded
  } catch {
    rankingConfig.value = normalizeRankingConfig(defaultZeroCityRankingConfig)
  }
}

watch(
  () => [route.query.district, route.query.channel, route.query.workspace],
  () => syncWorkspaceFromRoute(),
  { immediate: true }
)

watch([activePostFilter, activeDistrictKey, activeChannelKey, activeSourceType, activeSourceId], () => {
  void loadCommunityPosts()
}, { immediate: true })

onMounted(() => {
  loadTokenPower()
  loadRankingConfig()
})

watch(currentUserId, () => { void loadMyCommunityPosts() }, { immediate: true })

function jumpToSections() {
  document.getElementById('community-sections')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function setHomeFeedMode(value: string | number) {
  const mode = homeFeedModes.find((item) => item.key === value)
  if (mode) homeFeedMode.value = mode.key
}

function jumpToFeedback() {
  activateDistrict('governance', 'votes')
}

function jumpToTavern() {
  activateDistrict('tavern')
}

function selectArchiveEntry(key: ArchiveEntryKey) {
  activeArchiveEntry.value = key
}

function openArchiveEntry(key: ArchiveEntryKey) {
  if (key === 'rules') {
    activateDistrict('governance', 'rules')
    return
  }
  void router.push({
    path: '/zero-city/chronicle',
    query: key === 'world' ? { view: 'districts' } : key === 'mascot' ? { view: 'residents' } : {},
  })
}

function activateChannel(filter: PostFilterKey) {
  const district = zeroCityDistricts.value.find((item) => item.channels.some((channel) => channel.filter === filter))
  const channel = district?.channels.find((item) => item.filter === filter)
  if (district && channel) {
    activateDistrict(district.key, channel.key)
    return
  }
  activePostFilter.value = filter
}

function activateCityHome() {
  router.push({ path: '/community', query: buildCommunityQuery() })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function activateDistrict(key: ZeroCityWorkspaceKey, channelKey = '') {
  if (key === 'home') {
    activateCityHome()
    return
  }
  if (key === 'mine') {
    activatePersonalHub()
    return
  }
  const district = zeroCityDistricts.value.find((item) => item.key === key)
  const channel = district?.channels.find((item) => item.key === channelKey) || district?.channels[0]
  if (!district || !channel) return
  router.push({
    path: '/community',
    query: buildCommunityQuery({ district: district.key, channel: channel.key })
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function activatePersonalHub() {
  router.push({ path: '/community', query: buildCommunityQuery({ workspace: 'mine' }) })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function jumpToComposer() {
  composerOpen.value = true
}

function closeComposer() {
  if (isSubmittingPost.value) return
  composerOpen.value = false
}

function setComposerOpen(open: boolean) {
  if (open) {
    composerOpen.value = true
    return
  }
  closeComposer()
}

function openFormalCreate(path: string) {
  composerOpen.value = false
  router.push(path)
}

function prefillComposer(title: string) {
  composerTitle.value = title
  jumpToComposer()
}

function openZeroCityProfile(userID: number) {
  router.push(`/zero-city/${userID}`)
}

function handleRankingSelect(row: ZeroCityRankingRow, definition?: ZeroCityRankingDefinition) {
  if (row.userId) {
    openZeroCityProfile(row.userId)
    return
  }
  handleRankingOpen(definition)
}

function handleRankingOpen(definition?: ZeroCityRankingDefinition) {
  if (!definition) return
  rankingDetailsDefinition.value = definition
  rankingDetailsOpen.value = true
}

function handleRankingDetailsSelect(row: ZeroCityRankingRow): void {
  rankingDetailsOpen.value = false
  handleRankingSelect(row, rankingDetailsDefinition.value)
}

function openRankingRelatedDistrict(): void {
  const routePath = rankingDetailsDefinition.value?.route
  if (!routePath) return
  rankingDetailsOpen.value = false
  router.push(routePath)
}

function openSharedPoolDiscussion(sourceID: string | number) {
  router.push({ path: '/community', query: { source_type: 'shared_pool', source_id: String(sourceID), kind: 'pool' } })
}

function featuredReason(post: CommunityPost): string {
  if (post.pinned || hasPostRuntimeState(post, ['featured'])) return '精华内容'
  if (hasAcceptedAnswer(post)) return '已采纳答案'
  if (hasOfficialConfirmation(post)) return '官方确认'
  if (runtimeFlag(post, ['saved_as_asset', 'asset_candidate'])) return '资产候选'
  if (hasPostRuntimeState(post, ['confirmed'])) return '已确认信号'
  return '精选沉淀'
}

function postKindLabel(post: CommunityPost): string {
  if (isResidentProposal(post)) return '议'
  if (isOfficialPost(post)) return '告'
  if (isTechnicalPost(post)) return '工'
  if (isTavernPost(post)) return '酒'
  if (post.kind === 'pool' || post.scenario === 'capability_showcase' || post.scenario === 'demand_match') return '协'
  return '城'
}

function postStatusLabel(post: CommunityPost): string {
  if (hasOfficialConfirmation(post)) return '官方确认'
  if (hasAcceptedAnswer(post)) return '已采纳'
  if (hasPostRuntimeState(post, ['resolved', 'closed']) || runtimeFlag(post, ['resolved'])) return '已解决'
  if (normalizedStatus(post.status) === 'blocked') return '需处理'
  if (normalizedStatus(post.status) === 'reviewing') return '待验证'
  if (post.pinned || hasPostRuntimeState(post, ['featured'])) return '精选'
  return ''
}

function postStatusTone(post: CommunityPost): 'good' | 'gold' | 'risk' | 'neutral' {
  if (hasOfficialConfirmation(post) || hasAcceptedAnswer(post) || hasPostRuntimeState(post, ['resolved', 'closed'])) return 'good'
  if (post.pinned || hasPostRuntimeState(post, ['featured'])) return 'gold'
  if (normalizedStatus(post.status) === 'blocked' || runtimeFlag(post, ['ragebait', 'clickbait'])) return 'risk'
  return 'neutral'
}

function recommendationReason(post: CommunityPost): string {
  if (hasOfficialConfirmation(post)) return '官方确认'
  if (hasAcceptedAnswer(post)) return '已采纳答案'
  if (evidenceCount(post) > 0) return '有证据可复用'
  if (runtimeFlag(post, ['saved_as_asset', 'asset_reused', 'asset_candidate'])) return '正在沉淀为资产'
  if (runtimeFlag(post, ['new_author', 'explore', 'underexposed'])) return '新作者探索位'
  if (isTavernPost(post)) return '酒馆城市动态'
  if (postFreshnessScore(post) >= 70) return '刚刚发生'
  return '城市讨论与经验'
}

function isFeedPostExpanded(post: CommunityPost): boolean {
  return Boolean(post.id && expandedFeedPostIds.value.includes(post.id))
}

function toggleFeedPost(post: CommunityPost): void {
  if (!post.id) return
  expandedFeedPostIds.value = isFeedPostExpanded(post)
    ? expandedFeedPostIds.value.filter((id) => id !== post.id)
    : [...expandedFeedPostIds.value, post.id]
}

function setFeedFeedback(post: CommunityPost, feedback: string): void {
  if (!post.id) return
  feedFeedback.value = { ...feedFeedback.value, [post.id]: feedback }
  post.trust_signals = { ...(post.trust_signals || {}), negative_feedback: runtimeNumber(post, ['negative_feedback']) + 1 }
  appStore.showSuccess(`已记录：${feedback}`)
}

function selectComposerTag(index: number) {
  selectedTagIndex.value = index
}

function resolvePostKind(): CommunityPostKind {
  const tag = selectedQuickTag.value.toLowerCase()
  if (tag.includes('token')) return 'token'
  if (tag.includes('反馈') || tag.includes('feedback') || tag.includes('обрат')) return 'feedback'
  if (tag.includes('支付') || tag.includes('redeem') || tag.includes('api') || tag.includes('сбой')) return 'support'
  if (tag.includes('共享') || tag.includes('pool')) return 'pool'
  return 'card'
}

function scenarioForKind(kind: CommunityPostKind): CommunityScenario {
  if (hasSourceFilter.value) return 'resource_decision'
  if (kind === 'pool') return 'resource_decision'
  if (kind === 'support') return 'incident_support'
  if (kind === 'feedback') return 'feedback_triage'
  if (kind === 'token') return 'usage_intel'
  return 'general'
}

function actionForKind(kind: CommunityPostKind): CommunityActionType {
  if (hasSourceFilter.value || kind === 'pool') return 'share_signal'
  if (kind === 'support') return 'ask_help'
  if (kind === 'feedback') return 'report'
  return 'discuss'
}

function scenarioLabel(scenario?: string): string {
  switch (scenario) {
    case 'resource_decision': return '资源决策'
    case 'incident_support': return '故障协助'
    case 'feedback_triage': return '反馈治理'
    case 'demand_match': return '需求撮合'
    case 'capability_showcase': return '能力展示'
    case 'delivery_collaboration': return '交付协作'
    case 'usage_intel': return '用量情报'
    case 'announcement': return '公告'
    default: return ''
  }
}

function actionLabel(action?: string): string {
  switch (action) {
    case 'share_signal': return '信号'
    case 'ask_help': return '求助'
    case 'report': return '反馈'
    case 'recommend': return '推荐'
    case 'offer': return '供给'
    case 'request': return '需求'
    case 'deliver': return '交付'
    case 'review': return '评价'
    case 'announce': return '公告'
    default: return ''
  }
}

function subjectLabel(post: CommunityPost): string {
  if (post.subject_title) return post.subject_title
  if (post.subject_type === 'shared_pool' && post.subject_id) return `资源主体 #${post.subject_id}`
  if (post.source_type === 'shared_pool' && post.source_id) return `资源主体 #${post.source_id}`
  return ''
}

function communitySignalBadges(post: CommunityPost): string[] {
  return [isResidentProposal(post) ? '居民议题' : scenarioLabel(post.scenario), subjectLabel(post), actionLabel(post.action_type)].filter(Boolean)
}

function normalizedStatus(value?: string): string {
  return String(value || '').trim().toLowerCase()
}

function runtimeSignalValue(post: CommunityPost, keys: string[]): unknown {
  const signals = post.trust_signals || {}
  for (const key of keys) {
    if (Object.prototype.hasOwnProperty.call(signals, key)) return signals[key]
  }
  return undefined
}

function runtimeFlag(post: CommunityPost, keys: string[]): boolean {
  const value = runtimeSignalValue(post, keys)
  return value === true || value === 'true' || value === 1 || value === '1'
}

function runtimeNumber(post: CommunityPost, keys: string[]): number {
  const value = runtimeSignalValue(post, keys)
  const numeric = Number(value || 0)
  return Number.isFinite(numeric) ? numeric : 0
}

function hasPostRuntimeState(post: CommunityPost, states: string[]): boolean {
  const status = normalizedStatus(post.status)
  if (states.includes(status)) return true
  return (post.comments || []).some((comment) => states.includes(normalizedStatus(comment.status)))
}

function hasAcceptedAnswer(post: CommunityPost): boolean {
  return runtimeFlag(post, ['accepted', 'accepted_answer', 'has_accepted_answer']) ||
    hasPostRuntimeState(post, ['accepted', 'adopted'])
}

function hasOfficialConfirmation(post: CommunityPost): boolean {
  return runtimeFlag(post, ['official', 'official_confirmed', 'confirmed_by_official']) ||
    (post.comments || []).some((comment) => Boolean(comment.official) || ['official', 'confirmed'].includes(normalizedStatus(comment.status)))
}

function evidenceCount(post: CommunityPost): number {
  const signalCount = runtimeNumber(post, ['evidence_count', 'evidenceCount'])
  return Math.max(signalCount, Array.isArray(post.evidence) ? post.evidence.length : 0)
}

function postFreshnessScore(post: CommunityPost): number {
  const value = post.time.trim()
  if (/刚刚|本地/.test(value)) return 100
  const minuteMatch = value.match(/(\d+)\s*分钟/)
  if (minuteMatch) return Math.max(70, 100 - Number(minuteMatch[1]))
  const hourMatch = value.match(/(\d+)\s*小时/)
  if (hourMatch) return Math.max(24, 70 - Number(hourMatch[1]) * 3)
  if (/昨天/.test(value)) return 18
  const parsed = new Date(value).getTime()
  if (!Number.isNaN(parsed)) {
    const ageDays = Math.max(0, Math.floor((Date.now() - parsed) / 86_400_000))
    return Math.max(0, 18 - ageDays)
  }
  return 0
}

function communityPostRankScore(post: CommunityPost): number {
  let score = 0
  if (post.pinned || hasPostRuntimeState(post, ['featured']) || runtimeFlag(post, ['featured'])) score += 800
  if (hasAcceptedAnswer(post)) score += 650
  else if (hasPostRuntimeState(post, ['answered'])) score += 360
  if (hasOfficialConfirmation(post)) score += 560
  if (hasPostRuntimeState(post, ['resolved', 'closed']) || runtimeFlag(post, ['resolved'])) score += 420
  if (hasPostRuntimeState(post, ['confirmed'])) score += 240
  if (normalizedStatus(post.status) === 'reviewing') score += 90
  if (normalizedStatus(post.status) === 'blocked') score -= 80
  score += Math.min(evidenceCount(post), 8) * 90
  score += Math.min(runtimeNumber(post, ['reputation', 'contribution', 'contribution_score']), 120) * 3
  score += Math.min(runtimeNumber(post, ['responders', 'helpers', 'participants']), 20) * 12
  score += Math.min(post.replies || 0, 40) * 4
  score += Math.min(post.catches || 0, 60) * 2
  score += Math.min(post.views || 0, 600) / 20
  score += postFreshnessScore(post)
  return score
}

function rankCommunityPosts(items: CommunityPost[]): CommunityPost[] {
  return items
    .map((post, index) => ({ post, index, score: communityPostRankScore(post) }))
    .sort((left, right) => right.score - left.score || left.index - right.index)
    .map((item) => item.post)
}

function communityRuntimeBadges(post: CommunityPost): RuntimeBadge[] {
  const badges: RuntimeBadge[] = []
  if (post.pinned || hasPostRuntimeState(post, ['featured']) || runtimeFlag(post, ['featured'])) badges.push({ label: '精华', tone: 'gold' })
  if (hasAcceptedAnswer(post)) badges.push({ label: '已采纳', tone: 'good' })
  else if (hasPostRuntimeState(post, ['answered'])) badges.push({ label: '已回答', tone: 'good' })
  if (hasOfficialConfirmation(post)) badges.push({ label: '官方确认', tone: 'gold' })
  if (hasPostRuntimeState(post, ['resolved', 'closed']) || runtimeFlag(post, ['resolved'])) badges.push({ label: '已解决', tone: 'good' })
  if (hasPostRuntimeState(post, ['confirmed']) && !badges.some((badge) => badge.label === '官方确认')) badges.push({ label: '已确认', tone: 'neutral' })
  if (normalizedStatus(post.status) === 'reviewing') badges.push({ label: '复核中', tone: 'neutral' })
  if (normalizedStatus(post.status) === 'blocked') badges.push({ label: '待处理', tone: 'risk' })
  const evidenceTotal = evidenceCount(post)
  if (evidenceTotal > 0) badges.push({ label: `证据 ${evidenceTotal}`, tone: 'neutral' })
  return badges
}

function commentRuntimeBadges(comment: CommunityPostComment): RuntimeBadge[] {
  const status = normalizedStatus(comment.status)
  const badges: RuntimeBadge[] = []
  if (comment.official) badges.push({ label: '官方', tone: 'gold' })
  if (status === 'accepted' || status === 'adopted') badges.push({ label: '采纳', tone: 'good' })
  if (status === 'confirmed') badges.push({ label: '确认', tone: 'gold' })
  if (status === 'resolved') badges.push({ label: '解决', tone: 'good' })
  if (!badges.length && comment.helper_role) badges.push({ label: comment.helper_role, tone: 'neutral' })
  return badges
}

function communityRuntimeMeta(post: CommunityPost): string[] {
  const meta: string[] = []
  const reputation = runtimeNumber(post, ['reputation', 'contribution', 'contribution_score'])
  const responders = runtimeNumber(post, ['responders', 'helpers', 'participants'])
  if (reputation > 0) meta.push(`贡献声誉 +${reputation}`)
  if (responders > 0) meta.push(`${responders} 人接力`)
  if (runtimeFlag(post, ['saved_as_asset', 'asset_candidate'])) meta.push('可沉淀为能力资产')
  return meta
}

function isRemotePost(post: CommunityPost): post is CommunityPost & { id: number } {
  return Number.isFinite(post.id) && Number(post.id) > 0
}

function canOperateOwnPost(post: CommunityPost): boolean {
  return isRemotePost(post) && currentUserId.value > 0 && Number(post.user_id || 0) === currentUserId.value
}

function replaceRemotePost(post: import('@/features/bizdecipher/api/community').CommunityPost) {
  const nextPost = mapRemotePost(post)
  remotePosts.value = remotePosts.value.map((item) => item.id === nextPost.id ? nextPost : item)
}

function replaceRemoteComment(comment: import('@/features/bizdecipher/api/community').CommunityComment) {
  remotePosts.value = remotePosts.value.map((post) => {
    if (post.id !== comment.post_id) return post
    const comments = post.comments || []
    return {
      ...post,
      comments: comments.map((item) => item.id === comment.id ? comment : item)
    }
  })
}

async function runPostAction(post: CommunityPost, action: () => Promise<import('@/features/bizdecipher/api/community').CommunityPost>, successMessage: string) {
  if (!isRemotePost(post) || operatingPostId.value) return
  operatingPostId.value = post.id
  try {
    const updated = await action()
    replaceRemotePost(updated)
    appStore.showSuccess(successMessage)
  } catch {
    appStore.showWarning('操作没有完成，请确认登录态或权限后再试。')
  } finally {
    operatingPostId.value = null
  }
}

async function runCommentAction(comment: CommunityPostComment, action: () => Promise<import('@/features/bizdecipher/api/community').CommunityComment>, successMessage: string) {
  if (!comment.id || operatingCommentId.value) return
  operatingCommentId.value = comment.id
  try {
    const updated = await action()
    replaceRemoteComment(updated)
    appStore.showSuccess(successMessage)
  } catch {
    appStore.showWarning('评论操作没有完成，请确认登录态或权限后再试。')
  } finally {
    operatingCommentId.value = null
  }
}

function postActionButtons(post: CommunityPost): RuntimeActionButton[] {
  if (!isRemotePost(post)) return []
  const buttons: RuntimeActionButton[] = []
  const status = normalizedStatus(post.status)
  if (canOperateOwnPost(post)) {
    if (status !== 'resolved') {
      buttons.push({ key: 'owner-resolve', label: '标记已解决', handler: () => runPostAction(post, () => communityAPI.updatePostAction(post.id, { action: 'mark_resolved' }), '已标记为解决。') })
    } else {
      buttons.push({ key: 'owner-reopen', label: '重新打开', handler: () => runPostAction(post, () => communityAPI.updatePostAction(post.id, { action: 'reopen' }), '已重新打开。') })
    }
  }
  if (canModerateCommunity.value) {
    if (status !== 'confirmed') {
      buttons.push({ key: 'admin-confirm', label: '官方确认', handler: () => runPostAction(post, () => communityAPI.adminUpdatePostStatus(post.id, { status: 'confirmed' }), '已做官方确认。') })
    }
    buttons.push({
      key: post.pinned ? 'admin-unpin' : 'admin-pin',
      label: post.pinned ? '取消精华' : '设为精华',
      handler: () => runPostAction(post, () => communityAPI.adminUpdatePostStatus(post.id, { status: status || 'open', pinned: !post.pinned }), post.pinned ? '已取消精华。' : '已设为精华。')
    })
  }
  return buttons
}

function commentActionButtons(post: CommunityPost, comment: CommunityPostComment): RuntimeActionButton[] {
  if (!isRemotePost(post) || !comment.id) return []
  const buttons: RuntimeActionButton[] = []
  const status = normalizedStatus(comment.status)
  if (canOperateOwnPost(post) && status !== 'accepted') {
    buttons.push({ key: 'owner-accept', label: '采纳', handler: () => runPostAction(post, () => communityAPI.updatePostAction(post.id, { action: 'accept_comment', comment_id: comment.id }), '已采纳该回复。') })
  }
  if (canModerateCommunity.value) {
    if (!comment.official || status !== 'confirmed') {
      buttons.push({ key: 'admin-confirm-comment', label: '官方确认', handler: () => runCommentAction(comment, () => communityAPI.adminUpdateCommentStatus(comment.id, { status: 'confirmed', official: true }), '已确认该回复。') })
    }
  }
  return buttons
}

async function handleComposerSubmit() {
  if (isSubmittingPost.value || !composerAllowed.value) return
  const title = composerTitle.value.trim()
  const body = composerBody.value.trim()
  if (!title || !body) {
    appStore.showWarning(t('community.composer.validation'))
    return
  }

  const channel = activeChannelMeta.value
  const kind = resolvePostKind()
  const resolvedKind = hasSourceFilter.value ? 'pool' : channel?.kind || kind
  const scenario = channel?.scenario || scenarioForKind(resolvedKind)
  const actionType = channel?.actionType || actionForKind(resolvedKind)
  const districtValue = isCityHome.value || isPersonalWorkspace.value ? undefined : activeCommunityDistrict.value
  const channelValue = isCityHome.value || isPersonalWorkspace.value ? undefined : activeCommunityChannel.value
  const tags = [channel?.label, selectedQuickTag.value].filter((tag): tag is string => Boolean(tag))
  isSubmittingPost.value = true
  try {
    const created = await communityAPI.createPost({
      kind: resolvedKind,
      title,
      body,
      tags,
      district: districtValue,
      channel: channelValue,
      private: false,
      source_type: hasSourceFilter.value ? activeSourceType.value : undefined,
      source_id: hasSourceFilter.value ? activeSourceId.value : undefined,
      scenario,
      subject_type: hasSourceFilter.value ? activeSourceType.value : undefined,
      subject_id: hasSourceFilter.value ? activeSourceId.value : undefined,
      action_type: actionType,
      evidence: [],
      trust_signals: {}
    })
    const mapped = mapRemotePost(created)
    remotePosts.value = [mapped, ...remotePosts.value].slice(0, 80)
    myPosts.value = [mapped, ...myPosts.value].filter((post, index, items) =>
      items.findIndex((item) => item.id === post.id) === index,
    )
    appStore.showSuccess(t('community.composer.saved'))
  } catch (cause) {
    appStore.showError(extractActionableApiErrorMessage(cause, '发布没有提交成功，请稍后重试。'))
    void refreshParticipation()
    return
  } finally {
    isSubmittingPost.value = false
  }

  composerTitle.value = ''
  composerBody.value = ''
  composerOpen.value = false
  document.querySelector(isCityHome.value ? '.zero-city-editorial-post, .feed-row' : '.zero-city-workspace-feed .feed-row')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

function selectedLabel<T extends string>(items: Array<{ key: T; label: string }>, key: T): string {
  return items.find((item) => item.key === key)?.label || key
}

function feedbackStatusLabel(item: FeedbackItem): string {
  if (item.officialAnswered) return t('community.feedback.status.official')
  if (item.acceptedAnswer) return t('community.feedback.status.accepted')
  const status = normalizedStatus(item.status)
  if (status === 'resolved' || status === 'closed') return t('community.feedback.status.resolved')
  if (status === 'confirmed') return t('community.feedback.status.confirmed')
  if (status === 'reviewing') return t('community.feedback.status.reviewing')
  if (item.postId) return t('community.feedback.status.synced')
  return t('community.feedback.status.open')
}

function feedbackStatusTone(item: FeedbackItem): RuntimeBadgeTone {
  if (item.officialAnswered || item.acceptedAnswer) return 'gold'
  const status = normalizedStatus(item.status)
  if (status === 'resolved' || status === 'closed' || status === 'confirmed') return 'good'
  if (status === 'reviewing' || item.postId) return 'neutral'
  return item.private ? 'risk' : 'neutral'
}

function feedbackVisibilityLabel(item: FeedbackItem): string {
  return item.private ? t('community.feedback.privateRouting') : t('community.feedback.publicRouting')
}

function feedbackEvidenceLabel(item: FeedbackItem): string {
  const count = Number(item.evidenceCount || 0)
  return count > 0 ? t('community.feedback.evidenceCount', { count }) : t('community.feedback.evidenceMissing')
}

function feedbackNextStepLabel(item: FeedbackItem): string {
  if (item.private) return t('community.feedback.nextStepPrivate')
  if (item.officialAnswered) return t('community.feedback.nextStepOfficial')
  if (item.acceptedAnswer || normalizedStatus(item.status) === 'resolved') return t('community.feedback.nextStepResolved')
  return t('community.feedback.nextStepPublic')
}

function openFeedbackHandling(item: FeedbackItem) {
  activePostFilter.value = 'feedback'
  if (item.postId) {
    router.replace({ path: '/community', query: { kind: 'feedback', post_id: String(item.postId) } })
  }
  document.querySelector('.feed-row')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function prefillFeedbackReply(item: FeedbackItem) {
  activePostFilter.value = 'feedback'
  composerTitle.value = `反馈跟进：${item.title}`
  composerBody.value = `关联反馈：${item.title}\n\n补充信息：`
  jumpToComposer()
}

async function handleFeedbackSubmit() {
  const title = feedbackTitle.value.trim()
  const body = feedbackBody.value.trim()
  if (!title || !body) {
    appStore.showWarning(t('community.feedback.validation'))
    return
  }

  const category = selectedLabel(feedbackCategories.value, selectedFeedbackCategory.value)
  const severity = selectedLabel(feedbackSeverities.value, selectedFeedbackSeverity.value)
  try {
    const created = await communityAPI.createPost({
      kind: 'feedback',
      title,
      body,
      tags: [category, severity].filter(Boolean),
      private: feedbackPrivate.value,
      scenario: 'feedback_triage',
      action_type: 'report',
      evidence: [],
      trust_signals: { severity, category }
    })
    if (!created.private) {
      remotePosts.value = [mapRemotePost(created), ...remotePosts.value].slice(0, 80)
    }
  } catch {
    appStore.showError('反馈没有提交成功，请确认登录态或稍后重试。')
    return
  }

  feedbackTitle.value = ''
  feedbackBody.value = ''
  activePostFilter.value = 'feedback'
  appStore.showSuccess(t('community.feedback.saved'))
}

async function submitComment(post: CommunityPost) {
  if (!post.id) {
    appStore.showWarning('这条记录还没有同步到社区服务，暂时不能回复。')
    return
  }
  const body = (commentDrafts.value[post.id] || '').trim()
  if (!body) {
    appStore.showWarning('先写一句回复，居民才接得住。')
    return
  }
  submittingCommentId.value = post.id
  try {
    const comment = await communityAPI.createComment(post.id, { body, helper_role: 'resident' })
    post.comments = [...(post.comments || []), comment]
    post.replies += 1
    commentDrafts.value = { ...commentDrafts.value, [post.id]: '' }
    appStore.showSuccess('已回复到居民广场。')
  } catch (cause) {
    appStore.showWarning(extractActionableApiErrorMessage(cause, '回复没有提交成功，请稍后重试。'))
  } finally {
    submittingCommentId.value = null
  }
}

function tokenPowerRows() {
  return tokenPower.value?.rows || []
}

function formatMoney(value: number): string {
  return `$${Number(value || 0).toFixed(2)}`
}

function handleVolunteer(roleTitle: string) {
  activePostFilter.value = 'support'
  composerTitle.value = `申请协作角色：${roleTitle}`
  composerBody.value = `我想以「${roleTitle}」参与社群协作，能提供的帮助是：\n\n`
  jumpToComposer()
}
</script>

<style scoped>
.zero-city-page {
  --zc-city-gap: var(--bd-space-4);
  --zc-city-event: var(--bd-accent-teal);
  --zc-city-level: color-mix(in srgb, var(--zc-accent) 10%, var(--zc-card));
  --zc-city-night: var(--bd-brand-ink);
  --zc-city-title: 1.5rem;
  --zc-city-section: 1.125rem;
  --zc-city-line-heading: 1.2;
  --zc-city-body: 0.875rem;
  --zc-city-line-body: 1.6;
}

.zero-city-task-board {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--bd-space-2);
  margin-top: var(--bd-space-4);
  padding-block: var(--bd-space-3);
  border-bottom: var(--bd-line-width) solid var(--zc-line);
}

.zero-city-task-board--collaboration { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.zero-city-task-board button,
.zero-city-profile-actions button {
  display: grid;
  min-width: 0;
  gap: var(--bd-space-1);
  padding: var(--bd-space-3);
  border: var(--bd-line-width) solid var(--zc-line);
  border-radius: var(--bd-radius-md);
  background: transparent;
  color: var(--zc-text);
  text-align: left;
  transition: transform var(--bd-motion-micro) ease-out, border-color var(--bd-motion-micro) ease-out;
}

.zero-city-task-board button:hover,
.zero-city-task-board button:focus-visible,
.zero-city-task-board button.active,
.zero-city-profile-actions button:hover,
.zero-city-profile-actions button:focus-visible {
  border-color: var(--zc-accent);
  transform: translateY(var(--bd-motion-lift));
}

.zero-city-task-board button:focus-visible,
.zero-city-profile-actions button:focus-visible { outline: var(--bd-focus-ring-width) solid var(--zc-accent); outline-offset: var(--bd-motion-shift); }
.zero-city-task-board span { color: var(--zc-accent); font-size: var(--bd-type-overline); font-weight: var(--bd-weight-emphasis); }
.zero-city-task-board strong,
.zero-city-profile-actions strong { color: var(--zc-text-strong); font-size: var(--bd-type-caption); }
.zero-city-task-board small,
.zero-city-profile-actions small { color: var(--zc-muted); font-size: var(--bd-type-overline); line-height: var(--bd-line-caption); }
.zero-city-profile-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--bd-space-3); }
.feed-row--workshop { border-left: 3px solid var(--bd-accent-blue); }
.feed-row--collaboration { border-left: 3px solid var(--bd-accent-gold); }
.feed-row--activity { border-left: 3px solid var(--zc-accent-2); }
.feed-row--plaza { border-left: 3px solid var(--bd-accent-teal); }

@media (max-width: 820px) {
  .zero-city-task-board,
  .zero-city-task-board--collaboration { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 540px) {
  .zero-city-task-board,
  .zero-city-task-board--collaboration,
  .zero-city-profile-actions { grid-template-columns: minmax(0, 1fr); }
}

@media (prefers-reduced-motion: reduce) {
  .zero-city-task-board button,
  .zero-city-profile-actions button { transition: none; }
  .zero-city-task-board button:hover,
  .zero-city-profile-actions button:hover { transform: none; }
}

.city-story-entry { order: 3; display: flex; align-items: center; gap: 16px; padding: 12px 0; border-bottom: 1px solid var(--bd-ui-line); }
.city-story-entry img { width: 72px; height: 80px; object-fit: contain; flex-shrink: 0; }
.city-story-entry div { flex: 1; min-width: 0; }
.city-story-entry span, .city-story-entry p { color: var(--bd-text-secondary); font-size: 12px; line-height: 1.7; }
.city-story-entry strong { display: block; color: var(--bd-text-primary); font-size: 16px; margin: 4px 0; }
.zero-city-page .zero-city-forum-title h1 { font-size: 28px; line-height: 1.4; }
.zero-city-page .zero-city-city-pulse { grid-template-columns: minmax(260px, 360px) 1fr; background: transparent; box-shadow: none; }
@media (max-width: 767px) {
  .zero-city-page .zero-city-city-pulse { grid-template-columns: minmax(0, 1fr); }
  .city-story-entry { flex-wrap: wrap; }
  .city-story-entry button { margin-left: 88px; }
}
.zero-city-composer-overlay {
  position: fixed;
  inset: 0;
  z-index: 240;
  display: flex;
  justify-content: flex-end;
  background: color-mix(in srgb, #10202c 34%, transparent);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.zero-city-composer-drawer {
  position: fixed;
  z-index: 241;
  top: 0;
  right: 0;
  bottom: 0;
  display: flex;
  width: min(42rem, calc(100vw - 1.5rem));
  height: 100%;
  flex-direction: column;
  gap: 1.25rem;
  overflow-y: auto;
  padding: 1.5rem;
  background: var(--zc-bg);
  color: var(--zc-text);
  border-left: 1px solid var(--zc-line);
  box-shadow: -8px 0 20px color-mix(in srgb, var(--zc-shadow-dark) 55%, transparent);
}

.zero-city-composer-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.zero-city-composer-head h2 {
  margin-top: 0.35rem;
  color: var(--zc-text-strong);
  font-size: clamp(1.45rem, 3vw, 2rem);
  font-weight: 950;
}

.zero-city-composer-head p:last-child {
  margin-top: 0.4rem;
  max-width: 34rem;
  color: var(--zc-muted);
  font-size: 0.875rem;
  line-height: 1.65;
}

.zero-city-composer-close {
  display: inline-flex;
  width: 2.75rem;
  height: 2.75rem;
  flex: 0 0 2.75rem;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 999px;
  background: var(--zc-bg);
  color: var(--zc-text-strong);
  font-size: 1.5rem;
  box-shadow: 5px 5px 12px var(--zc-shadow-dark), -5px -5px 12px var(--zc-shadow-light);
}

.zero-city-composer-context,
.zero-city-composer-routes {
  border: 1px solid var(--zc-line);
  border-radius: var(--bd-radius-md);
  background: var(--zc-bg);
  padding: 1rem;
}

.zero-city-composer-context {
  display: grid;
  gap: 0.25rem;
}

.zero-city-composer-context span,
.zero-city-composer-context small {
  color: var(--zc-muted);
  font-size: 0.75rem;
  font-weight: 750;
}

.zero-city-composer-context strong {
  color: var(--zc-accent);
  font-size: 1rem;
  font-weight: 950;
}

.participation-notice {
  display: grid;
  gap: 8px;
  padding: 12px 0;
  border-block: 1px solid var(--bd-ui-line);
  font-size: 13px;
  line-height: 1.6;
  color: var(--bd-text-secondary);
}
.participation-notice button {
  justify-self: start;
  color: var(--bd-accent-teal);
  text-decoration: underline;
  text-underline-offset: 3px;
}
.zero-city-composer-form {
  display: grid;
  gap: 0.9rem;
}

.zero-city-composer-routes {
  display: grid;
  gap: 1rem;
}

.zero-city-composer-route-grid {
  display: grid;
  gap: 0.75rem;
}

.zero-city-composer-route-grid button {
  display: grid;
  gap: 0.3rem;
  border: 1px solid var(--zc-line);
  border-radius: var(--bd-radius-md);
  background: var(--zc-bg);
  padding: 0.95rem;
  color: var(--zc-text);
  text-align: left;
}

.zero-city-composer-route-grid button:hover {
  border-color: color-mix(in srgb, var(--zc-accent) 45%, var(--zc-line));
  color: var(--zc-accent);
}

.zero-city-composer-route-grid strong {
  font-size: 0.875rem;
  font-weight: 950;
}

.zero-city-composer-route-grid small {
  color: var(--zc-muted);
  font-size: 0.75rem;
  line-height: 1.5;
}

.composer-drawer-enter-active,
.composer-drawer-leave-active {
  transition: opacity 0.2s ease;
}

.composer-drawer-enter-active .zero-city-composer-drawer,
.composer-drawer-leave-active .zero-city-composer-drawer {
  transition: transform 0.25s ease;
}

.composer-drawer-enter-from,
.composer-drawer-leave-to {
  opacity: 0;
}

.composer-drawer-enter-from .zero-city-composer-drawer,
.composer-drawer-leave-to .zero-city-composer-drawer {
  transform: translateX(100%);
}

.zero-city-forum-home {
  display: grid;
  gap: 0.85rem;
  min-width: 0;
}

.zero-city-forum-header { order: 1; }
.zero-city-home-pulse { order: 2; }
.zero-city-city-pulse { order: 2; }
.zero-city-forum-search { order: 3; }
.zero-city-forum-layout { order: 4; }
.zero-city-forum-districts { order: 5; }
.zero-city-forum-layers { order: 6; }

.zero-city-forum-header,
.zero-city-forum-search,
.zero-city-forum-districts,
.zero-city-city-pulse,
.forum-feed-panel,
.forum-side-panel {
  border: 1px solid color-mix(in srgb, var(--zc-line) 86%, transparent);
  background: var(--zc-card);
  box-shadow: var(--zc-shadow-soft);
}

.zero-city-forum-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1.5rem;
  padding: 1.1rem 1.5rem;
  border-radius: 1.15rem;
  background:
    radial-gradient(circle at 88% 15%, color-mix(in srgb, var(--zc-accent) 12%, transparent), transparent 28%),
    linear-gradient(135deg, color-mix(in srgb, var(--zc-card) 88%, var(--zc-bg-2)), var(--zc-card));
}

.zero-city-forum-header {
  border: 0;
  padding-inline: 0;
  background: transparent;
  box-shadow: none;
}

.zero-city-city-pulse {
  border-right: 0;
  border-left: 0;
  border-radius: 0;
  box-shadow: none;
}

.zero-city-forum-districts,
.forum-feed-panel,
.forum-side-panel {
  border-right: 0;
  border-left: 0;
  border-radius: 0;
  box-shadow: none;
}

.zero-city-forum-districts {
  background: transparent;
}

.forum-feed-panel,
.forum-side-panel {
  background: color-mix(in srgb, var(--zc-card) 58%, transparent);
}

.zero-city-forum-title {
  min-width: 0;
  max-width: 48rem;
}

.forum-overline,
.forum-thread-context,
.forum-thread-byline,
.forum-visible-count,
.forum-feed-heading-row,
.forum-side-heading,
.forum-side-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
}

.forum-overline {
  gap: 0.55rem;
  color: var(--zc-accent);
  font-size: 0.7rem;
  font-weight: 950;
  letter-spacing: 0.07em;
}

.forum-live-dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 50%;
  background: var(--zc-accent);
  box-shadow: 0 0 0 0.25rem color-mix(in srgb, var(--zc-accent) 16%, transparent);
}

.forum-open-label,
.forum-gate-lock,
.forum-card-mark {
  padding: 0.24rem 0.48rem;
  border: 1px solid color-mix(in srgb, var(--zc-accent) 32%, var(--zc-line));
  border-radius: 0.4rem;
  color: var(--zc-accent);
  font-size: 0.64rem;
  letter-spacing: 0;
}

.zero-city-forum-title h1 {
  margin-top: 0.45rem;
  color: var(--zc-text-strong);
  font-size: clamp(1.8rem, 3.7vw, 2.8rem);
  font-weight: 950;
  line-height: 1.08;
  letter-spacing: -0.045em;
  text-wrap: balance;
}

.zero-city-forum-title p {
  max-width: 44rem;
  margin-top: 0.55rem;
  color: var(--zc-muted);
  font-size: 0.86rem;
  line-height: 1.7;
}

.zero-city-forum-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.55rem;
}

.zero-city-city-pulse {
  display: grid;
  grid-template-columns: minmax(20rem, 0.95fr) minmax(0, 1.05fr);
  align-items: start;
  gap: 0.9rem;
  padding: 0.85rem 1rem;
  border-radius: var(--bd-radius-md);
  background:
    radial-gradient(circle at 50% 20%, color-mix(in srgb, var(--bd-accent-gold) 12%, transparent), transparent 36%),
    radial-gradient(circle at 8% 0%, color-mix(in srgb, var(--zc-accent) 12%, transparent), transparent 34%),
    var(--zc-card);
}

.zero-city-city-pulse :deep(.zero-city-rankings-panel--top) {
  grid-column: 1;
  grid-row: 1 / span 2;
  overflow: visible;
  padding: var(--bd-space-1) var(--bd-space-4) var(--bd-space-1) 0;
  border-top: 0;
  border-right: var(--bd-line-width) solid color-mix(in srgb, var(--zc-line) 90%, transparent);
  border-left: 0;
  border-radius: 0;
  align-self: start;
}

.city-pulse-lead {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 0.22rem;
  padding: 0.3rem 0.3rem 0.3rem 0.45rem;
  grid-column: 2;
  grid-row: 1;
  align-self: center;
}

.city-pulse-kicker {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--zc-accent);
  font-size: 0.68rem;
  font-weight: 950;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.city-pulse-kicker .forum-live-dot {
  width: 0.38rem;
  height: 0.38rem;
  box-shadow: 0 0 0 0.2rem color-mix(in srgb, var(--zc-accent) 13%, transparent);
}

.city-pulse-lead > strong {
  max-width: 26rem;
  color: var(--zc-text-strong);
  font-size: 0.95rem;
  font-weight: 950;
  line-height: 1.35;
  text-wrap: balance;
}

.city-pulse-lead > span:last-child {
  max-width: 30rem;
  color: var(--zc-muted);
  font-size: 0.72rem;
  line-height: 1.6;
}

.city-pulse-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 0.48rem;
  grid-column: 2;
  grid-row: 2;
  justify-content: flex-start;
  align-self: start;
}

.forum-visible-count {
  min-height: 2.35rem;
  padding: 0 0.7rem;
  border: 1px solid var(--zc-line);
  border-radius: 0.7rem;
  color: var(--zc-muted);
  font-size: 0.7rem;
  font-weight: 900;
}

.zero-city-forum-search {
  display: flex;
  min-height: 3rem;
  align-items: center;
  gap: 0.7rem;
  padding: 0.55rem 0.85rem;
  border-radius: 0.8rem;
  box-shadow: var(--zc-shadow-inset);
}

.zero-city-forum-search svg {
  width: 1.05rem;
  flex: 0 0 auto;
  color: var(--zc-muted);
}

.zero-city-forum-search input {
  min-width: 0;
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--zc-text-strong);
  font-size: 0.84rem;
  font-weight: 700;
}

.zero-city-forum-search input::placeholder {
  color: var(--zc-muted);
}

.zero-city-forum-search > span {
  flex: 0 0 auto;
  color: var(--zc-accent);
  font-size: 0.7rem;
  font-weight: 900;
}

.forum-search-clear {
  display: grid;
  width: 1.55rem;
  height: 1.55rem;
  place-items: center;
  border-radius: 0.45rem;
  color: var(--zc-muted);
  font-size: 1rem;
  line-height: 1;
}

.forum-search-clear:hover,
.forum-search-clear:focus-visible {
  background: color-mix(in srgb, var(--zc-accent) 10%, transparent);
  color: var(--zc-text-strong);
}

.zero-city-forum-districts {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  overflow: hidden;
  border-radius: 0.95rem;
}

.forum-district-link {
  display: grid;
  min-width: 0;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.6rem;
  padding: 0.72rem 0.78rem;
  border-right: 1px solid var(--zc-line);
  color: var(--zc-muted);
  text-align: left;
  transition: background-color 160ms ease, color 160ms ease, transform 160ms ease;
}

.forum-district-link:last-child {
  border-right: 0;
}

.forum-district-link:hover,
.forum-district-link:focus-visible {
  background: color-mix(in srgb, var(--zc-accent) 7%, var(--zc-card));
  color: var(--zc-text-strong);
}

.forum-district-link.active {
  background: color-mix(in srgb, var(--zc-accent) 10%, var(--zc-card));
  color: var(--zc-text-strong);
}

.forum-district-mark {
  display: grid;
  width: 1.7rem;
  height: 1.7rem;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--zc-accent) 28%, var(--zc-line));
  border-radius: 0.45rem;
  color: var(--zc-accent);
  font-size: 0.74rem;
  font-weight: 950;
}

.forum-district-copy {
  display: grid;
  min-width: 0;
  gap: 0.15rem;
}

.forum-district-copy strong {
  overflow: hidden;
  color: inherit;
  font-size: 0.78rem;
  font-weight: 950;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.forum-district-copy small {
  overflow: hidden;
  color: var(--zc-muted);
  font-size: 0.65rem;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.forum-district-arrow {
  color: var(--zc-muted);
  font-size: 0.85rem;
  transition: transform 160ms ease, color 160ms ease;
}

.forum-district-link:hover .forum-district-arrow,
.forum-district-link:focus-visible .forum-district-arrow {
  transform: translateX(0.15rem);
  color: var(--zc-accent);
}

.zero-city-forum-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(18rem, 21rem);
  align-items: start;
  gap: 0.85rem;
}

.zero-city-forum-layers {
  display: grid;
  gap: 0.7rem;
  margin: 0.85rem 0;
  padding: 0.9rem 1rem;
  border: 1px solid var(--zc-line);
  border-radius: var(--bd-radius-md);
  background: color-mix(in srgb, var(--zc-card) 88%, transparent);
}

.forum-layer-intro {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
}

.forum-layer-intro h2 {
  margin-top: 0.22rem;
  color: var(--zc-text-strong);
  font-size: 0.95rem;
  font-weight: 950;
}

.forum-layer-intro > p {
  max-width: 31rem;
  color: var(--zc-muted);
  font-size: 0.68rem;
  line-height: 1.5;
  text-align: right;
}

.forum-layer-tabs {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.45rem;
}

.forum-layer-tabs button {
  display: grid;
  grid-template-columns: 1.65rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
  padding: 0.55rem 0.6rem;
  border: 0;
  border-bottom: 2px solid transparent;
  border-radius: 0;
  color: var(--zc-muted);
  background: transparent;
  text-align: left;
  transition: border-color 160ms ease, color 160ms ease;
}

.forum-layer-tabs button:hover,
.forum-layer-tabs button:focus-visible,
.forum-layer-tabs button.active {
  border-bottom-color: color-mix(in srgb, var(--zc-accent) 72%, var(--zc-line));
  color: var(--zc-text-strong);
}

.forum-layer-mark {
  display: grid;
  width: 1.65rem;
  height: 1.65rem;
  place-items: center;
  border-radius: 0.45rem;
  background: color-mix(in srgb, var(--zc-accent) 12%, transparent);
  color: var(--zc-accent);
  font-size: 0.7rem;
  font-weight: 950;
}

.forum-layer-copy {
  display: grid;
  min-width: 0;
  gap: 0.12rem;
}

.forum-layer-copy strong {
  overflow: hidden;
  color: inherit;
  font-size: 0.71rem;
  font-weight: 950;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.forum-layer-copy small {
  overflow: hidden;
  color: var(--zc-muted);
  font-size: 0.6rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.forum-layer-count {
  min-width: 1.35rem;
  padding: 0.15rem 0.28rem;
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--zc-line) 55%, transparent);
  color: var(--zc-muted);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.62rem;
  font-weight: 850;
  text-align: center;
}

.forum-layer-context {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem 0.7rem;
  padding-top: 0.55rem;
  border-top: 1px solid var(--zc-line);
  color: var(--zc-muted);
  font-size: 0.66rem;
  line-height: 1.5;
}

.forum-layer-context strong {
  color: var(--zc-text-strong);
  font-size: 0.68rem;
}

.forum-layer-access {
  padding: 0.2rem 0.4rem;
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--zc-accent) 9%, transparent);
  color: var(--zc-accent);
  font-size: 0.6rem;
  font-weight: 850;
}

.zero-city-forum-feed,
.zero-city-forum-aside {
  min-width: 0;
}

.zero-city-forum-aside {
  display: grid;
  gap: 0.85rem;
}

.forum-feed-panel,
.forum-side-panel {
  overflow: hidden;
  border-radius: var(--bd-radius-md);
}

.forum-feed-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.15rem 0.85rem;
  border-bottom: 1px solid var(--zc-line);
}

.forum-feed-heading-row {
  gap: 0.45rem;
  margin-top: 0.18rem;
}

.forum-feed-heading-row h2,
.forum-side-heading h2 {
  color: var(--zc-text-strong);
  font-size: 1.02rem;
  font-weight: 950;
  line-height: 1.3;
}

.forum-feed-count {
  display: inline-grid;
  min-width: 1.35rem;
  height: 1.35rem;
  place-items: center;
  padding: 0 0.3rem;
  border-radius: 0.4rem;
  background: color-mix(in srgb, var(--zc-accent) 12%, transparent);
  color: var(--zc-accent);
  font-size: 0.66rem;
  font-weight: 950;
}

.forum-feed-subtitle {
  margin-top: 0.25rem;
  color: var(--zc-muted);
  font-size: 0.68rem;
  line-height: 1.5;
}

.forum-rules-toggle,
.forum-side-heading > button,
.forum-side-panel > button,
.forum-side-actions button {
  color: var(--zc-accent);
  font-size: 0.69rem;
  font-weight: 950;
}

.forum-rules-toggle {
  padding: 0.4rem 0.55rem;
  border: 1px solid var(--zc-line);
  border-radius: 0.55rem;
}

.forum-rules-toggle:hover,
.forum-rules-toggle:focus-visible,
.forum-side-heading > button:hover,
.forum-side-heading > button:focus-visible,
.forum-side-panel > button:hover,
.forum-side-panel > button:focus-visible,
.forum-side-actions button:hover,
.forum-side-actions button:focus-visible {
  background: color-mix(in srgb, var(--zc-accent) 9%, transparent);
}

.forum-mode-tabs {
  display: flex;
  overflow-x: auto;
  gap: 0.1rem;
  padding: 0 0.65rem;
  border-bottom: 1px solid var(--zc-line);
  scrollbar-width: thin;
}

.forum-mode-tabs button {
  position: relative;
  flex: 0 0 auto;
  min-height: 2.45rem;
  padding: 0.55rem 0.6rem;
  color: var(--zc-muted);
  font-size: 0.72rem;
  font-weight: 900;
  white-space: nowrap;
}

.forum-mode-tabs button::after {
  position: absolute;
  right: 0.6rem;
  bottom: -1px;
  left: 0.6rem;
  height: 2px;
  background: transparent;
  content: '';
}

.forum-mode-tabs button:hover,
.forum-mode-tabs button:focus-visible {
  color: var(--zc-text-strong);
}

.forum-mode-tabs button.active {
  color: var(--zc-accent);
}

.forum-mode-tabs button.active::after {
  background: var(--zc-accent);
}

.forum-rules-note {
  display: grid;
  gap: 0.25rem;
  padding: 0.72rem 1.15rem;
  border-bottom: 1px solid var(--zc-line);
  background: color-mix(in srgb, var(--zc-accent) 6%, var(--zc-card));
}

.forum-rules-note strong {
  color: var(--zc-text-strong);
  font-size: 0.72rem;
}

.forum-rules-note span {
  color: var(--zc-muted);
  font-size: 0.68rem;
  line-height: 1.55;
}

.forum-thread-list {
  display: grid;
}

.forum-thread-row {
  display: grid;
  grid-template-columns: 2rem minmax(0, 1fr) 7rem 8.8rem;
  align-items: start;
  gap: 0.9rem;
  padding: 0.92rem 1.15rem;
  border-bottom: 1px solid var(--zc-line);
  transition: background-color 160ms ease;
}

.forum-thread-row:last-child {
  border-bottom: 0;
}

.forum-thread-row:hover {
  background: color-mix(in srgb, var(--zc-accent) 4%, var(--zc-card));
}

.forum-thread-mark {
  display: grid;
  justify-items: center;
  gap: 0.5rem;
  padding-top: 0.2rem;
}

.forum-thread-mark > span {
  display: grid;
  width: 1.65rem;
  height: 1.65rem;
  place-items: center;
  border-radius: 0.45rem;
  background: color-mix(in srgb, var(--zc-accent) 10%, transparent);
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 950;
}

.thread-dot {
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 50%;
  background: var(--zc-muted);
}

.thread-dot-good { background: var(--zc-accent); }
.thread-dot-gold { background: var(--zc-accent-2); }
.thread-dot-risk { background: var(--bd-status-danger, var(--zc-accent-2)); }

.forum-thread-main {
  min-width: 0;
}

.forum-thread-topline,
.forum-thread-context,
.forum-thread-byline {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
}

.forum-thread-topline {
  justify-content: space-between;
  gap: 0.6rem;
}

.forum-thread-context {
  min-width: 0;
  gap: 0.45rem;
  color: var(--zc-muted);
  font-size: 0.66rem;
  font-weight: 750;
}

.forum-thread-section {
  color: var(--zc-accent);
  font-weight: 950;
}

.forum-thread-status {
  padding: 0.18rem 0.35rem;
  border-radius: 0.3rem;
  background: color-mix(in srgb, var(--zc-accent) 9%, transparent);
  color: var(--zc-accent);
  font-size: 0.61rem;
  font-weight: 950;
}

.forum-thread-topline time {
  flex: 0 0 auto;
  color: var(--zc-muted);
  font-size: 0.65rem;
  font-weight: 750;
}

.forum-thread-main h3 {
  margin-top: 0.34rem;
  color: var(--zc-text-strong);
  font-size: 0.92rem;
  font-weight: 950;
  line-height: 1.45;
  text-wrap: balance;
}

.forum-thread-title {
  display: flex;
  width: 100%;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.7rem;
  margin-top: 0.34rem;
  color: var(--zc-text-strong);
  text-align: left;
}

.forum-thread-title > span:first-child {
  min-width: 0;
  font-size: 0.92rem;
  font-weight: 950;
  line-height: 1.45;
  text-wrap: balance;
}

.forum-thread-title:hover > span:first-child,
.forum-thread-title:focus-visible > span:first-child {
  color: var(--zc-accent);
}

.forum-thread-open-hint {
  flex: 0 0 auto;
  color: var(--zc-accent);
  font-size: 0.62rem;
  font-weight: 900;
  white-space: nowrap;
}

.forum-thread-main > p {
  display: -webkit-box;
  overflow: hidden;
  margin-top: 0.25rem;
  color: var(--zc-muted);
  font-size: 0.76rem;
  line-height: 1.58;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.forum-thread-signals {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  margin-top: 0.42rem;
}

.forum-thread-signals span {
  padding: 0.18rem 0.34rem;
  border: 1px solid color-mix(in srgb, var(--zc-line) 92%, transparent);
  border-radius: 0.3rem;
  color: var(--zc-muted);
  font-size: 0.6rem;
  font-weight: 850;
}

.forum-thread-byline {
  gap: 0.45rem;
  margin-top: 0.5rem;
  color: var(--zc-muted);
  font-size: 0.64rem;
  font-weight: 750;
}

.forum-author-mark {
  color: var(--zc-text-strong);
  font-weight: 950;
}

.forum-thread-byline button {
  color: var(--zc-accent);
  font-weight: 900;
}

.forum-thread-detail {
  display: grid;
  gap: 0.55rem;
  margin-top: 0.65rem;
  padding: 0.7rem;
  border-left: 2px solid color-mix(in srgb, var(--zc-accent) 45%, transparent);
  background: color-mix(in srgb, var(--zc-bg) 34%, var(--zc-card));
}

.forum-thread-detail > p {
  color: var(--zc-text);
  font-size: 0.74rem;
  line-height: 1.65;
}

.forum-thread-comments {
  display: grid;
  gap: 0.3rem;
}

.forum-thread-comments > div {
  display: flex;
  gap: 0.45rem;
  color: var(--zc-muted);
  font-size: 0.66rem;
  line-height: 1.5;
}

.forum-thread-comments strong {
  flex: 0 0 auto;
  color: var(--zc-text-strong);
}

.forum-thread-reply {
  display: flex;
  gap: 0.4rem;
}

.forum-thread-reply input {
  min-width: 0;
  flex: 1;
  min-height: 2rem;
  padding: 0.38rem 0.55rem;
  border: 1px solid var(--zc-line);
  border-radius: 0.45rem;
  background: var(--zc-card);
  color: var(--zc-text-strong);
  font-size: 0.68rem;
}

.forum-thread-reply button {
  flex: 0 0 auto;
  padding: 0.35rem 0.62rem;
  border-radius: 0.45rem;
  background: var(--zc-accent);
  color: var(--zc-accent-ink);
  font-size: 0.68rem;
  font-weight: 950;
}

.forum-thread-reply button:disabled {
  opacity: 0.55;
}

.forum-thread-feedback {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.32rem;
  margin-top: 0.55rem;
  padding-top: 0.5rem;
  border-top: 1px solid var(--zc-line);
}

.forum-thread-feedback > span {
  margin-right: 0.15rem;
  color: var(--zc-muted);
  font-size: 0.62rem;
  font-weight: 800;
}

.forum-thread-feedback button,
.forum-thread-feedback-result {
  padding: 0.22rem 0.38rem;
  border: 1px solid var(--zc-line);
  border-radius: 0.32rem;
  color: var(--zc-muted);
  font-size: 0.6rem;
  font-weight: 800;
}

.forum-thread-feedback button:hover,
.forum-thread-feedback button:focus-visible {
  border-color: color-mix(in srgb, var(--zc-accent) 50%, var(--zc-line));
  color: var(--zc-accent);
}

.forum-thread-feedback-result {
  margin-top: 0.35rem;
  border-color: color-mix(in srgb, var(--zc-accent) 30%, var(--zc-line));
  color: var(--zc-accent);
}

.forum-thread-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-self: center;
  gap: 0.3rem;
}

.forum-thread-stats > div {
  display: grid;
  justify-items: center;
  gap: 0.14rem;
  min-width: 0;
}

.forum-thread-stats strong {
  color: var(--zc-text-strong);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.78rem;
  font-weight: 850;
}

.forum-thread-stats span {
  color: var(--zc-muted);
  font-size: 0.59rem;
  font-weight: 750;
}

.forum-thread-reason {
  display: grid;
  align-self: center;
  gap: 0.2rem;
  padding-left: 0.8rem;
  border-left: 1px solid var(--zc-line);
}

.forum-thread-reason span {
  color: var(--zc-muted);
  font-size: 0.59rem;
  font-weight: 750;
}

.forum-thread-reason strong {
  color: var(--zc-accent);
  font-size: 0.66rem;
  font-weight: 950;
  line-height: 1.4;
}

.forum-side-panel {
  padding: 1rem;
}

.forum-side-heading {
  justify-content: space-between;
  gap: 0.7rem;
}

.forum-side-list {
  display: grid;
  gap: 0.18rem;
  margin-top: 0.65rem;
}

.forum-side-list button {
  display: grid;
  gap: 0.2rem;
  padding: 0.58rem 0.62rem;
  border-radius: 0.55rem;
  background: color-mix(in srgb, var(--zc-bg) 46%, var(--zc-card));
  text-align: left;
}

.forum-side-list button:hover,
.forum-side-list button:focus-visible {
  background: color-mix(in srgb, var(--zc-accent) 8%, var(--zc-card));
}

.forum-side-list strong {
  overflow: hidden;
  color: var(--zc-text-strong);
  font-size: 0.72rem;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.forum-side-list small,
.forum-side-empty,
.forum-gate-panel > p,
.forum-card-panel > p {
  color: var(--zc-muted);
  font-size: 0.68rem;
  line-height: 1.6;
}

.forum-side-empty {
  margin-top: 0.65rem;
}

.forum-gate-panel > p,
.forum-card-panel > p {
  margin-top: 0.7rem;
}

.forum-gate-panel > button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  margin-top: 0.75rem;
  padding-top: 0.7rem;
  border-top: 1px solid var(--zc-line);
}

.forum-side-actions {
  gap: 0.45rem;
  margin-top: 0.75rem;
}

.forum-side-actions button {
  padding: 0.38rem 0.52rem;
  border: 1px solid var(--zc-line);
  border-radius: 0.45rem;
}

@media (max-width: 1100px) {
  .zero-city-city-pulse {
    grid-template-columns: minmax(18rem, 0.95fr) minmax(0, 1.05fr);
  }

  .zero-city-city-pulse :deep(.zero-city-rankings-panel--top) {
    grid-column: 1;
  }

  .forum-layer-tabs {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .zero-city-forum-districts {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .zero-city-forum-layout {
    grid-template-columns: 1fr;
  }

  .zero-city-forum-aside {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .zero-city-forum-title h1 {
    font-size: clamp(1.65rem, 5vw, 2.2rem);
  }
}

@media (max-width: 720px) {
  .zero-city-city-pulse {
    grid-template-columns: 1fr;
    gap: 0.65rem;
    padding: 0.75rem;
  }

  .zero-city-city-pulse :deep(.zero-city-rankings-panel--top) {
    grid-column: 1;
    grid-row: 1;
    padding: var(--bd-space-1) 0 var(--bd-space-3);
    border-top: 0;
    border-right: 0;
    border-bottom: var(--bd-line-width) solid color-mix(in srgb, var(--zc-line) 90%, transparent);
  }

  .city-pulse-lead {
    grid-column: 1;
    grid-row: 2;
  }

  .city-pulse-actions {
    grid-column: 1;
    grid-row: 3;
  }

  .city-pulse-lead {
    padding: 0.15rem;
  }

  .forum-layer-intro {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.45rem;
  }

  .forum-layer-intro > p {
    max-width: none;
    text-align: left;
  }

  .forum-layer-tabs {
    grid-template-columns: 1fr;
  }

  .zero-city-forum-header {
    align-items: stretch;
    flex-direction: column;
    gap: 0.9rem;
  }

  .zero-city-forum-actions {
    justify-content: flex-start;
  }

  .zero-city-forum-districts {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    overflow: visible;
  }

  .forum-district-link {
    min-width: 0;
    border-right: 1px solid var(--zc-line);
    border-bottom: 1px solid var(--zc-line);
  }

  .forum-district-link:nth-child(2n) {
    border-right: 0;
  }

  .forum-district-link:nth-last-child(-n + 2) {
    border-bottom: 0;
  }

  .zero-city-forum-aside {
    grid-template-columns: 1fr;
  }

  .forum-thread-row {
    grid-template-columns: 1.8rem minmax(0, 1fr) 6.2rem;
    gap: 0.65rem;
    padding: 0.85rem 0.9rem;
  }

  .forum-thread-reason {
    display: none;
  }
}

@media (max-width: 520px) {
  .zero-city-forum-title h1 {
    font-size: 1.65rem;
  }

  .zero-city-forum-actions .forum-visible-count {
    order: 3;
    width: 100%;
    justify-content: flex-start;
  }

  .forum-feed-header {
    align-items: stretch;
    flex-direction: column;
    gap: 0.7rem;
  }

  .forum-rules-toggle {
    align-self: flex-start;
  }

  .forum-mode-tabs {
    overflow-x: auto;
    flex-wrap: nowrap;
    gap: 0.1rem;
    padding-top: 0.22rem;
    padding-bottom: 0.22rem;
  }

  .forum-mode-tabs button {
    min-height: 2.2rem;
  }

  .forum-thread-row {
    grid-template-columns: 1.8rem minmax(0, 1fr);
  }

  .forum-thread-stats {
    grid-column: 2;
    grid-template-columns: repeat(3, auto);
    justify-content: start;
    justify-items: start;
    gap: 0.9rem;
    padding-top: 0.15rem;
  }

  .forum-thread-stats > div {
    display: flex;
    align-items: baseline;
    gap: 0.24rem;
  }

  .forum-thread-stats strong {
    font-size: 0.7rem;
  }

  .forum-thread-stats span {
    font-size: 0.58rem;
  }

  .forum-thread-main h3 {
    font-size: 0.86rem;
  }

  .forum-thread-title > span:first-child {
    font-size: 0.86rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .forum-district-link,
  .forum-district-arrow,
  .forum-thread-row {
    transition: none;
  }
}

.zero-city-editorial-home {
  display: grid;
  gap: 1rem;
}

.zero-city-editorial-head,
.zero-city-searchbar,
.zero-city-feed-column,
.zero-city-aside-panel,
.zero-city-channel-tabs {
  border: 1px solid color-mix(in srgb, var(--zc-line) 82%, transparent);
  background: var(--zc-card);
  box-shadow: var(--zc-shadow-soft);
}

.zero-city-editorial-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1.5rem;
  padding: clamp(1.25rem, 3vw, 2rem);
  border-radius: 1.75rem;
}

.zero-city-editorial-copy {
  max-width: 48rem;
}

.zero-city-editorial-copy h1 {
  margin-top: 0.45rem;
  color: var(--zc-text-strong);
  font-size: clamp(1.75rem, 4vw, 3.1rem);
  font-weight: 950;
  line-height: 1.08;
  letter-spacing: -0.04em;
}

.zero-city-editorial-copy > p:last-child {
  margin-top: 0.8rem;
  max-width: 42rem;
  color: var(--zc-muted);
  font-size: 0.94rem;
  line-height: 1.8;
}

.zero-city-editorial-actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  gap: 0.65rem;
}

.zero-city-searchbar {
  display: flex;
  min-height: 3.6rem;
  align-items: center;
  gap: 0.8rem;
  padding: 0.7rem 1rem;
  border-radius: 1.2rem;
  box-shadow: var(--zc-shadow-inset);
}

.zero-city-searchbar svg {
  width: 1.2rem;
  flex: 0 0 auto;
  color: var(--zc-muted);
}

.zero-city-searchbar input {
  min-width: 0;
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--zc-text-strong);
  font-size: 0.95rem;
  font-weight: 750;
}

.zero-city-searchbar input::placeholder {
  color: var(--zc-muted);
}

.zero-city-searchbar > span {
  flex: 0 0 auto;
  color: var(--zc-accent);
  font-size: 0.78rem;
  font-weight: 900;
}

.zero-city-district-strip {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 0.7rem;
}

.zero-city-district-strip > button,
.zero-city-channel-tabs > button {
  min-width: 0;
  border: 1px solid transparent;
  background: transparent;
  color: var(--zc-text);
  text-align: left;
  transition: transform 160ms ease, border-color 160ms ease, background-color 160ms ease;
}

.zero-city-district-strip > button {
  display: grid;
  min-height: 6.6rem;
  grid-template-columns: auto 1fr;
  grid-template-rows: auto auto;
  column-gap: 0.7rem;
  padding: 0.95rem;
  border-radius: 1.15rem;
  background: color-mix(in srgb, var(--zc-card) 78%, var(--zc-bg));
  box-shadow: var(--zc-shadow-soft);
}

.zero-city-district-strip > button:hover,
.zero-city-channel-tabs > button:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--zc-accent) 36%, var(--zc-line));
}

.zero-city-district-strip > button.active {
  border-color: color-mix(in srgb, var(--zc-accent) 58%, var(--zc-line));
  background: color-mix(in srgb, var(--zc-accent) 8%, var(--zc-card));
}

.zero-city-district-strip > button > span {
  display: grid;
  width: 2rem;
  height: 2rem;
  grid-row: 1 / span 2;
  place-items: center;
  border-radius: 0.7rem;
  background: color-mix(in srgb, var(--zc-accent) 12%, var(--zc-card));
  color: var(--zc-accent);
  font-weight: 950;
}

.zero-city-district-strip strong,
.zero-city-channel-tabs strong {
  color: var(--zc-text-strong);
  font-size: 0.88rem;
  font-weight: 950;
}

.zero-city-district-strip small,
.zero-city-channel-tabs small {
  color: var(--zc-muted);
  font-size: 0.72rem;
  line-height: 1.45;
}

.zero-city-editorial-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) minmax(16rem, 0.72fr);
  align-items: start;
  gap: 1rem;
}

.zero-city-feed-column,
.zero-city-aside-panel {
  overflow: hidden;
  border-radius: 1.5rem;
}

.zero-city-feed-toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.2rem 1.25rem;
  border-bottom: 1px solid var(--zc-line);
}

.zero-city-feed-toolbar h2,
.zero-city-aside-head h2,
.zero-city-aside-entry h2 {
  margin-top: 0.2rem;
  color: var(--zc-text-strong);
  font-size: 1.1rem;
  font-weight: 950;
}

.zero-city-feed-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.zero-city-feed-tabs button {
  padding: 0.45rem 0.72rem;
  border-radius: 999px;
  color: var(--zc-muted);
  font-size: 0.76rem;
  font-weight: 900;
}

.zero-city-feed-tabs button.active {
  background: var(--zc-accent);
  color: var(--zc-accent-ink);
}

.zero-city-feed-state {
  display: grid;
  min-height: 15rem;
  place-content: center;
  gap: 0.7rem;
  padding: 2rem;
  color: var(--zc-muted);
  text-align: center;
}

.zero-city-feed-state strong {
  color: var(--zc-text-strong);
  font-size: 1.05rem;
}

.zero-city-editorial-post {
  padding: 1.25rem;
  border-bottom: 1px solid var(--zc-line);
}

.zero-city-editorial-post:last-child {
  border-bottom: 0;
}

.zero-city-post-topline,
.zero-city-editorial-post footer,
.zero-city-editorial-post footer > div {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.7rem;
}

.zero-city-post-topline {
  justify-content: space-between;
}

.zero-city-post-topline > span {
  color: var(--zc-accent);
  font-size: 0.76rem;
  font-weight: 950;
}

.zero-city-post-topline small,
.zero-city-editorial-post footer {
  color: var(--zc-muted);
  font-size: 0.74rem;
  font-weight: 800;
}

.zero-city-editorial-post h3 {
  margin-top: 0.7rem;
  color: var(--zc-text-strong);
  font-size: 1rem;
  font-weight: 950;
  line-height: 1.45;
}

.zero-city-editorial-post > p {
  display: -webkit-box;
  overflow: hidden;
  margin-top: 0.45rem;
  color: var(--zc-muted);
  font-size: 0.86rem;
  line-height: 1.75;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.zero-city-post-signals {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
  margin-top: 0.7rem;
}

.zero-city-post-signals span {
  padding: 0.32rem 0.55rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent) 8%, var(--zc-card));
  color: var(--zc-accent);
  font-size: 0.7rem;
  font-weight: 900;
}

.zero-city-editorial-post footer {
  justify-content: space-between;
  margin-top: 0.9rem;
}

.zero-city-editorial-post footer button,
.zero-city-aside-head > button {
  color: var(--zc-accent);
  font-size: 0.74rem;
  font-weight: 950;
}

.zero-city-editorial-aside {
  display: grid;
  gap: 1rem;
}

.zero-city-aside-panel {
  padding: 1.1rem;
}

.zero-city-aside-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.zero-city-aside-list {
  display: grid;
  gap: 0.45rem;
  margin-top: 0.9rem;
}

.zero-city-aside-list button {
  display: grid;
  gap: 0.28rem;
  padding: 0.8rem;
  border-radius: 0.9rem;
  background: color-mix(in srgb, var(--zc-bg) 52%, var(--zc-card));
  text-align: left;
}

.zero-city-aside-list strong {
  color: var(--zc-text-strong);
  font-size: 0.82rem;
  line-height: 1.45;
}

.zero-city-aside-list small,
.zero-city-aside-empty {
  color: var(--zc-muted);
  font-size: 0.72rem;
  line-height: 1.55;
}

.zero-city-aside-empty {
  margin-top: 0.9rem;
}

.zero-city-aside-entry > div {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.45rem;
  margin-top: 0.9rem;
}

.zero-city-aside-entry button {
  min-height: 2.4rem;
  padding: 0.5rem;
  border-radius: 0.8rem;
  background: color-mix(in srgb, var(--zc-bg) 55%, var(--zc-card));
  color: var(--zc-text-strong);
  font-size: 0.72rem;
  font-weight: 900;
}

.zero-city-channel-tabs {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.55rem;
  padding: 0.65rem;
  border-radius: 1.15rem;
}

.zero-city-channel-tabs > button {
  display: grid;
  gap: 0.3rem;
  padding: 0.8rem;
  border-radius: 0.9rem;
}

.zero-city-channel-tabs > button.active {
  border-color: color-mix(in srgb, var(--zc-accent) 48%, var(--zc-line));
  background: color-mix(in srgb, var(--zc-accent) 8%, var(--zc-card));
}

@media (max-width: 1100px) {
  .zero-city-district-strip {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .zero-city-editorial-grid {
    grid-template-columns: 1fr;
  }

  .zero-city-editorial-aside {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .zero-city-composer-overlay {
    align-items: flex-end;
    justify-content: stretch;
    background: var(--zc-bg);
  }

  .zero-city-composer-drawer {
    width: 100%;
    max-width: none;
    height: 100dvh;
    padding: 1rem;
    box-shadow: none;
  }

  .zero-city-composer-route-grid {
    grid-template-columns: 1fr;
  }

  .zero-city-editorial-head,
  .zero-city-feed-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .zero-city-editorial-actions > button {
    flex: 1 1 8rem;
  }

  .zero-city-district-strip {
    display: flex;
    overflow-x: auto;
    padding: 0.15rem 0 0.45rem;
    scroll-snap-type: x proximity;
  }

  .zero-city-district-strip > button {
    min-width: 12.5rem;
    scroll-snap-align: start;
  }

  .zero-city-editorial-aside,
  .zero-city-channel-tabs {
    grid-template-columns: 1fr;
  }

  .zero-city-aside-entry > div {
    grid-template-columns: 1fr;
  }

  .zero-city-searchbar > span {
    display: none;
  }
}

.community-page {
  color: var(--zc-text);
}

.district-tavern {
  --zc-accent: #b66a2d;
  --zc-accent-2: #d8963f;
  --zc-accent-3: #f0a94c;
}

.district-workshop {
  --zc-accent: #1e8f9a;
  --zc-accent-2: #3f79e8;
  --zc-accent-3: #27a9c7;
}

.district-market {
  --zc-accent: #26835f;
  --zc-accent-2: #7a63d8;
  --zc-accent-3: #2fa46f;
}

.district-governance {
  --zc-accent: #5b70d7;
  --zc-accent-2: #b058b6;
  --zc-accent-3: #6e86f5;
}

.district-mine {
  --zc-accent: #7a5fd4;
  --zc-accent-2: #2b8f8a;
  --zc-accent-3: #8b73f0;
}

.zero-city-app-shell {
  display: grid;
  gap: 1rem;
  grid-template-columns: minmax(0, 1fr);
  align-items: start;
}

@media (min-width: 1180px) {
  .zero-city-app-shell {
    grid-template-columns: minmax(0, 1fr);
  }
}

.zero-city-main {
  min-width: 0;
}

.zero-city-workspace {
  min-height: calc(100vh - 7.5rem);
  position: relative;
  overflow: hidden;
}

.zero-city-workspace::before {
  content: none;
  position: absolute;
  inset: 0 auto 0 0;
  width: 0.42rem;
  background: linear-gradient(180deg, var(--zc-accent), var(--zc-accent-2));
}

.zero-city-workspace > * {
  position: relative;
}

.zero-city-workspace-head {
  display: grid;
  gap: 1rem;
  align-items: start;
}

@media (min-width: 900px) {
  .zero-city-workspace-head {
    grid-template-columns: minmax(0, 1fr) auto;
  }
}

.zero-city-workspace-head h1 {
  margin-top: 0.45rem;
  color: var(--zc-text-strong);
  font-size: clamp(2rem, 4vw, 4.1rem);
  font-weight: 950;
  letter-spacing: 0;
  line-height: 0.98;
}

.zero-city-workspace-head p:last-child {
  margin-top: 0.65rem;
  max-width: 52rem;
  color: var(--zc-muted);
  font-size: 0.98rem;
  font-weight: 750;
  line-height: 1.8;
}

.zero-city-workspace-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  justify-content: flex-start;
}

@media (min-width: 900px) {
  .zero-city-workspace-actions {
    justify-content: flex-end;
  }
}

.zero-city-workspace-actions > span,
.zero-city-current-channel {
  border: 1px solid var(--zc-line);
  background: color-mix(in srgb, var(--zc-bg) 62%, var(--zc-card-strong));
  box-shadow:
    inset 4px 4px 10px var(--zc-shadow-dark),
    inset -4px -4px 10px var(--zc-shadow-light);
}

.zero-city-workspace-actions > span {
  display: inline-flex;
  min-height: 2.75rem;
  align-items: center;
  border-radius: 999px;
  padding: 0.75rem 1rem;
  color: var(--zc-accent-2);
  font-size: 0.78rem;
  font-weight: 950;
}

.zero-city-current-channel {
  display: grid;
  gap: 0.35rem;
  border-left: 0.32rem solid color-mix(in srgb, var(--zc-accent) 72%, transparent);
  border-radius: 1.25rem;
  padding: 0.85rem 1rem;
}

.zero-city-current-channel span {
  color: var(--zc-accent);
  font-size: 0.8rem;
  font-weight: 950;
}

.zero-city-current-channel p {
  color: var(--zc-muted);
  font-size: 0.82rem;
  font-weight: 750;
  line-height: 1.65;
}

.zero-city-workbench-grid {
  display: grid;
  gap: 1rem;
}

@media (min-width: 1180px) {
  .zero-city-workbench-grid {
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
  }
}

.zero-city-workspace-feed {
  min-height: 34rem;
}

.zero-city-profile-workbench {
  display: grid;
  gap: 1rem;
}

@media (min-width: 840px) {
  .zero-city-profile-workbench {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .zero-city-profile-wide {
    grid-column: span 1;
  }
}

@media (min-width: 1180px) {
  .zero-city-profile-workbench {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .zero-city-profile-wide {
    grid-column: span 2;
  }
}

.zero-city-profile-card {
  display: grid;
  align-content: start;
  min-height: 9rem;
  border: 1px solid var(--zc-line);
  border-radius: 1.25rem;
  background: var(--zc-card);
  padding: 1rem;
  box-shadow: var(--zc-shadow-soft);
}

.zero-city-profile-card span {
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 950;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.zero-city-profile-card strong {
  margin-top: 0.45rem;
  color: var(--zc-text-strong);
  font-size: 1.35rem;
  font-weight: 950;
  line-height: 1.1;
}

.zero-city-profile-card small {
  margin-top: 0.55rem;
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 750;
  line-height: 1.65;
}

.zero-city-sidebar,
.zero-city-home-panel,
.city-home-card,
.city-district-card,
.my-zero-city-card,
.zero-city-nav-home,
.zero-city-nav-group,
.zero-city-my-panel,
.zero-city-home-list button,
.zero-city-compact-list button,
.zero-city-progress-grid article,
.zero-city-task-grid button {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-text);
  box-shadow: var(--zc-shadow-soft);
}

.zero-city-sidebar {
  display: grid;
  gap: 0.85rem;
  border-radius: 1.5rem;
  padding: 0.9rem;
}

.zero-city-sidebar-head,
.zero-city-nav-home,
.zero-city-my-panel,
.zero-city-nav-group {
  border-radius: 1.15rem;
}

.zero-city-sidebar-head {
  display: grid;
  gap: 0.35rem;
  background: color-mix(in srgb, var(--zc-card-strong) 72%, var(--zc-bg));
  padding: 0.85rem;
}

.zero-city-sidebar-head span,
.zero-city-progress-grid span,
.zero-city-task-grid span,
.zero-city-home-list span,
.city-visibility-chip,
.zero-city-nav-lock {
  color: var(--zc-accent);
  font-size: 0.7rem;
  font-weight: 950;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.zero-city-sidebar-head strong,
.zero-city-nav-home strong,
.zero-city-nav-district strong,
.zero-city-my-panel strong,
.zero-city-home-head h2,
.city-district-head h2,
.zero-city-home-list strong,
.zero-city-compact-list strong,
.zero-city-progress-grid strong,
.zero-city-task-grid strong,
.zero-city-home-empty strong {
  color: var(--zc-text-strong);
  font-weight: 950;
}

.zero-city-sidebar-head small,
.zero-city-nav-home small,
.zero-city-my-panel small,
.zero-city-home-head p,
.city-district-head p,
.zero-city-home-list small,
.zero-city-compact-list small,
.zero-city-progress-grid small,
.zero-city-task-grid small,
.zero-city-home-note,
.zero-city-home-empty p {
  color: var(--zc-muted);
  font-size: 0.76rem;
  font-weight: 750;
  line-height: 1.55;
}

.zero-city-nav-home,
.zero-city-nav-district,
.zero-city-subnav-item,
.zero-city-my-panel button,
.city-home-card,
.city-channel-list button,
.zero-city-home-list button,
.zero-city-compact-list button,
.zero-city-task-grid button {
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease, color 0.18s ease;
}

.zero-city-nav-home,
.zero-city-nav-district,
.zero-city-my-panel button {
  display: grid;
  width: 100%;
  gap: 0.25rem;
  text-align: left;
}

.zero-city-nav-home {
  padding: 0.9rem;
}

.zero-city-nav-home > span,
.zero-city-nav-district > span,
.zero-city-my-entry > span,
.city-home-card > span,
.city-district-head > span {
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.8rem;
  background: color-mix(in srgb, var(--zc-accent) 12%, var(--zc-card));
  font-size: 1rem;
}

.zero-city-nav-active,
.zero-city-subnav-active,
.zero-city-nav-group-active {
  border-color: color-mix(in srgb, var(--zc-accent) 42%, var(--zc-line)) !important;
  background: color-mix(in srgb, var(--zc-accent) 10%, var(--zc-bg)) !important;
  color: var(--zc-accent) !important;
}

.zero-city-district-nav {
  display: grid;
  gap: 0.72rem;
}

.zero-city-nav-group {
  position: relative;
  padding: 0.75rem;
}

.zero-city-nav-group::before {
  content: '';
  position: absolute;
  inset: 0.75rem auto 0.75rem 0.55rem;
  width: 0.22rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent) 42%, transparent);
}

.zero-city-nav-district {
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  border-radius: 0.9rem;
  padding: 0.45rem 0.45rem 0.45rem 0.7rem;
}

.zero-city-nav-district em {
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent) 10%, var(--zc-card));
  padding: 0.18rem 0.45rem;
  color: color-mix(in srgb, var(--zc-accent) 86%, var(--zc-muted));
  font-size: 0.62rem;
  font-style: normal;
  font-weight: 950;
}

.zero-city-subnav {
  display: grid;
  gap: 0.38rem;
  margin-top: 0.55rem;
}

.zero-city-subnav-item,
.zero-city-my-panel button,
.city-channel-list button {
  min-height: 2rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-card-strong) 82%, var(--zc-bg));
  padding: 0.42rem 0.7rem;
  color: var(--zc-muted);
  font-size: 0.74rem;
  font-weight: 900;
  text-align: left;
}

.zero-city-nav-lock {
  display: inline-flex;
  margin-top: 0.55rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent-2) 12%, transparent);
  padding: 0.28rem 0.55rem;
  color: var(--zc-accent-2);
}

.zero-city-my-panel {
  display: grid;
  gap: 0.5rem;
  padding: 0.85rem;
}

.zero-city-my-entry {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  column-gap: 0.7rem;
}

.zero-city-my-entry small {
  grid-column: 2;
}

.zero-city-nav-home:hover,
.zero-city-nav-district:hover,
.zero-city-subnav-item:hover,
.zero-city-my-panel button:hover,
.city-home-card:hover,
.city-channel-list button:hover,
.zero-city-home-list button:hover,
.zero-city-compact-list button:hover,
.zero-city-task-grid button:hover {
  border-color: color-mix(in srgb, var(--zc-accent) 34%, var(--zc-line));
  background: color-mix(in srgb, var(--zc-accent) 8%, var(--zc-bg));
  transform: translateY(-1px);
}

.zero-city-map {
  display: grid;
  gap: 0.85rem;
}

@media (min-width: 900px) {
  .zero-city-map {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .city-home-card,
  .my-zero-city-card {
    grid-column: span 1;
  }
}

.city-home-card,
.city-district-card,
.my-zero-city-card {
  display: grid;
  min-height: 10.5rem;
  align-content: start;
  gap: 0.8rem;
  border-radius: 1.2rem;
  padding: 1rem;
  text-align: left;
}

.city-home-card strong {
  color: var(--zc-text-strong);
  font-size: 1rem;
  font-weight: 950;
}

.city-home-card small {
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 750;
  line-height: 1.55;
}

.city-district-head {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
}

.city-district-head div {
  min-width: 0;
}

.city-district-head h2 {
  font-size: 1rem;
}

.city-channel-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.city-visibility-chip {
  width: fit-content;
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent-2) 12%, transparent);
  padding: 0.3rem 0.6rem;
  color: var(--zc-accent-2);
}

.zero-city-home-panel {
  border-radius: 1.45rem;
}

.zero-city-home-head {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
  justify-content: space-between;
}

.zero-city-home-head h2 {
  margin-top: 0.3rem;
  font-size: 1.25rem;
  line-height: 1.15;
}

.zero-city-home-head p {
  margin-top: 0.45rem;
  max-width: 40rem;
  font-size: 0.86rem;
}

.zero-city-home-head.compact > span {
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-bg) 58%, var(--zc-card-strong));
  padding: 0.35rem 0.65rem;
  color: var(--zc-muted);
  font-size: 0.72rem;
  font-weight: 950;
  white-space: nowrap;
}

.zero-city-home-list,
.zero-city-compact-list,
.zero-city-task-grid,
.zero-city-progress-grid {
  display: grid;
  gap: 0.75rem;
}

.zero-city-home-list button,
.zero-city-compact-list button,
.zero-city-task-grid button,
.zero-city-progress-grid article,
.zero-city-home-empty {
  border-radius: 1rem;
  padding: 0.9rem;
  text-align: left;
}

.zero-city-home-list button,
.zero-city-compact-list button,
.zero-city-task-grid button {
  display: grid;
  gap: 0.35rem;
}

.zero-city-progress-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.zero-city-progress-grid strong {
  display: block;
  margin-top: 0.45rem;
  font-size: 1.45rem;
  line-height: 1;
}

.zero-city-progress-grid small {
  display: block;
  margin-top: 0.45rem;
}

.zero-city-task-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.zero-city-home-empty {
  border: 1px solid var(--zc-line);
  background: color-mix(in srgb, var(--zc-card-strong) 78%, var(--zc-bg));
}

@media (max-width: 1179px) {
  .zero-city-sidebar {
    position: relative;
  }

  .zero-city-district-nav {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .zero-city-sidebar {
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    overflow-x: auto;
    border-radius: 1.15rem;
    padding: 0.75rem;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: none;
  }

  .zero-city-sidebar::-webkit-scrollbar {
    display: none;
  }

  .zero-city-sidebar-head,
  .zero-city-nav-home,
  .zero-city-my-panel,
  .zero-city-nav-group {
    min-width: 13.5rem;
  }

  .zero-city-sidebar-head,
  .zero-city-nav-home,
  .zero-city-my-panel {
    flex: 0 0 13.5rem;
  }

  .zero-city-district-nav {
    display: flex;
    align-items: flex-start;
    flex: 0 0 auto;
    gap: 0.75rem;
  }

  .zero-city-nav-group {
    flex: 0 0 13.5rem;
  }

  .zero-city-progress-grid,
  .zero-city-task-grid {
    grid-template-columns: 1fr;
  }

  .zero-city-home-head {
    flex-direction: column;
  }
}

.community-hero,
.panel,
.asset-card,
.token-board,
.district-card,
.archive-card,
.helper-card,
.roster-card {
  box-shadow: var(--zc-shadow-soft);
}

.panel {
  border: 1px solid var(--zc-line);
  border-radius: 2rem;
  background: var(--zc-bg);
}

.zero-city-strip,
.zero-city-module-card,
.tavern-tier-card,
.tavern-mode-card {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  box-shadow: var(--zc-shadow-soft);
}

.zero-city-strip {
  display: grid;
  min-height: 9.5rem;
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
  align-items: center;
  border-radius: 1.35rem;
  padding: 1rem;
}

@media (min-width: 860px) {
  .zero-city-strip {
    grid-template-columns: minmax(0, 1fr) auto;
  }
}

.zero-city-strip-world {
  background: var(--zc-bg);
  color: var(--zc-text);
}

.zero-city-strip-rank {
  background: var(--zc-bg);
}

.zero-city-strip-copy {
  min-width: 0;
}

.zero-city-strip-copy h1,
.zero-city-strip-copy h2 {
  margin-top: 0.25rem;
  font-size: clamp(1.55rem, 3vw, 2.35rem);
  font-weight: 950;
  line-height: 1;
}

.zero-city-strip-copy p:last-child {
  margin-top: 0.55rem;
  max-width: 34rem;
  color: var(--zc-muted);
  font-size: 0.85rem;
  font-weight: 750;
  line-height: 1.65;
}

.zero-city-strip-world .zero-city-strip-copy p:last-child {
  color: var(--zc-muted);
}

.zero-city-strip-actions,
.zero-city-rank-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.zero-city-rank-preview {
  max-width: 14rem;
}

.zero-city-rank-preview span,
.zero-city-strip-link,
.tavern-open-badge {
  display: inline-flex;
  min-height: 2rem;
  align-items: center;
  border-radius: 999px;
  padding: 0.4rem 0.7rem;
  font-size: 0.72rem;
  font-weight: 950;
}

.zero-city-rank-preview span {
  background: var(--zc-bg);
  color: var(--zc-muted);
}

.zero-city-strip-link,
.tavern-open-badge {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-accent);
}

.zero-city-module-panel,
.tavern-panel {
  border-radius: 1.35rem;
}

.zero-city-module-card,
.tavern-tier-card,
.tavern-mode-card {
  display: grid;
  min-height: 7.2rem;
  align-content: start;
  border-radius: 1rem;
  padding: 0.9rem;
  text-align: left;
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.zero-city-module-card:hover,
.tavern-mode-card:hover {
  border-color: color-mix(in srgb, var(--zc-accent) 32%, var(--zc-line));
  background: var(--zc-bg);
  transform: translateY(-1px);
}

.zero-city-module-card span,
.tavern-tier-card span,
.tavern-mode-card span {
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 950;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.zero-city-module-card strong,
.tavern-tier-card strong,
.tavern-mode-card strong {
  margin-top: 0.4rem;
  color: var(--zc-text-strong);
  font-size: 0.95rem;
  font-weight: 950;
}

.zero-city-module-card small,
.tavern-mode-card small,
.tavern-tier-card p,
.tavern-panel-head p {
  margin-top: 0.5rem;
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 700;
  line-height: 1.55;
}

.tavern-panel-head {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
  justify-content: space-between;
}

.tavern-panel-head h2 {
  margin-top: 0.25rem;
  color: var(--zc-text-strong);
  font-size: 1.35rem;
  font-weight: 950;
}

.community-empty-state {
  display: grid;
  gap: 0.8rem;
  background: var(--zc-bg);
}

.community-empty-state h3 {
  color: var(--zc-text-strong);
  font-size: 1.2rem;
  font-weight: 950;
  line-height: 1.2;
}

.community-empty-state p {
  max-width: 38rem;
  color: var(--zc-muted);
  font-size: 0.9rem;
  font-weight: 750;
  line-height: 1.7;
}

.empty-state-kicker {
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 950;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.community-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.75rem;
  border-radius: 999px;
  padding: 0.75rem 1.1rem;
  font-size: 0.875rem;
  font-weight: 900;
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.community-btn:hover {
  transform: translateY(-1px);
}

.community-btn-primary {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-accent);
}

.community-btn-secondary {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-text-strong);
}

.community-btn-ghost {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-text-strong);
}

.feedback-status-badge {
  display: inline-flex;
  min-height: 1.55rem;
  align-items: center;
  border-radius: 999px;
  padding: 0.25rem 0.6rem;
  font-size: 0.72rem;
  font-weight: 950;
}

.feedback-status-good {
  background: var(--zc-bg);
  color: var(--zc-accent);
}

.feedback-status-gold {
  background: color-mix(in srgb, var(--zc-warning) 16%, transparent);
  color: var(--zc-warning);
}

.feedback-status-neutral {
  background: var(--zc-bg);
  color: var(--zc-muted);
}

.feedback-status-risk {
  background: color-mix(in srgb, var(--zc-danger) 14%, transparent);
  color: var(--zc-danger);
}

.feedback-followup {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.feedback-followup span {
  display: inline-flex;
  min-height: 1.75rem;
  align-items: center;
  border: 1px solid var(--zc-line);
  border-radius: 999px;
  background: var(--zc-bg);
  padding: 0.35rem 0.65rem;
  color: var(--zc-muted);
  font-size: 0.72rem;
  font-weight: 850;
}

.little-stage {
  position: relative;
  display: flex;
  height: 8.5rem;
  width: 8.5rem;
  align-items: center;
  justify-content: center;
}

.little-orbit {
  position: absolute;
  inset: 0.25rem;
  border: 1px solid rgba(249, 208, 106, 0.32);
  border-radius: 999px;
}

.little-orbit::before,
.little-orbit::after {
  position: absolute;
  content: '';
  border-radius: 999px;
  background: var(--zc-warning);
}

.little-orbit::before {
  top: 0.35rem;
  left: 1.2rem;
  height: 0.55rem;
  width: 0.55rem;
}

.little-orbit::after {
  right: 0.85rem;
  bottom: 1.45rem;
  height: 0.38rem;
  width: 0.38rem;
  background: var(--zc-accent);
}

.little-mascot-image {
  position: relative;
  z-index: 1;
  width: 7.8rem;
  height: 7.8rem;
  object-fit: contain;
  filter: drop-shadow(0 18px 20px rgba(0, 0, 0, 0.32));
}

.little-prop {
  position: absolute;
  right: 0.15rem;
  top: 2.35rem;
  z-index: 2;
  display: flex;
  height: 2.15rem;
  width: 2.15rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--zc-bg);
  color: var(--zc-accent-2);
  font-size: 0.7rem;
  font-weight: 900;
}

.mascot-chat-log {
  max-height: 11.5rem;
  overflow-y: auto;
  padding-right: 0.25rem;
}

.chat-bubble {
  width: fit-content;
  max-width: 88%;
  border-radius: 1rem;
  padding: 0.65rem 0.8rem;
  font-size: 0.78rem;
  font-weight: 700;
  line-height: 1.55;
}

.chat-bubble-assistant {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-text);
}

.chat-bubble-user {
  margin-left: auto;
  background: var(--zc-bg);
  color: var(--zc-accent);
}

.suggestion-chip,
.mascot-send {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 900;
}

.suggestion-chip {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  padding: 0.4rem 0.65rem;
  color: var(--zc-muted);
}

.mascot-input {
  min-width: 0;
  flex: 1;
  border-radius: 999px;
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  padding: 0.65rem 0.85rem;
  color: var(--zc-text-strong);
  font-size: 0.8rem;
  outline: none;
}

.mascot-input::placeholder {
  color: var(--zc-subtle);
}

.mascot-send {
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  padding: 0.6rem 0.85rem;
  color: var(--zc-accent);
}

.mascot-send:disabled {
  opacity: 0.65;
}

.token-tab,
.post-filter {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.2rem;
  white-space: nowrap;
  border-radius: 999px;
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  padding: 0.55rem 0.9rem;
  color: var(--zc-muted);
  font-size: 0.75rem;
  font-weight: 900;
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease, color 0.18s ease;
}

.post-filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  overflow: visible;
}

@media (max-width: 640px) {
  .post-filter-bar {
    flex-wrap: nowrap;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: none;
  }

  .post-filter-bar::-webkit-scrollbar {
    display: none;
  }
}

.token-tab:hover,
.post-filter:hover {
  transform: translateY(-1px);
}

.token-tab-active {
  border-color: color-mix(in srgb, var(--zc-accent) 42%, var(--zc-line));
  background: var(--zc-bg);
  color: var(--zc-accent);
}

.post-filter,
.quick-tag,
.severity-chip {
  background: var(--zc-bg);
  color: var(--zc-muted);
  box-shadow: 4px 4px 9px var(--zc-shadow-dark), -4px -4px 9px var(--zc-shadow-light);
  border: none;
}

.post-filter-active,
.quick-tag-active,
.severity-chip-active {
  border-color: color-mix(in srgb, var(--zc-accent) 26%, var(--zc-line));
  background: var(--zc-bg);
  color: var(--zc-accent);
}

.quick-tag,
.severity-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-width: 1px;
  border-radius: 999px;
  padding: 0.38rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 900;
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease, color 0.18s ease;
}

.quick-tag:hover,
.severity-chip:hover {
  transform: translateY(-1px);
}

.district-mark {
  display: flex;
  height: 2.5rem;
  width: 2.5rem;
  align-items: center;
  justify-content: center;
  border-radius: 1rem;
  background: var(--zc-bg);
  color: var(--zc-accent);
  font-size: 0.95rem;
  font-weight: 900;
}

.feed-row {
  transition: background-color 0.18s ease;
}

.feed-row:hover {
  background: var(--zc-bg);
}

.signal-badge,
.runtime-badge,
.comment-badge,
.runtime-meta span {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  border-radius: 999px;
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  padding: 0.28rem 0.62rem;
  color: var(--zc-accent);
  font-size: 0.68rem;
  font-weight: 950;
  line-height: 1;
  white-space: nowrap;
}

.runtime-badge-gold,
.comment-badge-gold {
  border-color: var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-accent-2);
}

.runtime-badge-good,
.comment-badge-good {
  border-color: var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-accent);
}

.runtime-badge-neutral,
.comment-badge-neutral,
.runtime-meta span {
  border-color: var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-muted);
}

.runtime-badge-risk,
.comment-badge-risk {
  border-color: color-mix(in srgb, var(--zc-danger) 28%, transparent);
  background: color-mix(in srgb, var(--zc-danger) 12%, transparent);
  color: var(--zc-danger);
}

.runtime-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.runtime-action-btn {
  min-height: 1.85rem;
  border: 1px solid var(--zc-line);
  border-radius: 999px;
  background: var(--zc-bg);
  padding: 0.34rem 0.7rem;
  font-size: 0.72rem;
  font-weight: 900;
  color: var(--zc-accent);
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.runtime-action-btn-small {
  min-height: 1.6rem;
  padding: 0.24rem 0.58rem;
  font-size: 0.68rem;
}

.runtime-action-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--zc-accent) 32%, var(--zc-line));
  background: var(--zc-bg);
}

.runtime-action-btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.runtime-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.runtime-rule-card {
  display: flex;
  min-height: 5.4rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-radius: 1.25rem;
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  padding: 1rem;
}

.runtime-rule-card p {
  color: var(--zc-muted);
  font-size: 0.72rem;
  font-weight: 950;
}

.runtime-rule-card strong {
  display: block;
  margin-top: 0.35rem;
  color: var(--zc-text-strong);
  font-size: 1.28rem;
  font-weight: 950;
  line-height: 1;
}

.runtime-rule-card > span {
  max-width: 7rem;
  border-radius: 999px;
  padding: 0.32rem 0.62rem;
  text-align: right;
  color: var(--zc-muted);
  font-size: 0.68rem;
  font-weight: 900;
  line-height: 1.35;
}

.runtime-rule-card-good > span { background: var(--zc-bg); color: var(--zc-accent); }
.runtime-rule-card-gold > span { background: var(--zc-bg); color: var(--zc-accent-2); }
.runtime-rule-card-neutral > span { background: var(--zc-bg); color: var(--zc-muted); }
.runtime-rule-card-risk > span { background: var(--zc-bg); color: var(--zc-danger); }

.archive-warm {
  border-color: var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-accent-2);
}

.archive-cyan {
  border-color: var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-accent);
}

.archive-dark {
  border-color: var(--zc-line);
  background: var(--zc-bg);
  color: var(--zc-text-strong);
}

.community-input {
  width: 100%;
  border-radius: 1.25rem;
  border: 1px solid var(--zc-line);
  background: var(--zc-bg);
  padding: 0.85rem 1rem;
  color: var(--zc-text-strong);
  font-size: 0.875rem;
  font-weight: 700;
  outline: none;
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.community-input:focus {
  border-color: color-mix(in srgb, var(--zc-accent) 42%, var(--zc-line));
  box-shadow:
    inset 4px 4px 10px var(--zc-shadow-dark),
    inset -4px -4px 10px var(--zc-shadow-light);
}

.community-input::placeholder {
  color: var(--zc-subtle);
}

.community-command,
.decision-panel {
  border-color: var(--zc-line);
  background: color-mix(in srgb, var(--zc-surface-raised) 88%, var(--zc-bg));
  color: var(--zc-text);
  box-shadow: var(--zc-shadow-soft);
  backdrop-filter: var(--zc-blur);
  -webkit-backdrop-filter: var(--zc-blur);
}

.community-command::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(90deg, var(--zc-grid) 1px, transparent 1px),
    linear-gradient(180deg, var(--zc-grid) 1px, transparent 1px);
  background-size: 44px 44px;
  opacity: 0.42;
}

.command-primary,
.command-operator,
.decision-panel {
  position: relative;
  min-width: 0;
}

.command-kicker,
.command-source-chip,
.operator-status {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  border-radius: 999px;
  border: 1px solid var(--zc-line);
  background: color-mix(in srgb, var(--zc-card-strong) 82%, transparent);
  padding: 0.3rem 0.7rem;
  color: var(--zc-accent);
  font-size: 0.68rem;
  font-weight: 950;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.command-live-dot {
  display: inline-flex;
  height: 0.55rem;
  width: 0.55rem;
  border-radius: 999px;
  background: var(--zc-success);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--zc-success) 16%, transparent);
}

.command-mini-link,
.archive-entry-link {
  display: inline-flex;
  min-height: 2.2rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid var(--zc-line);
  background: color-mix(in srgb, var(--zc-card-strong) 86%, transparent);
  padding: 0.48rem 0.82rem;
  color: var(--zc-text-strong);
  font-size: 0.76rem;
  font-weight: 950;
  white-space: nowrap;
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease, color 0.18s ease;
}

.command-mini-link:hover,
.archive-entry-link:hover,
.archive-entry-link-active {
  border-color: color-mix(in srgb, var(--zc-accent) 42%, transparent);
  background: color-mix(in srgb, var(--zc-accent) 12%, var(--zc-card));
  color: var(--zc-accent);
  transform: translateY(-1px);
}

.command-source-chip {
  color: var(--zc-accent-2);
  text-transform: none;
}

.command-title {
  max-width: 46rem;
  color: var(--zc-text-strong);
  font-size: clamp(2.1rem, 4.5vw, 4.8rem);
  font-weight: 950;
  letter-spacing: 0;
  line-height: 0.96;
}

.command-description {
  max-width: 44rem;
  color: var(--zc-muted);
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.85;
}

.command-metric,
.priority-action,
.command-operator,
.decision-signal,
.focus-discussion,
.operator-task {
  border: 1px solid var(--zc-line);
  background: var(--zc-card);
  color: var(--zc-text);
}

.command-metric,
.priority-action,
.decision-signal,
.focus-discussion,
.operator-task {
  border-radius: 1.25rem;
}

.command-metric {
  min-height: 7rem;
  padding: 1rem;
}

.command-metric span,
.priority-action span,
.operator-task span,
.focus-discussion small {
  color: var(--zc-subtle);
  font-size: 0.72rem;
  font-weight: 900;
}

.command-metric strong {
  display: block;
  margin-top: 0.5rem;
  color: var(--zc-text-strong);
  font-size: 1.8rem;
  font-weight: 950;
  line-height: 1;
}

.command-metric small,
.priority-action small,
.operator-task small {
  display: block;
  margin-top: 0.5rem;
  color: var(--zc-muted);
  font-size: 0.76rem;
  font-weight: 700;
  line-height: 1.55;
}

.zero-city-command {
  isolation: isolate;
}

.zero-city-console {
  align-self: stretch;
}

.city-operator-card {
  display: grid;
  gap: 1rem;
  overflow: hidden;
}

.city-operator-main {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(8rem, 0.45fr);
  gap: 1rem;
  align-items: end;
}

.city-operator-copy {
  min-width: 0;
}

.city-operator-copy p {
  margin-top: 0.65rem;
  font-size: 0.9rem;
  font-weight: 800;
  line-height: 1.75;
}

.city-operator-portrait {
  display: grid;
  min-height: 11.5rem;
  place-items: end center;
  border-radius: 1.35rem;
  border: 1px solid var(--zc-line);
  background:
    radial-gradient(circle at 50% 86%, color-mix(in srgb, var(--zc-accent) 16%, transparent), transparent 58%),
    var(--zc-card-strong);
  box-shadow:
    inset 8px 8px 18px var(--zc-shadow-dark),
    inset -8px -8px 18px var(--zc-shadow-light);
}

.city-operator-portrait img {
  width: min(10rem, 86%);
  max-height: 12rem;
  object-fit: contain;
  filter: drop-shadow(0 18px 20px color-mix(in srgb, var(--zc-shadow-dark) 62%, transparent));
}

.city-duty-panel {
  display: grid;
  gap: 0.4rem;
  border-radius: 1.15rem;
  border: 1px solid var(--zc-line);
  background: color-mix(in srgb, var(--zc-card-strong) 86%, transparent);
  padding: 0.95rem;
}

.city-duty-panel strong {
  color: var(--zc-text-strong);
  font-size: 0.95rem;
  font-weight: 950;
}

.city-duty-panel span {
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 750;
  line-height: 1.65;
}

.city-suggestion-chip,
.zero-city-route-card {
  border: 1px solid var(--zc-line);
  background: var(--zc-card);
  color: var(--zc-text-strong);
  box-shadow: var(--zc-shadow-soft);
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.city-suggestion-chip {
  min-height: 2.25rem;
  border-radius: 999px;
  padding: 0.45rem 0.8rem;
  font-size: 0.76rem;
  font-weight: 950;
}

.city-suggestion-chip:hover,
.zero-city-route-card:hover {
  border-color: color-mix(in srgb, var(--zc-accent) 42%, var(--zc-line));
  background: color-mix(in srgb, var(--zc-accent) 9%, var(--zc-card));
  transform: translateY(-1px);
}

.zero-city-route-card {
  display: grid;
  min-height: 8.4rem;
  align-content: start;
  gap: 0.5rem;
  border-radius: 1.2rem;
  padding: 1rem;
  text-align: left;
}

.zero-city-route-card span {
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 950;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.zero-city-route-card strong {
  color: var(--zc-text-strong);
  font-size: 1rem;
  font-weight: 950;
}

.zero-city-route-card small {
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 750;
  line-height: 1.55;
}

@media (max-width: 720px) {
  .city-operator-main {
    grid-template-columns: 1fr;
  }

  .city-operator-portrait {
    min-height: 10rem;
  }
}

.priority-action,
.operator-task {
  display: grid;
  width: 100%;
  gap: 0.35rem;
  padding: 1rem;
  text-align: left;
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.priority-action:hover,
.operator-task:hover,
.focus-discussion:hover {
  border-color: color-mix(in srgb, var(--zc-accent) 38%, var(--zc-line));
  background: color-mix(in srgb, var(--zc-accent) 8%, var(--zc-card));
  transform: translateY(-1px);
}

.priority-action strong,
.operator-task strong {
  color: var(--zc-text-strong);
  font-size: 0.95rem;
  font-weight: 950;
}

.command-operator,
.decision-panel {
  border-radius: 1.5rem;
}

.command-operator {
  padding: 1.1rem;
}

.command-operator h2 {
  margin-top: 0.55rem;
  color: var(--zc-text-strong);
  font-size: 1.35rem;
  font-weight: 950;
}

.command-operator p,
.operator-brief p {
  color: var(--zc-muted);
}

.operator-stage,
.operator-brief {
  border-radius: 1.2rem;
  border: 1px solid var(--zc-line);
  background: color-mix(in srgb, var(--zc-card-strong) 84%, transparent);
  padding: 0.85rem;
}

.operator-stage .little-stage {
  height: 7rem;
  width: 7rem;
}

.operator-stage .little-mascot-image {
  height: 6.4rem;
  width: 6.4rem;
}

.compact-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.compact-head h2 {
  margin-top: 0.45rem;
  color: var(--zc-text-strong);
  font-size: 1.05rem;
  font-weight: 950;
}

.compact-head > span,
.compact-head > button {
  border-radius: 999px;
  border: 1px solid var(--zc-line);
  background: var(--zc-card-strong);
  padding: 0.35rem 0.65rem;
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 900;
  white-space: nowrap;
}

.decision-signal {
  display: flex;
  min-height: 5rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.9rem;
}

.decision-signal strong,
.focus-discussion h3 {
  color: var(--zc-text-strong);
  font-weight: 950;
}

.decision-signal p,
.focus-discussion p {
  margin-top: 0.35rem;
  color: var(--zc-muted);
  font-size: 0.78rem;
  font-weight: 700;
  line-height: 1.55;
}

.decision-signal > span {
  flex: 0 0 auto;
  border-radius: 999px;
  padding: 0.3rem 0.62rem;
  font-size: 0.78rem;
  font-weight: 950;
}

.signal-good > span { background: color-mix(in srgb, var(--zc-success) 16%, transparent); color: var(--zc-success); }
.signal-risk > span { background: color-mix(in srgb, var(--zc-danger) 16%, transparent); color: var(--zc-danger); }
.signal-hot > span { background: color-mix(in srgb, var(--zc-accent-2) 18%, transparent); color: var(--zc-accent-2); }
.signal-calm > span { background: color-mix(in srgb, var(--zc-accent) 14%, transparent); color: var(--zc-accent); }

.focus-discussion {
  min-height: 11rem;
  padding: 1rem;
  transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.focus-discussion span {
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent) 12%, transparent);
  padding: 0.22rem 0.55rem;
  color: var(--zc-accent);
  font-size: 0.7rem;
  font-weight: 900;
}

.focus-discussion h3 {
  margin-top: 0.75rem;
  font-size: 0.95rem;
  line-height: 1.45;
}

.focus-discussion b,
.focus-discussion button {
  color: var(--zc-accent);
  font-size: 0.72rem;
  font-weight: 950;
}

.operator-task span {
  display: inline-flex;
  height: 1.6rem;
  width: 1.6rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: color-mix(in srgb, var(--zc-accent-2) 18%, transparent);
  color: var(--zc-accent-2);
}

/* Zero City community workbench refresh */
.community-page {
  color: var(--zc-text);
}

.community-page :deep(.text-gray-900),
.community-page :deep(.text-gray-800),
.community-page :deep(.text-gray-700) {
  color: var(--zc-text-strong) !important;
}

.community-page :deep(.text-gray-600),
.community-page :deep(.text-gray-500) {
  color: var(--zc-muted) !important;
}

.community-page :deep(.text-gray-400) {
  color: var(--zc-subtle) !important;
}

.community-hero {
  border-color: var(--zc-line) !important;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--zc-surface-raised) 84%, transparent), color-mix(in srgb, var(--zc-accent) 8%, var(--zc-surface)) 58%, color-mix(in srgb, var(--zc-accent-2) 8%, var(--zc-surface))) !important;
  box-shadow: var(--zc-shadow-soft);
}

.community-hero h1,
.community-page h2,
.community-page h3,
.community-page b,
.community-page .section-head h2 {
  color: var(--zc-text-strong) !important;
}

.community-hero p,
.community-page p,
.community-page .hint,
.community-page .empty-line {
  color: var(--zc-muted);
}

.community-hero > .pointer-events-none {
  display: none;
}

.community-hero::before,
.community-hero::after {
  content: '';
  position: absolute;
  pointer-events: none;
}

.community-hero::before {
  inset: 0;
  background:
    linear-gradient(90deg, var(--zc-grid) 1px, transparent 1px),
    linear-gradient(180deg, var(--zc-grid) 1px, transparent 1px);
  background-size: 48px 48px;
  opacity: 0.42;
}

.community-hero::after {
  left: 1.5rem;
  right: 1.5rem;
  bottom: 1rem;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--zc-line-strong), transparent);
}

.community-hero .inline-flex,
.community-page .rounded-full {
  border-color: var(--zc-line);
}

.city-card,
#community-token-intel {
  border-color: var(--zc-line-strong) !important;
  background:
    linear-gradient(145deg, color-mix(in srgb, var(--zc-bg) 88%, var(--zc-accent) 6%), color-mix(in srgb, var(--zc-surface-solid) 90%, var(--zc-accent-2) 5%)) !important;
  color: var(--zc-text-strong) !important;
  box-shadow: var(--zc-shadow-lift);
}

.city-card p,
.city-card span,
#community-token-intel p,
#community-token-intel span {
  color: color-mix(in srgb, var(--zc-text) 70%, transparent);
}

.panel,
.asset-card,
.district-card,
.helper-card,
.roster-card,
.token-board,
.archive-card {
  border-color: var(--zc-line) !important;
  background: var(--zc-surface) !important;
  color: var(--zc-text) !important;
  box-shadow: var(--zc-shadow-soft);
  backdrop-filter: var(--zc-blur);
  -webkit-backdrop-filter: var(--zc-blur);
}

.asset-card,
.district-card,
.helper-card,
.roster-card,
.archive-card {
  border-radius: var(--zc-radius-card) !important;
}

.community-btn {
  border-radius: 999px;
  color: var(--zc-text-strong);
}

.community-btn-primary {
  border-color: color-mix(in srgb, var(--zc-accent) 68%, transparent);
  background: linear-gradient(135deg, var(--zc-accent), color-mix(in srgb, var(--zc-accent) 72%, var(--zc-accent-2)));
  color: var(--zc-accent-ink);
  box-shadow: 0 14px 34px color-mix(in srgb, var(--zc-accent) 18%, transparent);
}

.community-btn-secondary,
.community-btn-ghost {
  border-color: var(--zc-line-strong);
  background: color-mix(in srgb, var(--zc-card-strong) 86%, transparent);
  color: var(--zc-text-strong);
}

.community-btn:hover:not(:disabled),
.quick-tag:hover,
.severity-chip:hover,
.token-tab:hover,
.post-filter:hover,
.suggestion-chip:hover,
.pool-market-card:hover {
  transform: translateY(-1px);
}

.little-stage,
.mascot-chat,
.token-board,
.feed-row,
.pool-row,
.post-row,
.community-page .rounded-2xl,
.community-page .rounded-xl {
  border-color: var(--zc-line) !important;
  background: var(--zc-card) !important;
  color: var(--zc-text) !important;
}

.little-prop,
.chat-bubble-user,
.mascot-send,
.district-mark {
  background: var(--zc-accent-2);
  color: var(--zc-accent-ink);
}

.little-orbit {
  border-color: color-mix(in srgb, var(--zc-accent-2) 32%, transparent);
}

.chat-bubble-assistant,
.suggestion-chip,
.token-tab,
.post-filter,
.quick-tag,
.severity-chip {
  border-color: var(--zc-line);
  background: color-mix(in srgb, var(--zc-card-strong) 78%, transparent);
  color: var(--zc-muted);
}

.token-tab-active,
.post-filter-active,
.quick-tag-active,
.severity-chip-active {
  border-color: color-mix(in srgb, var(--zc-accent) 42%, transparent);
  background: color-mix(in srgb, var(--zc-accent) 12%, transparent);
  color: var(--zc-accent);
}

.mascot-input,
.community-input {
  border-color: var(--zc-line) !important;
  background: var(--zc-surface-raised) !important;
  color: var(--zc-text-strong) !important;
}

.mascot-input::placeholder,
.community-input::placeholder {
  color: var(--zc-subtle) !important;
}

.feed-row:hover {
  background: color-mix(in srgb, var(--zc-accent) 7%, var(--zc-card)) !important;
}

.archive-warm,
.archive-cyan,
.archive-dark {
  border-color: var(--zc-line-strong) !important;
  background: var(--zc-card-strong) !important;
  color: var(--zc-text) !important;
}

.asset-card span,
.district-card span,
.helper-card span,
.roster-card span,
.feed-row button,
.panel button:not(.community-btn):not(.quick-tag):not(.severity-chip) {
  color: var(--zc-accent);
}

.token-board strong,
#community-token-intel strong,
.archive-card span,
.archive-card h3 {
  color: var(--zc-accent-2);
}

.community-page .bg-white,
.community-page .bg-white\/70 {
  background-color: var(--zc-card) !important;
}

.panel > div,
.feed-row,
.helper-card,
.roster-card,
.district-card,
.asset-card {
  border-color: var(--zc-line) !important;
}

/* comment thread block — Neumorphism concave surface */
.community-comments-block {
  background: var(--zc-bg);
  box-shadow: inset 5px 5px 12px var(--zc-shadow-dark), inset -5px -5px 12px var(--zc-shadow-light);
}

.community-comment-item {
  background: var(--zc-bg);
  box-shadow: 4px 4px 9px var(--zc-shadow-dark), -4px -4px 9px var(--zc-shadow-light);
}

.community-comment-item b { color: var(--zc-text-strong); }
.community-comment-item p { color: var(--zc-muted); }

/* ── Module-aware community material overrides ── */
.community-page {
  color: var(--module-text, var(--zc-text));
}

.community-hero,
.panel,
.asset-card,
.token-board,
.district-card,
.archive-card,
.helper-card,
.roster-card,
.zero-city-strip,
.zero-city-module-card,
.tavern-tier-card,
.tavern-mode-card {
  border: 1px solid var(--module-line, var(--zc-line)) !important;
  background: var(--module-canvas, var(--zc-surface)) !important;
  box-shadow: 0 14px 34px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 28%, transparent), inset 0 1px 0 color-mix(in srgb, var(--module-shadow-light, var(--zc-shadow-light)) 62%, transparent) !important;
}

.zero-city-module-card,
.tavern-tier-card,
.tavern-mode-card,
.district-card,
.helper-card,
.archive-card,
.asset-card,
.roster-card {
  background: var(--zc-card) !important;
}

.zero-city-strip-world {
  background: var(--zc-bg) !important;
  color: var(--zc-text) !important;
}

.zero-city-strip-copy h1,
.zero-city-strip-copy h2,
.zero-city-module-card strong,
.tavern-tier-card strong,
.tavern-mode-card strong,
.tavern-panel-head h2,
.community-empty-state h3 {
  color: var(--zc-text-strong) !important;
}

.zero-city-strip-copy p:last-child,
.zero-city-strip-world .zero-city-strip-copy p:last-child,
.zero-city-module-card small,
.tavern-mode-card small,
.tavern-tier-card p,
.tavern-panel-head p,
.community-empty-state p,
.feedback-followup span {
  color: var(--zc-muted) !important;
}

.zero-city-rank-preview span,
.feedback-status-neutral,
.runtime-rule-card-neutral > span,
.feedback-followup span {
  background: color-mix(in srgb, var(--module-canvas, var(--zc-bg)) 68%, var(--zc-card-strong)) !important;
  color: var(--module-muted, var(--zc-muted)) !important;
  box-shadow: inset 0 1px 6px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 24%, transparent) !important;
}

.zero-city-strip-link,
.tavern-open-badge,
.empty-state-kicker,
.zero-city-module-card span,
.tavern-tier-card span,
.tavern-mode-card span,
.feedback-status-good {
  color: var(--zc-accent) !important;
}

.zero-city-strip-link,
.tavern-open-badge,
.feedback-status-good,
.feedback-status-gold,
.feedback-status-risk {
  border: 1px solid var(--module-line, var(--zc-line)) !important;
  background: color-mix(in srgb, var(--module-canvas, var(--zc-bg)) 68%, var(--zc-card-strong)) !important;
  box-shadow: inset 0 1px 6px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 24%, transparent) !important;
}

.feedback-status-gold {
  color: var(--zc-warning) !important;
}

.feedback-status-risk {
  color: var(--zc-danger) !important;
}

.zero-city-module-card:hover,
.tavern-mode-card:hover {
  background: var(--module-canvas, var(--zc-bg)) !important;
  box-shadow: 0 18px 40px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 34%, transparent) !important;
}

.community-empty-state {
  background: color-mix(in srgb, var(--module-canvas, var(--zc-bg)) 68%, var(--zc-card-strong)) !important;
  box-shadow: inset 0 3px 12px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 28%, transparent) !important;
}

.community-btn-secondary,
.community-btn-ghost {
  border: 1px solid var(--module-line-strong, var(--zc-line-strong)) !important;
  background: var(--module-canvas, var(--zc-card-strong)) !important;
  color: var(--module-text-strong, var(--zc-text-strong)) !important;
  box-shadow: 0 8px 20px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 26%, transparent) !important;
}

.community-btn-primary {
  border: 1px solid color-mix(in srgb, var(--zc-accent) 62%, transparent) !important;
  background: linear-gradient(135deg, var(--zc-accent), color-mix(in srgb, var(--zc-accent) 68%, var(--zc-accent-2))) !important;
  color: var(--zc-accent-ink) !important;
  box-shadow: 0 14px 34px color-mix(in srgb, var(--zc-accent) 20%, transparent) !important;
}

.community-kicker {
  color: var(--zc-accent);
  font-size: 0.75rem;
  font-weight: 950;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.community-section-title,
.community-section-small-title,
.community-card-title {
  color: var(--zc-text-strong);
}

.community-card-copy,
.community-muted-line,
.community-meta,
.community-time,
.community-checkbox-row {
  color: var(--zc-muted);
}

.community-section-head,
.community-divide > * + * {
  border-color: var(--zc-line) !important;
}

.community-chip,
.community-callout,
.community-checkbox-row {
  border: 1px solid var(--module-line, var(--zc-line)) !important;
  background: color-mix(in srgb, var(--module-canvas, var(--zc-bg)) 58%, var(--zc-card-strong)) !important;
  box-shadow: inset 0 2px 8px color-mix(in srgb, var(--module-shadow-dark, var(--zc-shadow-dark)) 24%, transparent) !important;
}

.community-chip {
  color: var(--zc-muted);
}

.community-chip-accent,
.community-link {
  color: var(--zc-accent) !important;
}

.community-chip-strong {
  color: var(--zc-text-strong) !important;
}

.community-link-secondary {
  color: var(--zc-accent-2) !important;
}

.community-link:hover {
  color: var(--zc-text-strong) !important;
}
</style>
