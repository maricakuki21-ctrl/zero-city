export interface ResourceMetric {
  label: string
  value: string
  note?: string
  tone?: 'good' | 'watch' | 'danger' | 'muted'
}
