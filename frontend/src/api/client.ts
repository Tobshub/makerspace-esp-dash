export type User = {
  id: string
  email: string
  name: string
}

export type Team = {
  id: string
  name: string
  slug: string
  role: string
  createdAt: string
  updatedAt: string
}

export type Project = {
  id: string
  teamId: string
  name: string
  slug: string
  description: string
  createdAt: string
  updatedAt: string
}

export type Member = {
  id: string
  userId: string
  email: string
  name: string
  role: string
  createdAt: string
}

export type Device = {
  id: string
  projectId: string
  name: string
  description: string
  deviceKey: string
  status: string
  lastSeenAt: string | null
  firmwareVersion: string
  createdAt: string
  updatedAt: string
}

export type DeviceConnection = {
  host: string
  port: number
  username: string
  password: string
  telemetryTopic: string
  stateTopic: string
  commandTopic: string
  ackTopic: string
  eventsTopic: string
  statusTopic: string
}

export type IssuedDevice = {
  device: Device
  credentials: { username: string; secret: string }
  connection: DeviceConnection
  firmwareSnippet: string
}

export type TelemetryValue = {
  metric: string
  numericValue: number | null
  booleanValue: boolean | null
  stringValue: string | null
  recordedAt: string
  receivedAt: string
}

export type TelemetryPoint = {
  numericValue: number | null
  booleanValue: boolean | null
  stringValue: string | null
  recordedAt: string
  receivedAt: string
}

export type DeviceTelemetryLatest = {
  deviceId: string
  metrics: TelemetryValue[]
}

export type TelemetrySeries = {
  deviceId: string
  metric: string
  resolution: string
  from: string
  to: string
  limit: number
  truncated: boolean
  points: TelemetryPoint[]
}

export type ProjectTelemetryLatest = {
  projectId: string
  devices: {
    deviceId: string
    deviceKey: string
    name: string
    metrics: TelemetryValue[]
  }[]
}

export type BrokerInfo = {
  host: string
  port: number
  tls: boolean
  offlineTimeoutSeconds: number
  anonymousLocal: boolean
  topics: Record<string, string>
}

const TOKEN_KEY = 'makerspace.token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  status: number
  code: string
  fields?: Record<string, string>

  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }
}

export function errorText(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof TypeError) return 'API unavailable'
  return 'Something went wrong'
}

export function fieldErrors(err: unknown): Record<string, string> {
  if (err instanceof ApiError && err.fields) return err.fields
  return {}
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  let response: Response
  try {
    response = await fetch(path, { ...init, headers })
  } catch (err) {
    if (err instanceof TypeError) {
      throw new ApiError(0, 'UNAVAILABLE', 'API unavailable')
    }
    throw err
  }

  if (response.status === 204) return undefined as T
  const text = await response.text()
  const data = text ? (JSON.parse(text) as unknown) : null
  if (!response.ok) {
    const body = data as {
      error?: { code?: string; message?: string; fields?: Record<string, string> }
    } | null
    throw new ApiError(
      response.status,
      body?.error?.code ?? 'INTERNAL',
      body?.error?.message ?? 'Something went wrong',
      body?.error?.fields,
    )
  }
  return data as T
}

export function deviceTelemetryLatest(deviceId: string) {
  return api<DeviceTelemetryLatest>(`/api/v1/devices/${deviceId}/telemetry/latest`)
}

export function deviceTelemetry(
  deviceId: string,
  params: { metric: string; from?: string; to?: string; limit?: number; resolution?: string },
) {
  const query = new URLSearchParams({ metric: params.metric })
  if (params.from) query.set('from', params.from)
  if (params.to) query.set('to', params.to)
  if (params.limit != null) query.set('limit', String(params.limit))
  if (params.resolution) query.set('resolution', params.resolution)
  return api<TelemetrySeries>(`/api/v1/devices/${deviceId}/telemetry?${query}`)
}

export function projectTelemetryLatest(projectId: string) {
  return api<ProjectTelemetryLatest>(`/api/v1/projects/${projectId}/telemetry/latest`)
}

export function formatTelemetry(value: Pick<TelemetryValue, 'numericValue' | 'booleanValue' | 'stringValue'>): string {
  if (value.numericValue != null) return String(value.numericValue)
  if (value.booleanValue != null) return value.booleanValue ? 'true' : 'false'
  if (value.stringValue != null) return value.stringValue
  return '—'
}
