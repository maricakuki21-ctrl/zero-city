import { readFile } from 'node:fs/promises'
const text = await readFile('/data/bizdecipher/TEST-CREDENTIALS.txt', 'utf8')
const email = text.match(/^Admin email:\s*(.+)$/m)?.[1].trim()
const password = text.match(/^Admin password:\s*(.+)$/m)?.[1].trim()
const response = await fetch('http://127.0.0.1:18081/api/v1/auth/login', {
  method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password }),
})
const result = await response.json()
if (!result.data?.access_token) throw new Error('Test login unavailable')
// Consumed only by the QA parent process; never print this output in reports.
process.stdout.write(JSON.stringify(result.data))
