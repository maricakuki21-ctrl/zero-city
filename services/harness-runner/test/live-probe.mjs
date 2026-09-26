import { readFile } from 'node:fs/promises'
const base = 'http://127.0.0.1:18081'
const text = await readFile('/data/bizdecipher/TEST-CREDENTIALS.txt', 'utf8')
const email = text.match(/^Admin email:\s*(.+)$/m)?.[1].trim()
const password = text.match(/^Admin password:\s*(.+)$/m)?.[1].trim()
if (!email || !password) throw new Error('Test login configuration missing')
const response = await fetch(`${base}/api/v1/auth/login`, {
  method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password }),
})
const login = await response.json()
const token = login.data?.access_token
if (!token) throw new Error(`Test login failed (${response.status})`)
const headers = { authorization: `Bearer ${token}` }
const keysResponse = await fetch(`${base}/api/v1/keys?page=1&page_size=100`, { headers })
const keys = (await keysResponse.json()).data?.items || []
console.log(JSON.stringify({ login: true, keys: keys.map(key => ({ id: key.id, name: key.name, status: key.status, group: key.group?.name, platform: key.group?.platform })) }))
for (const key of keys.filter(key => key.status === 'active').slice(0, 5)) {
  const modelsResponse = await fetch(`${base}/v1/models`, { headers: { authorization: `Bearer ${key.key}` } })
  const models = await modelsResponse.json()
  console.log(JSON.stringify({ keyId: key.id, modelStatus: modelsResponse.status, models: models.data?.map(model => model.id).slice(0, 25) }))
}
