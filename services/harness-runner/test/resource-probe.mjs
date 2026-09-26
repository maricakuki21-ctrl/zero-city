import { readFile } from 'node:fs/promises'
import { randomUUID } from 'node:crypto'
const text = await readFile('/data/bizdecipher/TEST-CREDENTIALS.txt', 'utf8')
const email = text.match(/^Admin email:\s*(.+)$/m)?.[1].trim()
const password = text.match(/^Admin password:\s*(.+)$/m)?.[1].trim()
const base = 'http://127.0.0.1:18081/api/v1'
const login = await (await fetch(`${base}/auth/login`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password }) })).json()
if (!login.data?.access_token) throw new Error('Login unavailable')
const headers = { authorization: `Bearer ${login.data.access_token}` }
for (const path of ['/groups/available', '/admin/groups?page=1&page_size=100', '/biz/my-pools']) {
  const response = await fetch(base + path, { headers })
  const body = await response.json()
  const items = Array.isArray(body.data) ? body.data : body.data?.items || body.data?.pools || []
  console.log(JSON.stringify({ path, status: response.status, items: items.map(item => ({
    id: item.id, name: item.name, platform: item.platform, status: item.status,
    account_mode_enabled: item.account_mode_enabled, model_count: item.models?.length,
    native_readiness_status: item.native_readiness_status,
  })) }))
}
if (process.env.PROVISION_GROUP_ID) {
  const groupId = Number(process.env.PROVISION_GROUP_ID)
  const current = await (await fetch(`${base}/keys?page=1&page_size=100`, { headers })).json()
  let key = current.data?.items?.find(item => item.group_id === groupId && item.status === 'active')
  if (!key) {
    const response = await fetch(`${base}/keys`, {
      method: 'POST', headers: { ...headers, 'content-type': 'application/json' },
      body: JSON.stringify({ name: '工作台 · 测试分组', group_id: groupId }),
    })
    const result = await response.json()
    if (!response.ok || !result.data?.key) throw new Error(`Key creation failed (${response.status})`)
    key = result.data
  }
  const response = await fetch('http://127.0.0.1:18081/v1/models', { headers: { authorization: `Bearer ${key.key}` } })
  const result = await response.json()
  console.log(JSON.stringify({ keyId: key.id, groupId, catalogStatus: response.status, models: result.data?.map(item => item.id), errorType: result.error?.type }))
  for (const body of [{ customKey: key.key }, { customKey: 'invalid-resource-key' }]) {
    const custom = await fetch(`${base}/biz/harness/models`, {
      method: 'POST', headers: { ...headers, 'content-type': 'application/json' }, body: JSON.stringify(body),
    })
    const catalog = await custom.json()
    console.log(JSON.stringify({ customCatalogValidKey: body.customKey === key.key, status: custom.status, modelCount: catalog.models?.length ?? catalog.data?.models?.length }))
  }
}
if (process.env.CHECK_POOL_CREATE === '1') {
  const response = await fetch(`${base}/biz/pools`, {
    method: 'POST', headers: { ...headers, 'content-type': 'application/json' },
    body: JSON.stringify({ supply_mode: 'native', operation_id: randomUUID(), name: '接口诊断草稿', description: '未上架的创建检查，检查后归档' }),
  })
  const result = await response.json()
  console.log(JSON.stringify({ poolCreateStatus: response.status, id: result.data?.id, reason: result.reason, message: result.message }))
  if (response.ok && result.data?.id) {
    const cleanup = await fetch(`${base}/biz/pools/${result.data.id}`, { method: 'DELETE', headers })
    console.log(JSON.stringify({ diagnosticPoolArchiveStatus: cleanup.status }))
  }
}
