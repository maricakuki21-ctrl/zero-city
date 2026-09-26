let generation = 0

export function getAuthGeneration(): number {
  return generation
}

// Only account/session boundaries advance this value, never token rotation.
export function advanceAuthGeneration(): number {
  return ++generation
}

export function staleAuthSessionError(): Error {
  return Object.assign(new Error('Authentication session changed'), { code: 'AUTH_SESSION_CHANGED' })
}
