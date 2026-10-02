import { useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ApiError,
  api,
  canWrite,
  discoveredMetrics,
  errorText,
  fieldErrors,
  projectMetrics,
  type DiscoveredMetric,
  type MetricDefinition,
} from '../api/client'
import { formatWhen, suggestMetricName } from '../format'
import { liveInterval } from '../live'
import { useNow } from '../useNow'
import { useSession } from '../useSession'

const dataTypes = [
  { id: 'number', label: 'Number' },
  { id: 'boolean', label: 'Boolean' },
  { id: 'string', label: 'Text' },
] as const

const displays: Record<string, { id: string; label: string }[]> = {
  number: [
    { id: 'number', label: 'Number' },
    { id: 'line', label: 'Line chart' },
    { id: 'gauge', label: 'Gauge' },
  ],
  boolean: [
    { id: 'boolean', label: 'On / off' },
    { id: 'status', label: 'Status' },
  ],
  string: [{ id: 'text', label: 'Text' }],
}

type Draft = {
  key: string
  name: string
  description: string
  dataType: string
  unit: string
  displayType: string
  minValue: string
  maxValue: string
  definitionId: string | null
}

export function MetricsPage() {
  const { projectId = '' } = useParams()
  const now = useNow()
  const { projects, teams } = useSession()
  const project = projects.find((item) => item.id === projectId)
  const role = teams.find((team) => team.id === project?.teamId)?.role
  const write = canWrite(role)
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<Draft | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const [saved, setSaved] = useState(false)

  const definitions = useQuery({
    queryKey: ['metrics', projectId],
    queryFn: () => projectMetrics(projectId),
    refetchInterval: liveInterval(),
  })
  const discovered = useQuery({
    queryKey: ['metrics-discovered', projectId],
    queryFn: () => discoveredMetrics(projectId),
    refetchInterval: liveInterval(),
  })

  if (definitions.isLoading || discovered.isLoading) {
    return (
      <section className="page">
        <p>Loading…</p>
      </section>
    )
  }
  if (definitions.isError || discovered.isError) {
    return (
      <section className="page">
        <p className="error">{errorText(definitions.error ?? discovered.error)}</p>
      </section>
    )
  }

  const items = mergeRows(definitions.data?.metrics ?? [], discovered.data?.metrics ?? [])
  const fields = fieldErrors(error)

  function openDiscovered(row: Row) {
    const definition = row.definition
    setSaved(false)
    setError(null)
    setDraft(
      definition
        ? {
            key: definition.key,
            name: definition.name,
            description: definition.description,
            dataType: definition.dataType,
            unit: definition.unit,
            displayType: definition.displayType,
            minValue: definition.minValue == null ? '' : String(definition.minValue),
            maxValue: definition.maxValue == null ? '' : String(definition.maxValue),
            definitionId: definition.id,
          }
        : {
            key: row.key,
            name: suggestMetricName(row.key),
            description: '',
            dataType: row.dataType,
            unit: '',
            displayType: defaultDisplay(row.dataType),
            minValue: '',
            maxValue: '',
            definitionId: null,
          },
    )
  }

  function openNew() {
    setSaved(false)
    setError(null)
    setDraft({
      key: '',
      name: '',
      description: '',
      dataType: 'number',
      unit: '',
      displayType: 'number',
      minValue: '',
      maxValue: '',
      definitionId: null,
    })
  }

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!draft) return
    setPending(true)
    setSaved(false)
    setError(null)
    if (draft.dataType === 'number') {
      if (draft.minValue.trim() && blankNumber(draft.minValue) == null) {
        setError(new ApiError(400, 'VALIDATION_ERROR', 'Min must be a number'))
        setPending(false)
        return
      }
      if (draft.maxValue.trim() && blankNumber(draft.maxValue) == null) {
        setError(new ApiError(400, 'VALIDATION_ERROR', 'Max must be a number'))
        setPending(false)
        return
      }
    }
    const body = {
      key: draft.key.trim(),
      name: draft.name.trim(),
      description: draft.description.trim(),
      dataType: draft.dataType,
      unit: draft.unit.trim(),
      displayType: draft.displayType,
      minValue: draft.dataType === 'number' ? blankNumber(draft.minValue) : null,
      maxValue: draft.dataType === 'number' ? blankNumber(draft.maxValue) : null,
    }
    try {
      if (draft.definitionId) {
        await api(`/api/v1/metrics/${draft.definitionId}`, { method: 'PATCH', body: JSON.stringify(body) })
      } else {
        const created = await api<{ metric: MetricDefinition }>(`/api/v1/projects/${projectId}/metrics`, {
          method: 'POST',
          body: JSON.stringify(body),
        })
        setDraft({ ...draft, definitionId: created.metric.id, key: created.metric.key })
      }
      await refresh(queryClient, projectId)
      setSaved(true)
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  async function remove() {
    if (!draft?.definitionId) return
    if (!window.confirm(`Remove the definition for ${draft.key}? Telemetry for this key is kept.`)) return
    setError(null)
    try {
      await api(`/api/v1/metrics/${draft.definitionId}`, { method: 'DELETE' })
      setDraft(null)
      setSaved(false)
      await refresh(queryClient, projectId)
    } catch (err) {
      setError(err)
    }
  }

  return (
    <section className="page">
      <p className="eyebrow">Metrics</p>
      <h1>Metrics</h1>
      <p className="lede">Name a telemetry key and choose how the dashboard draws it. Devices can send data before you do this.</p>
      {items.length === 0 && !draft ? (
        <div className="card">
          <p>No telemetry metrics discovered yet.</p>
          <p>Once your ESP32 sends telemetry, metrics will appear here automatically.</p>
        </div>
      ) : null}
      {write ? (
        <div className="row">
          <button type="button" className="secondary" onClick={openNew}>
            Define a metric
          </button>
        </div>
      ) : null}
      {items.length > 0 ? (
        <table>
          <thead>
            <tr>
              <th>Key</th>
              <th>Name</th>
              <th>Type</th>
              <th>Status</th>
              <th>Last seen</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {items.map((row) => (
              <tr key={row.key} className={draft?.key === row.key ? 'selected' : undefined}>
                <td className="secret-value">{row.key}</td>
                <td>{row.definition?.name ?? '—'}</td>
                <td>{dataTypes.find((item) => item.id === row.dataType)?.label ?? row.dataType}</td>
                <td>{row.configured ? 'Configured' : 'Unconfigured'}</td>
                <td>{formatWhen(row.lastSeenAt, now)}</td>
                <td>
                  {write ? (
                    <button type="button" className="secondary small" onClick={() => openDiscovered(row)}>
                      {row.configured ? 'Edit' : 'Configure'}
                    </button>
                  ) : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      {draft && write ? (
        <form className="form" onSubmit={(event) => void save(event)}>
          <h2>{draft.definitionId ? draft.key : 'New metric'}</h2>
          {error ? <p className="error">{errorText(error)}</p> : null}
          {saved ? <p className="muted">Saved. The overview uses this display.</p> : null}
          {draft.definitionId ? (
            <p>
              Key <span className="secret-value">{draft.key}</span>
            </p>
          ) : (
            <label>
              Key
              <input
                value={draft.key}
                onChange={(event) => setDraft({ ...draft, key: event.target.value })}
                required
                spellCheck={false}
              />
              {fields.key ? <span className="field-error">{fields.key}</span> : null}
            </label>
          )}
          <label>
            Display name
            <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} required />
            {fields.name ? <span className="field-error">{fields.name}</span> : null}
          </label>
          <label>
            Unit
            <input value={draft.unit} onChange={(event) => setDraft({ ...draft, unit: event.target.value })} placeholder="°C" />
            {fields.unit ? <span className="field-error">{fields.unit}</span> : null}
          </label>
          <label>
            Data type
            <select
              value={draft.dataType}
              onChange={(event) => {
                const dataType = event.target.value
                const options = displays[dataType] ?? displays.number
                const displayType = options.some((item) => item.id === draft.displayType) ? draft.displayType : options[0].id
                setDraft({ ...draft, dataType, displayType })
              }}
            >
              {dataTypes.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.label}
                </option>
              ))}
            </select>
            {fields.dataType ? <span className="field-error">{fields.dataType}</span> : null}
          </label>
          <label>
            Display type
            <select value={draft.displayType} onChange={(event) => setDraft({ ...draft, displayType: event.target.value })}>
              {(displays[draft.dataType] ?? []).map((item) => (
                <option key={item.id} value={item.id}>
                  {item.label}
                </option>
              ))}
            </select>
            {fields.displayType ? <span className="field-error">{fields.displayType}</span> : null}
          </label>
          {draft.dataType === 'number' ? (
            <>
              <label>
                Min
                <input
                  value={draft.minValue}
                  onChange={(event) => setDraft({ ...draft, minValue: event.target.value })}
                  inputMode="decimal"
                />
                {fields.minValue ? <span className="field-error">{fields.minValue}</span> : null}
              </label>
              <label>
                Max
                <input
                  value={draft.maxValue}
                  onChange={(event) => setDraft({ ...draft, maxValue: event.target.value })}
                  inputMode="decimal"
                />
                {fields.maxValue ? <span className="field-error">{fields.maxValue}</span> : null}
              </label>
            </>
          ) : null}
          <label>
            Description
            <textarea
              value={draft.description}
              onChange={(event) => setDraft({ ...draft, description: event.target.value })}
            />
            {fields.description ? <span className="field-error">{fields.description}</span> : null}
          </label>
          <div className="row">
            <button type="submit" disabled={pending}>
              Save
            </button>
            {draft.definitionId ? (
              <button type="button" className="danger" onClick={() => void remove()}>
                Remove definition
              </button>
            ) : (
              <button type="button" className="secondary" onClick={() => setDraft(null)}>
                Cancel
              </button>
            )}
          </div>
        </form>
      ) : null}
      {!write && items.length > 0 ? <p className="muted">You can view metrics. An editor on the team can change them.</p> : null}
    </section>
  )
}

type Row = {
  key: string
  dataType: string
  configured: boolean
  lastSeenAt: string | null
  definition?: MetricDefinition
}

function mergeRows(definitions: MetricDefinition[], discovered: DiscoveredMetric[]): Row[] {
  const byKey = new Map<string, Row>()
  for (const item of discovered) {
    byKey.set(item.key, {
      key: item.key,
      dataType: item.dataType,
      configured: false,
      lastSeenAt: item.lastSeenAt,
    })
  }
  for (const item of definitions) {
    byKey.set(item.key, {
      key: item.key,
      dataType: item.dataType,
      configured: true,
      lastSeenAt: item.lastSeenAt,
      definition: item,
    })
  }
  return [...byKey.values()].sort((a, b) => a.key.localeCompare(b.key))
}

function defaultDisplay(dataType: string) {
  if (dataType === 'boolean') return 'boolean'
  if (dataType === 'string') return 'text'
  return 'number'
}

function blankNumber(value: string): number | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : null
}

async function refresh(queryClient: ReturnType<typeof useQueryClient>, projectId: string) {
  await queryClient.invalidateQueries({ queryKey: ['metrics', projectId] })
  await queryClient.invalidateQueries({ queryKey: ['metrics-discovered', projectId] })
}
