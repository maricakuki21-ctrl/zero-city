const ROOT = '/biz/workbench'

function encoded(value: string): string {
  return encodeURIComponent(value)
}

export const WORKBENCH_ENDPOINTS = {
  workspace: `${ROOT}/workspace`,
  runs: `${ROOT}/runs`,
  run: (runId: string) => `${ROOT}/runs/${encoded(runId)}`,
  cancel: (runId: string) => `${ROOT}/runs/${encoded(runId)}/cancel`,
  save: (runId: string) => `${ROOT}/runs/${encoded(runId)}/save`,
  fork: (runId: string) => `${ROOT}/runs/${encoded(runId)}/fork`,
  replay: (snapshotId: string) => `${ROOT}/replays/${encoded(snapshotId)}/run`,
  events: (runId: string, cursor: string) => {
    const path = `${ROOT}/runs/${encoded(runId)}/events`
    return cursor ? `${path}?cursor=${encoded(cursor)}` : path
  },
  artifactMetadata: (runId: string, artifactId: string) => `${ROOT}/runs/${encoded(runId)}/artifacts/${encoded(artifactId)}`,
  artifactContent: (runId: string, artifactId: string) => `${ROOT}/runs/${encoded(runId)}/artifacts/${encoded(artifactId)}/content`,
} as const
