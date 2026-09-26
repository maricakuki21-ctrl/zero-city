export interface ResourceChoice {
  kind: 'official' | 'shared'
  id: number
  name: string
  ready: boolean
}
