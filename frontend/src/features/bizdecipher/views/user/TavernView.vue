<template>
  <AppLayout>
    <main class="tavern-page" data-visual-domain="tavern">
      <header class="tavern-page-head">
        <div>
          <p class="tavern-kicker">零号城 · 酒馆</p>
          <h1>{{ activeTab === 'rooms' ? '我的房间' : '剧本与游戏' }}</h1>
          <p>{{ activeTab === 'rooms' ? '这里仅展示服务器返回的已创建或已加入房间。' : '先体验本地故事，再从公开剧本创建真实服务器房间。' }}</p>
        </div>
        <div class="tavern-hero-actions">
          <button type="button" class="tavern-secondary-button" @click="openTavernChat"><Icon name="users" size="sm" />实时大厅</button>
          <button type="button" class="tavern-secondary-button" @click="router.push('/community?district=tavern&channel=chat-hall')"><Icon name="arrowLeft" size="sm" />闲聊广场</button>
          <button type="button" class="tavern-secondary-button" :disabled="loadingAny" @click="refreshAll"><Icon name="refresh" size="sm" :class="loadingAny ? 'animate-spin' : ''" />刷新</button>
        </div>
      </header>

      <section class="tavern-boundary">
        <Icon name="infoCircle" size="sm" />
        <span>放松，有底线。闲谈、剧本和房间各有归属；费用与房间状态始终清晰可见。</span>
      </section>

      <TavernHowTo />

      <section class="tavern-tabs">
        <button :class="['tavern-tab', activeTab === 'hall' && 'active']" type="button" @click="switchTab('hall')">剧本大厅</button>
        <button :class="['tavern-tab', activeTab === 'mine' && 'active']" type="button" @click="switchTab('mine')">投稿与我的剧本</button>
        <button :class="['tavern-tab', activeTab === 'rooms' && 'active']" type="button" @click="switchTab('rooms')">我的房间</button>
      </section>

      <section v-if="activeTab === 'hall'" class="tavern-local-section">
        <div class="tavern-local-heading">
          <div><p class="tavern-kicker">本地初始体验</p><h2>两段可以立即开始的固定故事</h2></div>
          <span>不联网 · 不调用 AI · 不扣费</span>
        </div>
        <StarterStories />
      </section>

      <section v-if="activeTab === 'hall'" class="tavern-filter-bar">
        <div class="tavern-search">
          <Icon name="search" size="sm" />
          <input v-model="keyword" type="search" placeholder="搜索剧本、场景、NPC、关键词" @keyup.enter="loadPublicScripts" />
        </div>
        <select v-model="activeGenre" class="tavern-select" @change="loadPublicScripts">
          <option v-for="option in genreOptionsWithAll" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <select v-model="activeSort" class="tavern-select" @change="loadPublicScripts">
          <option value="quality">质量优先</option>
          <option value="latest">最新发布</option>
          <option value="updated">最近更新</option>
          <option value="popular">热度优先</option>
        </select>
      </section>

      <section v-if="activeTab === 'hall'" class="tavern-workspace">
        <main class="tavern-panel tavern-catalog">
          <div class="tavern-section-head">
            <div>
              <p class="tavern-kicker">Script Hall</p>
              <h2>公开剧本馆</h2>
            </div>
            <span>{{ publicScripts.length }} 个上架剧本</span>
          </div>

          <div v-if="loadingPublic" class="tavern-script-list">
            <article v-for="i in 4" :key="i" class="tavern-script-card tavern-loading-card">
              <span></span>
              <strong></strong>
              <p></p>
            </article>
          </div>

          <div v-else-if="publicScripts.length === 0" class="tavern-empty">
            <Icon name="grid" size="xl" />
            <h3>还没有可公开开局的剧本</h3>
            <p>管理员审核通过后，公开剧本会出现在这里。你也可以先提交剧本草稿或进入待审核。</p>
            <button type="button" class="tavern-primary-button" @click="switchTab('mine')">提交第一个剧本</button>
          </div>

          <div v-else class="tavern-script-list">
            <article
              v-for="script in publicScripts"
              :key="script.id"
              :class="['tavern-script-card', selectedScript?.id === script.id && 'selected']"
              @click="selectScript(script)"
            >
              <div class="tavern-script-main">
                <div class="tavern-script-topline">
                  <span class="tavern-pill">{{ genreLabel(script.genre) }}</span>
                  <span class="tavern-pill muted">{{ difficultyLabel(script.difficulty) }}</span>
                  <span class="tavern-pill strong">{{ script.entry_price_usd && !/^0(?:\.0+)?$/.test(script.entry_price_usd) ? `USD ${script.entry_price_usd} / 人` : pricingLabel(script.pricing_mode, script.entry_credit_cost, script.entry_balance_cost) }}</span>
                </div>
                <h3>{{ script.title }}</h3>
                <p>{{ script.summary }}</p>
                <div class="tavern-tags">
                  <span v-for="tag in compactTags(script.tags)" :key="tag" class="tavern-tag">{{ tag }}</span>
                </div>
              </div>
              <div class="tavern-script-meta">
                <strong>{{ script.player_min }}-{{ script.player_max }} 人</strong>
                <span>{{ script.estimated_minutes }} 分钟</span>
                <span>质量 {{ formatScore(script.quality_score) }}</span>
              </div>
            </article>
          </div>
        </main>

        <aside class="tavern-panel tavern-control-panel">
          <div class="tavern-section-head">
            <div>
              <p class="tavern-kicker">Room Draft</p>
              <h2>创建房间草稿</h2>
            </div>
            <span>从公开剧本开局</span>
          </div>

          <div v-if="selectedScript" class="tavern-selected-script">
            <span>已选择剧本</span>
            <strong>{{ selectedScript.title }}</strong>
            <p>{{ selectedScript.summary }}</p>
            <div class="tavern-selected-grid">
              <span>{{ selectedScript.player_min }}-{{ selectedScript.player_max }} 人</span>
              <span>{{ pricingLabel(selectedScript.pricing_mode, selectedScript.entry_credit_cost, selectedScript.entry_balance_cost) }}</span>
            </div>
          </div>
          <div v-else class="tavern-selected-empty">
            <Icon name="document" size="lg" />
            <p>先在左侧选择一个已上架公开剧本，再创建房间草稿。</p>
          </div>

          <form class="tavern-form" @submit.prevent="handleCreateRoom">
            <label>
              房间标题
              <input v-model="roomForm.title" type="text" maxlength="160" placeholder="例如：午夜账本第一局" required />
            </label>
            <div class="tavern-form-grid">
              <label>
                人数上限
                <input v-model.number="roomForm.max_players" type="number" min="1" max="12" />
              </label>
              <label>
                主持模式
                <select v-model="roomForm.host_mode">
                  <option value="ai_host">AI 主持</option>
                  <option value="human_host">真人主持</option>
                  <option value="mixed">混合主持</option>
                </select>
              </label>
            </div>
            <div class="tavern-form-grid">
              <label>
                可见性
                <select v-model="roomForm.visibility">
                  <option value="private">私密草稿</option>
                  <option value="public">公开招募</option>
                </select>
              </label>
              <label>
                计费模式
                <select v-model="roomForm.billing_mode" :disabled="Boolean(selectedScript?.entry_price_usd && !/^0(?:\.0+)?$/.test(selectedScript.entry_price_usd))">
                  <option value="free">免费</option>
                  <option value="credit">积分</option>
                  <option v-if="selectedScript?.entry_price_usd && !/^0(?:\.0+)?$/.test(selectedScript.entry_price_usd)" value="balance">作者 USD 入场券</option>
                </select>
                <span v-if="selectedScript?.entry_price_usd && !/^0(?:\.0+)?$/.test(selectedScript.entry_price_usd)">作者入场价 USD {{ selectedScript.entry_price_usd }} / 人</span>
              </label>
            </div>
            <button type="submit" class="tavern-primary-button full" :disabled="creatingRoom || !selectedScript">
              {{ creatingRoom ? '创建中...' : '创建房间草稿' }}
            </button>
          </form>
        </aside>
      </section>

      <section v-if="activeTab === 'mine'" class="tavern-workspace tavern-workspace-mine">
        <main class="tavern-panel tavern-submit-panel">
          <div class="tavern-section-head">
            <div>
              <p class="tavern-kicker">Script Submission</p>
              <h2>投稿剧本</h2>
            </div>
            <span>草稿 / 待审核</span>
          </div>

          <form class="tavern-form" @submit.prevent="handleSubmitScript">
            <label>
              剧本标题
              <input v-model="scriptForm.title" type="text" maxlength="160" placeholder="例如：雾港来信" required />
            </label>
            <label>
              一句话简介
              <input v-model="scriptForm.summary" type="text" maxlength="360" placeholder="说明适合几人、什么题材、核心体验是什么" required />
            </label>
            <label>
              剧本说明
              <textarea v-model="scriptForm.description" rows="5" placeholder="写清楚故事背景、玩家目标、线索结构、结局条件和主持注意事项" required></textarea>
            </label>
            <div class="tavern-form-grid">
              <label>
                类型
                <select v-model="scriptForm.genre">
                  <option v-for="option in genreOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </label>
              <label>
                难度
                <select v-model="scriptForm.difficulty">
                  <option value="easy">入门</option>
                  <option value="normal">标准</option>
                  <option value="hard">进阶</option>
                  <option value="expert">专家</option>
                </select>
              </label>
            </div>
            <div class="tavern-form-grid">
              <label>
                最少人数
                <input v-model.number="scriptForm.player_min" type="number" min="1" max="12" />
              </label>
              <label>
                最多人数
                <input v-model.number="scriptForm.player_max" type="number" min="1" max="12" />
              </label>
              <label>
                预计时长
                <input v-model.number="scriptForm.estimated_minutes" type="number" min="10" max="480" step="10" />
              </label>
            </div>
            <label>
              标签
              <input v-model="scriptTagsText" type="text" placeholder="逗号分隔，例如：推理, 城市, 短局" />
            </label>
            <label>
              NPC 卡
              <input v-model="npcCardsText" type="text" placeholder="逗号分隔，例如：酒馆老板, 档案员, 失踪工程师" />
            </label>
            <label>
              AI 主持提示
              <textarea v-model="scriptForm.host_brief" rows="3" placeholder="给主持人的节奏、禁区、线索释放规则"></textarea>
            </label>
            <label>
              开场提示词
              <textarea v-model="scriptForm.opening_prompt" rows="3" placeholder="第一幕开场白或玩家进入房间后的启动提示"></textarea>
            </label>
            <label>
              安全和质量备注
              <textarea v-model="scriptForm.safety_notes" rows="2" placeholder="剧本边界、敏感内容规避、坏剧本风险点"></textarea>
            </label>
            <div class="tavern-form-grid">
              <label>
                定价模式
                <select v-model="scriptForm.pricing_mode">
                  <option value="free">免费</option>
                  <option value="credit">积分</option>
                </select>
              </label>
              <label>
                积分入场价
                <input v-model.number="scriptForm.entry_credit_cost" type="number" min="0" />
              </label>
            </div>
            <div class="tavern-form-footer">
              <label class="tavern-radio">
                <input v-model="scriptForm.status" type="radio" value="draft" />
                保存草稿
              </label>
              <label class="tavern-radio">
                <input v-model="scriptForm.status" type="radio" value="pending" />
                提交审核
              </label>
              <button type="submit" class="tavern-primary-button" :disabled="submittingScript">
                {{ submittingScript ? '提交中...' : '保存剧本' }}
              </button>
            </div>
          </form>
        </main>

        <aside class="tavern-panel tavern-my-panel">
          <div class="tavern-section-head">
            <div>
              <p class="tavern-kicker">My Scripts</p>
              <h2>我的剧本</h2>
            </div>
            <span>{{ myScripts.length }} 个</span>
          </div>

          <div v-if="loadingMine" class="tavern-row-list">
            <article v-for="i in 3" :key="i" class="tavern-my-row tavern-loading-card"><span></span><strong></strong><p></p></article>
          </div>
          <div v-else-if="myScripts.length === 0" class="tavern-empty compact">
            <Icon name="document" size="lg" />
            <h3>还没有剧本</h3>
            <p>先保存草稿或提交审核，后续管理员审核通过后会进入公开剧本馆。</p>
          </div>
          <div v-else class="tavern-row-list">
            <article v-for="script in myScripts" :key="script.id" class="tavern-my-row">
              <div class="tavern-my-row-main">
                <div class="tavern-script-topline">
                  <span class="tavern-pill" :data-status="script.status">{{ statusLabel(script.status) }}</span>
                  <span class="tavern-pill muted">{{ genreLabel(script.genre) }}</span>
                  <span v-if="latestPackage(script.id)" class="tavern-pill strong">
                    包 v{{ latestPackage(script.id)?.version }}
                  </span>
                </div>
                <h3>{{ script.title }}</h3>
                <p>{{ script.summary }}</p>
                <small v-if="script.review_note">审核备注：{{ script.review_note }}</small>
              </div>
              <div class="tavern-my-row-actions">
                <span class="tavern-date">{{ formatDate(script.created_at) }}</span>
                <button type="button" class="tavern-secondary-button compact" @click="togglePackageEditor(script)">
                  {{ openPackageScriptID === script.id ? '收起游戏包' : '游戏包版本' }}
                </button>
              </div>

              <div v-if="openPackageScriptID === script.id" class="tavern-package-inline">
                <TavernScriptPricing :script-id="script.id" :initial-price="script.entry_price_usd || '0'" @changed="loadMyScripts" />
                <div class="tavern-package-head">
                  <div>
                    <strong>声明式游戏包</strong>
                    <small>只提交规则与内容清单，不在主站进程执行上传代码。</small>
                  </div>
                  <span>{{ packagesByScript[script.id]?.length ?? 0 }} 个版本</span>
                </div>

                <div v-if="packagesLoading[script.id]" class="tavern-package-muted">正在读取版本…</div>
                <div v-else-if="(packagesByScript[script.id]?.length ?? 0) > 0" class="tavern-package-versions">
                  <article v-for="pkg in packagesByScript[script.id]" :key="pkg.id">
                    <div>
                      <strong>v{{ pkg.version }}</strong>
                      <span :data-status="pkg.status">{{ packageStatusLabel(pkg.status) }}</span>
                      <small>{{ pkg.manifest.limits.max_turns }} 回合 · {{ pkg.manifest.limits.max_scenes }} 场景</small>
                    </div>
                    <div class="tavern-package-actions">
                      <button type="button" class="tavern-secondary-button compact" @click="downloadPackage(pkg)">
                        <Icon name="download" size="sm" />下载
                      </button>
                      <button
                        v-if="pkg.status === 'draft'"
                        type="button"
                        class="tavern-primary-button compact"
                        :disabled="packageBusyKey === `${script.id}:${pkg.version}:publish`"
                        @click="handlePackageStatus(script, pkg, 'publish')"
                      >
                        发布
                      </button>
                      <button
                        v-if="pkg.status === 'published'"
                        type="button"
                        class="tavern-secondary-button compact danger"
                        :disabled="packageBusyKey === `${script.id}:${pkg.version}:revoke`"
                        @click="handlePackageStatus(script, pkg, 'revoke')"
                      >
                        撤销
                      </button>
                    </div>
                  </article>
                </div>
                <div v-else class="tavern-package-muted">还没有游戏包版本。新房间会等待第一个已发布版本。</div>

                <TavernPackageImport :script-id="script.id" @imported="loadPackages(script.id)" />
                <form class="tavern-package-form" @submit.prevent="handleCreatePackage(script)">
                  <div class="tavern-form-grid">
                    <label>
                      新版本
                      <input v-model.trim="packageForm.version" type="text" maxlength="64" placeholder="例如：v2" required />
                    </label>
                    <label>
                      入口类型
                      <select v-model="packageForm.entryKind">
                        <option value="prompt_flow">提示流</option>
                        <option value="scene_graph">场景图</option>
                      </select>
                    </label>
                  </div>
                  <div class="tavern-form-grid">
                    <label>
                      最大回合
                      <input v-model.number="packageForm.maxTurns" type="number" min="1" max="500" />
                    </label>
                    <label>
                      最大场景
                      <input v-model.number="packageForm.maxScenes" type="number" min="1" max="100" />
                    </label>
                    <label>
                      入口标识
                      <input v-model.trim="packageForm.entryRef" type="text" maxlength="160" placeholder="main" required />
                    </label>
                  </div>
                  <fieldset class="tavern-package-permissions">
                    <legend>能力声明</legend>
                    <label><input v-model="packageForm.aiGateway" type="checkbox" /> AI 网关</label>
                    <label><input v-model="packageForm.save" type="checkbox" /> 存档</label>
                    <label><input v-model="packageForm.score" type="checkbox" /> 计分</label>
                    <label><input v-model="packageForm.presence" type="checkbox" /> 在线状态</label>
                  </fieldset>
                  <label>
                    包内内容说明
                    <textarea v-model="packageForm.description" rows="2" maxlength="4000" />
                  </label>
                  <label>
                    包内主持提示
                    <textarea v-model="packageForm.hostBrief" rows="2" maxlength="4000" />
                  </label>
                  <label>
                    包内开场提示
                    <textarea v-model="packageForm.openingPrompt" rows="2" maxlength="4000" />
                  </label>
                  <div class="tavern-package-form-footer">
                    <small>创建后先保存为草稿，再由你决定何时发布。</small>
                    <button type="submit" class="tavern-primary-button compact" :disabled="packageBusyKey === `${script.id}:create`">
                      {{ packageBusyKey === `${script.id}:create` ? '创建中...' : '创建版本' }}
                    </button>
                  </div>
                </form>
              </div>
            </article>
          </div>
        </aside>
      </section>

      <section v-if="activeTab === 'rooms'" class="tavern-panel tavern-rooms-panel">
        <TavernCommercePolicy v-if="authStore.user?.role === 'admin'" />
        <form class="tavern-form-grid" @submit.prevent="invitedRoom = Number(inviteRoomInput)">
          <label>房间号<input v-model="inviteRoomInput" type="number" min="1" step="1" required /></label>
          <button type="submit" class="tavern-secondary-button compact">查看房间</button>
        </form>
        <TavernTicketPanel v-if="invitedRoom > 0" :key="`invite-${invitedRoom}`" :room-id="invitedRoom" @changed="loadMyRooms" />
        <div class="tavern-section-head">
          <div>
            <p class="tavern-kicker">My Rooms</p>
            <h2>我的房间</h2>
          </div>
          <button type="button" class="tavern-secondary-button compact" @click="loadMyRooms">
            <Icon name="refresh" size="sm" :class="loadingRooms ? 'animate-spin' : ''" />
            刷新
          </button>
        </div>

        <div v-if="loadingRooms" class="tavern-room-grid">
          <article v-for="i in 3" :key="i" class="tavern-room-card tavern-loading-card"><span></span><strong></strong><p></p></article>
        </div>
        <div v-else-if="myRooms.length === 0" class="tavern-empty">
          <Icon name="users" size="xl" />
          <h3>还没有房间</h3>
          <p>在剧本大厅选择一个已上架公开剧本后，可以创建房间并推进公开招募、加入、开始和收尾。</p>
          <button type="button" class="tavern-primary-button" @click="switchTab('hall')">去剧本大厅开房</button>
        </div>
        <div v-else class="tavern-room-grid">
          <article v-for="room in myRooms" :key="room.id" class="tavern-room-card">
            <div class="tavern-script-topline">
              <span class="tavern-pill" :data-status="room.status">{{ roomStatusLabel(room.status) }}</span>
              <span class="tavern-pill muted">{{ hostModeLabel(room.host_mode) }}</span>
            </div>
            <h3>{{ room.title }}</h3>
            <p>{{ room.script_title }}</p>
            <div class="tavern-room-meta">
              <span>{{ room.current_players }}/{{ room.max_players }} 人</span>
              <span>{{ room.ticket_price_usd ? `USD ${room.ticket_price_usd} / 人` : pricingLabel(room.billing_mode, room.entry_credit_cost, room.entry_balance_cost) }}</span>
              <span>房间号 #{{ room.id }}</span>
              <span>{{ room.package_version ? `包 v${room.package_version}` : '包未绑定' }}</span>
              <span>{{ formatDate(room.created_at) }}</span>
            </div>
            <TavernTicketPanel v-if="room.ticket_price_usd" :room-id="room.id" :status="room.status" :owner-id="room.owner_id" @changed="loadMyRooms" />
            <div class="tavern-room-actions">
              <button v-if="canOpenRoom(room)" type="button" class="tavern-secondary-button compact" :disabled="isRoomActionBusy(room, 'open')" @click="handleRoomAction(room, 'open')">
                {{ isRoomActionBusy(room, 'open') ? '公开中...' : '公开招募' }}
              </button>
              <span v-if="room.current_user_joined" class="tavern-joined-badge">已加入</span>
              <button v-else-if="canJoinRoom(room)" type="button" class="tavern-secondary-button compact" :disabled="isRoomActionBusy(room, 'join')" @click="handleRoomAction(room, 'join')">
                {{ isRoomActionBusy(room, 'join') ? '加入中...' : '加入房间' }}
              </button>
              <button v-if="canEnterRuntime(room)" type="button" class="tavern-primary-button compact" :disabled="isRoomActionBusy(room, 'runtime')" @click="handleEnterRuntime(room)">
                {{ isRoomActionBusy(room, 'runtime') ? '准备中...' : '进入舞台' }}
              </button>
              <button v-if="canStartRoom(room)" type="button" class="tavern-primary-button compact" :disabled="isRoomActionBusy(room, 'start')" @click="handleRoomAction(room, 'start')">
                {{ isRoomActionBusy(room, 'start') ? '启动中...' : '开始' }}
              </button>
              <button v-if="canCompleteRoom(room)" type="button" class="tavern-primary-button compact" :disabled="isRoomActionBusy(room, 'complete')" @click="handleRoomAction(room, 'complete')">
                {{ isRoomActionBusy(room, 'complete') ? '完成中...' : '完成' }}
              </button>
              <button v-if="canCancelRoom(room)" type="button" class="tavern-secondary-button compact danger" :disabled="isRoomActionBusy(room, 'cancel')" @click="handleRoomAction(room, 'cancel')">
                {{ isRoomActionBusy(room, 'cancel') ? '取消中...' : '取消' }}
              </button>
            </div>
          </article>
        </div>
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import AppLayout from '@/components/layout/AppLayout.vue'
import StarterStories from '@/features/bizdecipher/components/tavern/StarterStories.vue'
import TavernHowTo from '@/features/bizdecipher/components/tavern/TavernHowTo.vue'
import TavernTicketPanel from '@/features/bizdecipher/components/tavern/TavernTicketPanel.vue'
import TavernScriptPricing from '@/features/bizdecipher/components/tavern/TavernScriptPricing.vue'
import TavernCommercePolicy from '@/features/bizdecipher/components/tavern/TavernCommercePolicy.vue'
import TavernPackageImport from '@/features/bizdecipher/components/tavern/TavernPackageImport.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  cancelTavernRoom,
  completeTavernRoom,
  createTavernGamePackage,
  createTavernRoom,
  createTavernRuntimeSession,
  createTavernScript,
  joinTavernRoom,
  listTavernGamePackages,
  listMyTavernRooms,
  listMyTavernScripts,
  listTavernScripts,
  openTavernRoom,
  publishTavernGamePackage,
  revokeTavernGamePackage,
  startTavernRoom,
  type TavernGamePackage,
  type TavernGamePackagePayload,
  type TavernRoom,
  type TavernRoomPayload,
  type TavernScript,
  type TavernScriptPayload,
} from '@/features/bizdecipher/api/bizdecipher'

const appStore = useAppStore()
const authStore = useAuthStore()
const inviteRoomInput = ref('')
const invitedRoom = ref(0)
const route = useRoute()
const router = useRouter()
const activeTab = ref<'hall' | 'mine' | 'rooms'>('hall')

const loadingPublic = ref(false)
const loadingMine = ref(false)
const loadingRooms = ref(false)
const submittingScript = ref(false)
const creatingRoom = ref(false)
const roomActionKey = ref('')
const packageBusyKey = ref('')
const openPackageScriptID = ref<number | null>(null)

const publicScripts = ref<TavernScript[]>([])
const myScripts = ref<TavernScript[]>([])
const myRooms = ref<TavernRoom[]>([])
const selectedScript = ref<TavernScript | null>(null)
const packagesByScript = ref<Record<number, TavernGamePackage[]>>({})
const packagesLoading = ref<Record<number, boolean>>({})

const keyword = ref('')
const activeGenre = ref('all')
const activeSort = ref('quality')
const scriptTagsText = ref('')
const npcCardsText = ref('')

const loadingAny = computed(() => loadingPublic.value || loadingMine.value || loadingRooms.value || submittingScript.value || creatingRoom.value)
function openTavernChat() {
  router.push('/tavern/lounge')
}

const genreOptions = [
  { value: 'mystery', label: '推理' },
  { value: 'sci_fi', label: '科幻' },
  { value: 'fantasy', label: '奇幻' },
  { value: 'horror', label: '悬疑惊悚' },
  { value: 'workplace', label: '职场' },
  { value: 'historical', label: '历史' },
  { value: 'open_world', label: '开放世界' },
  { value: 'other', label: '其他' },
]
const genreOptionsWithAll = [{ value: 'all', label: '全部类型' }, ...genreOptions]

function defaultScriptForm(): TavernScriptPayload {
  return {
    title: '',
    summary: '',
    description: '',
    genre: 'mystery',
    status: 'pending',
    visibility: 'private',
    player_min: 1,
    player_max: 6,
    estimated_minutes: 60,
    difficulty: 'normal',
    tags: [],
    npc_cards: [],
    host_brief: '',
    opening_prompt: '',
    safety_notes: '',
    pricing_mode: 'free',
    entry_credit_cost: 0,
    entry_balance_cost: 0,
    author_revenue_share: 0,
  }
}

function defaultRoomForm(): Omit<TavernRoomPayload, 'script_id'> {
  return {
    title: '',
    visibility: 'private',
    host_mode: 'ai_host',
    billing_mode: 'free',
    entry_credit_cost: 0,
    entry_balance_cost: 0,
    max_players: 6,
    room_config: {},
  }
}

const scriptForm = ref<TavernScriptPayload>(defaultScriptForm())
const roomForm = ref<Omit<TavernRoomPayload, 'script_id'>>(defaultRoomForm())
const packageForm = ref({
  version: '',
  entryKind: 'prompt_flow' as 'prompt_flow' | 'scene_graph',
  entryRef: 'main',
  maxTurns: 48,
  maxScenes: 12,
  aiGateway: true,
  save: true,
  score: false,
  presence: false,
  description: '',
  hostBrief: '',
  openingPrompt: '',
})

function resetPackageForm(script: TavernScript) {
  packageForm.value = {
    version: '',
    entryKind: 'prompt_flow',
    entryRef: script.slug || 'main',
    maxTurns: 48,
    maxScenes: 12,
    aiGateway: true,
    save: true,
    score: false,
    presence: false,
    description: script.description,
    hostBrief: script.host_brief,
    openingPrompt: script.opening_prompt,
  }
}

function latestPackage(scriptID: number): TavernGamePackage | undefined {
  return packagesByScript.value[scriptID]?.[0]
}

function downloadPackage(pkg: TavernGamePackage) {
  const content = JSON.stringify({ version: pkg.version, manifest: pkg.manifest }, null, 2)
  const url = URL.createObjectURL(new Blob([content], { type: 'application/json' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `tavern-${pkg.script_id}-${pkg.version.replace(/[^a-zA-Z0-9._-]/g, '_')}.json`
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

async function loadPackages(scriptID: number) {
  packagesLoading.value = { ...packagesLoading.value, [scriptID]: true }
  try {
    packagesByScript.value = {
      ...packagesByScript.value,
      [scriptID]: await listTavernGamePackages(scriptID),
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载游戏包失败'))
  } finally {
    packagesLoading.value = { ...packagesLoading.value, [scriptID]: false }
  }
}

async function togglePackageEditor(script: TavernScript) {
  if (openPackageScriptID.value === script.id) {
    openPackageScriptID.value = null
    return
  }
  openPackageScriptID.value = script.id
  resetPackageForm(script)
  await loadPackages(script.id)
}

async function handleCreatePackage(script: TavernScript) {
  if (!packageForm.value.version.trim()) {
    appStore.showError('请填写游戏包版本')
    return
  }
  packageBusyKey.value = `${script.id}:create`
  try {
    const payload: TavernGamePackagePayload = {
      version: packageForm.value.version,
      manifest: {
        schema_version: 'tavern.package.v1',
        runtime_kind: 'declarative',
        protocol_version: '2026-09-13.package.v1',
        entry: {
          kind: packageForm.value.entryKind,
          ref: packageForm.value.entryRef || script.slug || 'main',
        },
        permissions: {
          ai_gateway: packageForm.value.aiGateway,
          save: packageForm.value.save,
          score: packageForm.value.score,
          purchases: false,
          presence: packageForm.value.presence,
        },
        content: {
          description: packageForm.value.description.trim(),
          host_brief: packageForm.value.hostBrief.trim(),
          opening_prompt: packageForm.value.openingPrompt.trim(),
          safety_notes: script.safety_notes,
          npc_cards: script.npc_cards,
        },
        limits: {
          max_turns: packageForm.value.maxTurns,
          max_scenes: packageForm.value.maxScenes,
        },
      },
    }
    await createTavernGamePackage(script.id, payload)
    appStore.showSuccess(`游戏包 v${payload.version} 已创建为草稿`)
    resetPackageForm(script)
    await loadPackages(script.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '创建游戏包失败'))
  } finally {
    packageBusyKey.value = ''
  }
}

async function handlePackageStatus(script: TavernScript, pkg: TavernGamePackage, action: 'publish' | 'revoke') {
  packageBusyKey.value = `${script.id}:${pkg.version}:${action}`
  try {
    if (action === 'publish') await publishTavernGamePackage(script.id, pkg.version)
    else await revokeTavernGamePackage(script.id, pkg.version)
    appStore.showSuccess(action === 'publish' ? `游戏包 v${pkg.version} 已发布` : `游戏包 v${pkg.version} 已撤销`)
    await loadPackages(script.id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, action === 'publish' ? '发布游戏包失败' : '撤销游戏包失败'))
  } finally {
    packageBusyKey.value = ''
  }
}

async function switchTab(tab: 'hall' | 'mine' | 'rooms') {
  activeTab.value = tab
  const view = tab === 'rooms' ? 'rooms' : tab === 'mine' ? 'scripts' : undefined
  await router.replace({ path: '/tavern', query: view ? { view } : {} })
  if (tab === 'hall') await loadPublicScripts()
  if (tab === 'mine') await loadMyScripts()
  if (tab === 'rooms') await loadMyRooms()
}

async function loadPublicScripts() {
  loadingPublic.value = true
  try {
    publicScripts.value = await listTavernScripts({
      keyword: keyword.value || undefined,
      genre: activeGenre.value === 'all' ? undefined : activeGenre.value,
      sort: activeSort.value,
      limit: 60,
    })
    if (selectedScript.value && !publicScripts.value.some(script => script.id === selectedScript.value?.id)) {
      selectedScript.value = publicScripts.value[0] ?? null
      syncRoomFormFromScript()
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载剧本大厅失败'))
  } finally {
    loadingPublic.value = false
  }
}

async function loadMyScripts() {
  loadingMine.value = true
  try {
    myScripts.value = await listMyTavernScripts({ limit: 50 })
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载我的剧本失败'))
  } finally {
    loadingMine.value = false
  }
}

async function loadMyRooms() {
  loadingRooms.value = true
  try {
    myRooms.value = await listMyTavernRooms({ limit: 50 })
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载我的房间失败'))
  } finally {
    loadingRooms.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadPublicScripts(), loadMyScripts(), loadMyRooms()])
}

function selectScript(script: TavernScript) {
  selectedScript.value = script
  syncRoomFormFromScript()
}

function syncRoomFormFromScript() {
  const script = selectedScript.value
  if (!script) return
  roomForm.value = {
    ...roomForm.value,
    title: roomForm.value.title || `${script.title} · 第一局`,
    billing_mode: script.pricing_mode,
    entry_credit_cost: script.entry_credit_cost,
    entry_balance_cost: script.entry_balance_cost,
    max_players: script.player_max,
  }
}

function parseList(value: string): string[] {
  return value
    .split(/[，,]/)
    .map(item => item.trim())
    .filter(Boolean)
}

async function handleSubmitScript() {
  if (!scriptForm.value.title.trim() || !scriptForm.value.summary.trim() || !scriptForm.value.description.trim()) {
    appStore.showError('请填写标题、简介和剧本说明')
    return
  }
  submittingScript.value = true
  try {
    const payload: TavernScriptPayload = {
      ...scriptForm.value,
      tags: parseList(scriptTagsText.value),
      npc_cards: parseList(npcCardsText.value),
    }
    await createTavernScript(payload)
    appStore.showSuccess(payload.status === 'draft' ? '剧本草稿已保存' : '剧本已提交审核')
    scriptForm.value = defaultScriptForm()
    scriptTagsText.value = ''
    npcCardsText.value = ''
    await switchTab('mine')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '提交剧本失败'))
  } finally {
    submittingScript.value = false
  }
}

async function handleCreateRoom() {
  if (!selectedScript.value) {
    appStore.showError('请先选择一个公开上架剧本')
    return
  }
  if (!roomForm.value.title?.trim()) {
    appStore.showError('请填写房间标题')
    return
  }
  creatingRoom.value = true
  try {
    await createTavernRoom({
      ...roomForm.value,
      script_id: selectedScript.value.id,
    })
    appStore.showSuccess('房间草稿已创建')
    roomForm.value = defaultRoomForm()
    await switchTab('rooms')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '创建房间失败'))
  } finally {
    creatingRoom.value = false
  }
}

type RoomAction = 'open' | 'join' | 'start' | 'complete' | 'cancel'
type RoomBusyAction = RoomAction | 'runtime'

function canOpenRoom(room: TavernRoom): boolean {
  return room.status === 'draft' || (room.status === 'lobby' && room.visibility !== 'public')
}

function canJoinRoom(room: TavernRoom): boolean {
  return !room.ticket_price_usd && !room.current_user_joined && room.status === 'lobby' && room.current_players < room.max_players
}

function canEnterRuntime(room: TavernRoom): boolean {
  return !['completed', 'cancelled'].includes(String(room.status))
}

function canStartRoom(room: TavernRoom): boolean {
  return room.status === 'lobby' && room.current_players > 0
}

function canCompleteRoom(room: TavernRoom): boolean {
  return room.status === 'running' || room.status === 'paused'
}

function canCancelRoom(room: TavernRoom): boolean {
  return room.status === 'draft' || room.status === 'lobby'
}

function roomActionBusyKey(room: TavernRoom, action: RoomBusyAction): string {
  return `${room.id}:${action}`
}

function isRoomActionBusy(room: TavernRoom, action: RoomBusyAction): boolean {
  return roomActionKey.value === roomActionBusyKey(room, action)
}

async function handleRoomAction(room: TavernRoom, action: RoomAction) {
  roomActionKey.value = roomActionBusyKey(room, action)
  try {
    const actionMap: Record<RoomAction, (id: number) => Promise<TavernRoom>> = {
      open: openTavernRoom,
      join: joinTavernRoom,
      start: startTavernRoom,
      complete: completeTavernRoom,
      cancel: cancelTavernRoom,
    }
    const messageMap: Record<RoomAction, string> = {
      open: '房间已公开招募',
      join: '已加入房间',
      start: '房间已开始',
      complete: '房间已完成',
      cancel: '房间已取消',
    }
    await actionMap[action](room.id)
    appStore.showSuccess(messageMap[action])
    await loadMyRooms()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '房间动作失败'))
  } finally {
    roomActionKey.value = ''
  }
}

async function handleEnterRuntime(room: TavernRoom) {
  roomActionKey.value = roomActionBusyKey(room, 'runtime')
  try {
    const session = await createTavernRuntimeSession(room.id)
    if (!session.token) {
      throw new Error('runtime session token missing')
    }
    sessionStorage.setItem(tavernRuntimeSessionStorageKey(session.id), session.token)
    appStore.showSuccess(`舞台凭证已创建：${session.config?.bridge.provider ?? 'runtime'} / ${session.config?.budget.turn_budget ?? 0} 回合预算`)
    await router.push({
      name: 'ZeroCityTavernStage',
      query: {
        session: String(session.id),
        room: String(room.id),
      },
    })
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '准备酒馆舞台失败'))
  } finally {
    roomActionKey.value = ''
  }
}

function tavernRuntimeSessionStorageKey(sessionID: number | string): string {
  return `zero-city:tavern-runtime:${sessionID}`
}

function genreLabel(value: string): string {
  return genreOptions.find(option => option.value === value)?.label ?? '其他'
}

function difficultyLabel(value: string): string {
  const map: Record<string, string> = { easy: '入门', normal: '标准', hard: '进阶', expert: '专家' }
  return map[value] ?? '标准'
}

function statusLabel(value: string): string {
  const map: Record<string, string> = {
    draft: '草稿',
    pending: '待审核',
    listed: '已上架',
    rejected: '已拒绝',
    archived: '已归档',
    delisted: '已下架',
  }
  return map[value] ?? value
}

function packageStatusLabel(value: string): string {
  const map: Record<string, string> = {
    draft: '草稿',
    published: '已发布',
    revoked: '已撤销',
  }
  return map[value] ?? value
}

function roomStatusLabel(value: string): string {
  const map: Record<string, string> = {
    draft: '草稿',
    lobby: '大厅',
    running: '进行中',
    paused: '暂停',
    completed: '已完成',
    cancelled: '已取消',
  }
  return map[value] ?? value
}

function hostModeLabel(value: string): string {
  const map: Record<string, string> = { ai_host: 'AI 主持', human_host: '真人主持', mixed: '混合主持' }
  return map[value] ?? 'AI 主持'
}

function pricingLabel(mode: string, credits = 0, balance = 0): string {
  if (mode === 'credit') return `${credits || 0} 积分`
  if (mode === 'balance') return `¥${Number(balance || 0).toFixed(2)}`
  if (mode === 'hybrid') return `${credits || 0} 积分 + ¥${Number(balance || 0).toFixed(2)}`
  return '免费'
}

function compactTags(tags: string[] = []): string[] {
  return tags.slice(0, 4)
}

function formatScore(value?: number): string {
  return Number(value || 0).toFixed(1)
}

function formatDate(dateStr?: string | null): string {
  if (!dateStr) return '--'
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return '--'
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

onMounted(() => {
  syncTabFromRoute()
  refreshAll()
})

function syncTabFromRoute(): void {
  if (route.query.view === 'rooms') activeTab.value = 'rooms'
  else if (route.query.view === 'scripts' || route.query.action === 'create-script') activeTab.value = 'mine'
  else activeTab.value = 'hall'
}

watch(() => [route.query.view, route.query.action] as const, syncTabFromRoute)
</script>

<style scoped>
.tavern-page {
  --tavern-accent: var(--module-accent, #d4a25a);
  --tavern-accent-2: var(--module-accent-2, #8fb6ff);
  --tavern-ink: var(--module-ink, var(--zc-text));
  --tavern-ink-strong: var(--module-ink-strong, var(--zc-text-strong));
  --tavern-muted: var(--module-muted, var(--zc-muted));
  --tavern-line: var(--module-line, var(--zc-line));
  --tavern-line-strong: var(--module-line-strong, var(--zc-line-strong));
  --tavern-panel: var(--module-panel, var(--zc-surface));
  --tavern-card: var(--module-card, var(--zc-card));
  --tavern-raised: var(--module-panel-raised, var(--zc-surface-raised));
  --tavern-shadow: var(--module-shadow, var(--zc-shadow-soft));
  --tavern-shadow-lift: var(--module-shadow-lift, var(--zc-shadow-lift));
  --tavern-soft: var(--module-accent-soft, rgba(212, 162, 90, 0.14));

  display: grid;
  max-width: 1240px;
  margin: 0 auto;
  padding: 24px 20px 64px;
  gap: 18px;
  color: var(--tavern-ink);
  background: var(--bd-canvas);
}

.tavern-page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; border-bottom: 1px solid var(--tavern-line); padding-bottom: 20px; }
.tavern-page-head h1 { margin: 8px 0; color: var(--tavern-ink-strong); font-size: 24px; line-height: 1.2; }
.tavern-page-head > div > p:last-child { margin: 0; color: var(--tavern-muted); font-size: 14px; line-height: 1.7; }
.tavern-local-section { border-bottom: 1px solid var(--tavern-line); padding: 8px 0 28px; }
.tavern-local-heading { display: flex; justify-content: space-between; align-items: flex-end; gap: 16px; margin-bottom: 18px; }
.tavern-local-heading h2 { margin: 6px 0 0; color: var(--tavern-ink-strong); font-size: 18px; }
.tavern-local-heading > span { color: var(--tavern-muted); font-size: 12px; }

.tavern-hero {
  display: grid;
  gap: 18px;
  align-items: stretch;
}

@media (min-width: 1024px) {
  .tavern-hero {
    grid-template-columns: minmax(0, 1.22fr) minmax(280px, 0.78fr);
  }
}

.tavern-hero-copy {
  position: relative;
  overflow: hidden;
  min-height: 280px;
  padding: 30px;
  border-radius: 24px;
  isolation: isolate;
}

.tavern-hero-copy::after {
  content: '';
  position: absolute;
  inset: auto -10% -40% 40%;
  height: 70%;
  border-radius: 50%;
  background: radial-gradient(circle, color-mix(in srgb, var(--tavern-accent) 28%, transparent), transparent 70%);
  filter: blur(8px);
  pointer-events: none;
  z-index: 0;
}

.tavern-hero-copy > * {
  position: relative;
  z-index: 1;
}

.tavern-kicker {
  color: var(--tavern-accent);
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.tavern-hero-copy .tavern-kicker {
  color: color-mix(in srgb, var(--module-art-ink, #fff1d6) 78%, var(--tavern-accent));
}

.tavern-hero-copy h1 {
  margin: 14px 0 14px;
  font-size: clamp(34px, 5vw, 48px);
  font-weight: 950;
  line-height: 1.02;
  letter-spacing: -0.03em;
  color: var(--module-art-ink, #fff8ec);
  text-shadow: 0 10px 30px rgba(0, 0, 0, 0.28);
}

.tavern-hero-copy p {
  max-width: 640px;
  color: color-mix(in srgb, var(--module-art-ink, #fff1d6) 78%, #b7ad9a);
  font-size: 15px;
  font-weight: 650;
  line-height: 1.8;
}

.tavern-hero-actions,
.tavern-form-footer,
.tavern-script-topline,
.tavern-tabs,
.tavern-filter-bar,
.tavern-selected-grid,
.tavern-room-meta,
.tavern-room-actions,
.tavern-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.tavern-hero-actions {
  margin-top: 24px;
}

.tavern-hero-board {
  display: grid;
  gap: 12px;
  padding: 0;
  background: transparent;
  border: 0;
  box-shadow: none;
}

.tavern-signal,
.tavern-chat-entry {
  display: grid;
  gap: 6px;
  border: 1px solid var(--tavern-line);
  border-radius: 16px;
  background: var(--tavern-card);
  padding: 16px;
  box-shadow: var(--tavern-shadow);
  text-align: left;
}

.tavern-signal span,
.tavern-selected-script span,
.tavern-script-meta span,
.tavern-date,
.tavern-room-meta span,
.tavern-chat-entry-kicker {
  color: var(--tavern-muted);
  font-size: 12px;
  font-weight: 800;
}

.tavern-signal strong {
  color: var(--tavern-ink-strong);
  font-size: 30px;
  font-weight: 950;
}

.tavern-chat-entry {
  cursor: pointer;
  transition: transform 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--tavern-accent) 10%, var(--tavern-card)), var(--tavern-card));
}

.tavern-chat-entry:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--tavern-accent) 36%, var(--tavern-line));
  box-shadow: var(--tavern-shadow-lift);
}

.tavern-chat-entry strong {
  color: var(--tavern-ink-strong);
  font-size: 16px;
  font-weight: 950;
}

.tavern-chat-entry p {
  margin: 0;
  color: var(--tavern-muted);
  font-size: 13px;
  font-weight: 650;
  line-height: 1.6;
}

.tavern-boundary {
  display: flex;
  align-items: center;
  gap: 10px;
  border: 1px solid color-mix(in srgb, var(--tavern-accent) 22%, var(--tavern-line));
  border-radius: 14px;
  background: color-mix(in srgb, var(--tavern-accent) 8%, var(--tavern-panel));
  padding: 12px 14px;
  color: var(--tavern-accent);
  font-size: 13px;
  font-weight: 800;
  line-height: 1.6;
}

.tavern-tabs,
.tavern-filter-bar {
  align-items: center;
}

.tavern-tabs {
  border-bottom: 1px solid var(--tavern-line);
  padding-bottom: 6px;
}

.tavern-tab,
.tavern-primary-button,
.tavern-secondary-button,
.tavern-select,
.tavern-search input,
.tavern-form input,
.tavern-form textarea,
.tavern-form select {
  border: 1px solid var(--tavern-line);
  border-radius: 12px;
  font: inherit;
}

.tavern-tab {
  background: transparent;
  padding: 10px 14px;
  color: var(--tavern-muted);
  font-size: 14px;
  font-weight: 900;
  transition: border-color 160ms ease, background 160ms ease, color 160ms ease;
}

.tavern-tab.active {
  border-color: color-mix(in srgb, var(--tavern-accent) 34%, var(--tavern-line));
  background: var(--tavern-soft);
  color: var(--tavern-accent);
}

.tavern-primary-button,
.tavern-secondary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 40px;
  padding: 0 14px;
  font-size: 13px;
  font-weight: 900;
  transition: transform 160ms ease, opacity 160ms ease, box-shadow 160ms ease, border-color 160ms ease, background 160ms ease;
}

.tavern-primary-button {
  border-color: color-mix(in srgb, var(--tavern-accent) 50%, transparent);
  background: linear-gradient(135deg, var(--tavern-accent), color-mix(in srgb, var(--tavern-accent) 72%, var(--tavern-accent-2)));
  color: #1a1208;
  box-shadow: 0 12px 28px color-mix(in srgb, var(--tavern-accent) 28%, transparent);
}

.tavern-primary-button--soft {
  background: color-mix(in srgb, var(--tavern-accent) 18%, transparent);
  color: var(--module-art-ink, var(--tavern-ink-strong));
  border-color: color-mix(in srgb, var(--tavern-accent) 34%, transparent);
  box-shadow: none;
}

.tavern-secondary-button {
  background: color-mix(in srgb, var(--tavern-card) 88%, transparent);
  color: var(--tavern-ink-strong);
  border-color: var(--tavern-line-strong);
}

.tavern-primary-button:hover,
.tavern-secondary-button:hover,
.tavern-script-card:hover {
  transform: translateY(-1px);
}

.tavern-primary-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
  transform: none;
}

.tavern-primary-button.full {
  width: 100%;
}

.tavern-primary-button.compact,
.tavern-secondary-button.compact {
  min-height: 34px;
}

.tavern-secondary-button.danger {
  border-color: color-mix(in srgb, var(--zc-danger) 28%, var(--tavern-line));
  color: var(--zc-danger);
}

.tavern-joined-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 34px;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--tavern-accent) 28%, var(--tavern-line));
  background: var(--tavern-soft);
  padding: 0 12px;
  color: var(--tavern-accent);
  font-size: 12px;
  font-weight: 950;
}

.tavern-filter-bar {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) repeat(2, minmax(150px, 190px));
}

@media (max-width: 760px) {
  .tavern-filter-bar {
    grid-template-columns: 1fr;
  }
}

.tavern-search {
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--tavern-line);
  border-radius: 12px;
  background: var(--tavern-raised);
  padding: 0 12px;
}

.tavern-search input {
  min-height: 42px;
  width: 100%;
  border: 0;
  background: transparent;
  outline: none;
  color: var(--tavern-ink-strong);
}

.tavern-select {
  min-height: 42px;
  background: var(--tavern-raised);
  padding: 0 12px;
  color: var(--tavern-ink-strong);
}

.tavern-workspace {
  display: grid;
  gap: 18px;
}

@media (min-width: 1120px) {
  .tavern-workspace {
    grid-template-columns: minmax(0, 1.1fr) minmax(340px, 0.9fr);
  }

  .tavern-workspace-mine {
    grid-template-columns: minmax(0, 0.95fr) minmax(360px, 0.75fr);
  }
}

.tavern-panel {
  padding: 18px;
  border: 1px solid var(--tavern-line);
  border-radius: 18px;
  background: var(--tavern-panel);
  box-shadow: var(--tavern-shadow);
}

.tavern-section-head {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  justify-content: space-between;
}

.tavern-section-head h2 {
  margin-top: 4px;
  font-size: 20px;
  font-weight: 950;
  color: var(--tavern-ink-strong);
}

.tavern-section-head > span {
  flex-shrink: 0;
  border-radius: 999px;
  border: 1px solid var(--tavern-line);
  background: var(--tavern-soft);
  padding: 6px 10px;
  color: var(--tavern-muted);
  font-size: 12px;
  font-weight: 900;
}

.tavern-script-list,
.tavern-row-list,
.tavern-room-grid {
  display: grid;
  gap: 12px;
  margin-top: 16px;
}

.tavern-room-grid {
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
}

.tavern-script-card,
.tavern-my-row,
.tavern-room-card,
.tavern-selected-script,
.tavern-selected-empty,
.tavern-empty {
  border: 1px solid var(--tavern-line);
  border-radius: 14px;
  background: var(--tavern-card);
  padding: 14px;
}

.tavern-script-card {
  display: grid;
  gap: 14px;
  cursor: pointer;
  transition: border-color 160ms ease, transform 160ms ease, background 160ms ease, box-shadow 160ms ease;
}

@media (min-width: 760px) {
  .tavern-script-card {
    grid-template-columns: minmax(0, 1fr) 120px;
  }
}

.tavern-script-card.selected {
  border-color: color-mix(in srgb, var(--tavern-accent) 42%, var(--tavern-line));
  background: color-mix(in srgb, var(--tavern-accent) 8%, var(--tavern-card));
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--tavern-accent) 18%, transparent);
}

.tavern-script-card h3,
.tavern-my-row h3,
.tavern-room-card h3,
.tavern-selected-script strong,
.tavern-empty h3 {
  margin-top: 8px;
  color: var(--tavern-ink-strong);
  font-size: 16px;
  font-weight: 950;
}

.tavern-script-card p,
.tavern-my-row p,
.tavern-my-row small,
.tavern-room-card p,
.tavern-selected-script p,
.tavern-selected-empty p,
.tavern-empty p {
  margin-top: 8px;
  color: var(--tavern-muted);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.65;
}

.tavern-script-meta {
  display: grid;
  align-content: start;
  gap: 6px;
  justify-items: start;
}

.tavern-script-meta strong {
  color: var(--tavern-accent);
  font-size: 20px;
  font-weight: 950;
}

.tavern-pill,
.tavern-tag {
  border-radius: 999px;
  border: 1px solid var(--tavern-line);
  background: color-mix(in srgb, var(--tavern-raised) 88%, transparent);
  padding: 5px 8px;
  color: var(--tavern-muted);
  font-size: 12px;
  font-weight: 900;
}

.tavern-pill.strong {
  background: var(--tavern-soft);
  color: var(--tavern-ink-strong);
}

.tavern-pill.muted {
  color: var(--tavern-accent);
}

.tavern-pill[data-status='listed'],
.tavern-pill[data-status='lobby'],
.tavern-pill[data-status='running'] {
  border-color: color-mix(in srgb, var(--tavern-accent) 28%, var(--tavern-line));
  background: var(--tavern-soft);
  color: var(--tavern-accent);
}

.tavern-pill[data-status='rejected'],
.tavern-pill[data-status='cancelled'] {
  border-color: color-mix(in srgb, var(--zc-danger) 28%, var(--tavern-line));
  color: var(--zc-danger);
}

.tavern-tags {
  margin-top: 12px;
}

.tavern-selected-script,
.tavern-selected-empty {
  margin-top: 16px;
}

.tavern-selected-grid {
  margin-top: 12px;
}

.tavern-selected-grid span {
  border-radius: 10px;
  border: 1px solid var(--tavern-line);
  background: var(--tavern-raised);
  padding: 8px 10px;
  color: var(--tavern-ink-strong);
  font-weight: 900;
}

.tavern-form {
  display: grid;
  gap: 12px;
  margin-top: 16px;
}

.tavern-form label {
  display: grid;
  gap: 7px;
  color: var(--tavern-muted);
  font-size: 13px;
  font-weight: 900;
}

.tavern-form input,
.tavern-form textarea,
.tavern-form select {
  width: 100%;
  background: var(--tavern-raised);
  padding: 10px 12px;
  color: var(--tavern-ink-strong);
  outline: none;
}

.tavern-form input:focus,
.tavern-form textarea:focus,
.tavern-form select:focus,
.tavern-search:focus-within,
.tavern-select:focus {
  border-color: color-mix(in srgb, var(--tavern-accent) 42%, var(--tavern-line));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--tavern-accent) 14%, transparent);
}

.tavern-form textarea {
  resize: vertical;
}

.tavern-form-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
}

.tavern-form-footer {
  align-items: center;
  justify-content: flex-end;
  border-top: 1px solid var(--tavern-line);
  padding-top: 14px;
}

.tavern-radio {
  display: inline-flex !important;
  grid-auto-flow: column;
  align-items: center;
  width: auto;
}

.tavern-radio input {
  width: auto;
}

.tavern-my-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  justify-content: space-between;
}

.tavern-my-row-main {
  min-width: 0;
}

.tavern-my-row-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
}

.tavern-package-inline {
  grid-column: 1 / -1;
  display: grid;
  gap: 14px;
  border-top: 1px solid var(--tavern-line);
  padding-top: 14px;
}

.tavern-package-head,
.tavern-package-form-footer,
.tavern-package-versions article {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.tavern-package-head small,
.tavern-package-muted,
.tavern-package-form-footer small {
  color: var(--tavern-muted);
  line-height: 1.6;
}

.tavern-package-muted {
  font-size: 13px;
}

.tavern-package-versions {
  display: grid;
  gap: 8px;
}

.tavern-package-versions article {
  border: 1px solid var(--tavern-line);
  background: var(--tavern-raised);
  padding: 10px 12px;
}

.tavern-package-versions article > div:first-child {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 8px;
}

.tavern-package-versions small {
  color: var(--tavern-muted);
}

.tavern-package-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.tavern-package-form {
  display: grid;
  gap: 12px;
  border: 1px solid var(--tavern-line);
  background: color-mix(in srgb, var(--tavern-panel) 82%, var(--tavern-soft));
  padding: 14px;
}

.tavern-package-permissions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 16px;
  margin: 0;
  border: 1px solid var(--tavern-line);
  padding: 10px 12px;
}

.tavern-package-permissions legend {
  padding: 0 6px;
  color: var(--tavern-muted);
  font-size: 12px;
}

.tavern-package-permissions label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.tavern-package-permissions input {
  width: auto;
}

.tavern-date {
  flex-shrink: 0;
}

.tavern-room-card {
  display: grid;
  gap: 6px;
}

.tavern-room-meta {
  margin-top: 8px;
}

.tavern-room-meta span {
  border-radius: 10px;
  border: 1px solid var(--tavern-line);
  background: var(--tavern-raised);
  padding: 7px 9px;
}

.tavern-empty {
  display: grid;
  justify-items: start;
  gap: 8px;
  margin-top: 16px;
}

.tavern-empty.compact {
  margin-top: 16px;
}

.tavern-loading-card span,
.tavern-loading-card strong,
.tavern-loading-card p {
  display: block;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    color-mix(in srgb, var(--tavern-line) 55%, transparent),
    color-mix(in srgb, var(--tavern-accent) 12%, transparent),
    color-mix(in srgb, var(--tavern-line) 55%, transparent)
  );
  min-height: 14px;
}

.tavern-loading-card strong {
  width: 58%;
  height: 20px;
}

.tavern-loading-card p {
  width: 86%;
  height: 14px;
}

.animate-spin {
  animation: spin 0.9s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .tavern-primary-button,
  .tavern-secondary-button,
  .tavern-script-card,
  .tavern-chat-entry,
  .animate-spin {
    transition: none;
    animation: none;
    transform: none;
  }
}

@media (max-width: 720px) {
  .tavern-page-head, .tavern-local-heading { align-items: flex-start; flex-direction: column; }
  .tavern-page-head .tavern-hero-actions { width: 100%; }
  .tavern-page-head .tavern-secondary-button { flex: 1; }
  .tavern-my-row { grid-template-columns: minmax(0, 1fr); }
  .tavern-my-row-actions { align-items: flex-start; flex-direction: row; flex-wrap: wrap; justify-content: space-between; }
  .tavern-package-versions article, .tavern-package-head, .tavern-package-form-footer { align-items: flex-start; flex-direction: column; }
}
</style>
