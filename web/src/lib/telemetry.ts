export type TelemetryStatus = {
  enabled: boolean
}

export async function fetchTelemetryStatus(): Promise<TelemetryStatus> {
  const res = await fetch('/api/admin/telemetry', { credentials: 'include' })
  if (!res.ok) throw new Error('Failed to load telemetry status')
  return res.json()
}

export async function setTelemetryEnabled(enabled: boolean): Promise<TelemetryStatus> {
  const res = await fetch('/api/admin/telemetry', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Failed to update telemetry setting')
  }
  return res.json()
}
