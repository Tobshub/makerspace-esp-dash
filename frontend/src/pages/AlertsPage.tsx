import { useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  api,
  canWrite,
  errorText,
  fieldErrors,
  projectAlertEvents,
  projectAlerts,
  projectDevices,
  type AlertRule,
} from '../api/client'
import { formatWhen } from '../format'
import { liveInterval } from '../live'
import { useNow } from '../useNow'
import { useSession } from '../useSession'

export function AlertsPage() {
  const { projectId = '' } = useParams()
  const now = useNow()
  const { projects, teams } = useSession()
  const project = projects.find((item) => item.id === projectId)
  const write = canWrite(teams.find((team) => team.id === project?.teamId)?.role)
  const queryClient = useQueryClient()
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const [ruleType, setRuleType] = useState('metric_threshold')
  const rules = useQuery({
    queryKey: ['alerts', projectId],
    queryFn: () => projectAlerts(projectId),
    refetchInterval: liveInterval(),
  })
  const events = useQuery({
    queryKey: ['alert-events', projectId],
    queryFn: () => projectAlertEvents(projectId),
    refetchInterval: liveInterval(),
  })
  const devices = useQuery({
    queryKey: ['devices', projectId],
    queryFn: () => projectDevices(projectId),
  })

  if (rules.isLoading || events.isLoading) {
    return (
      <section className="page">
        <p>Loading…</p>
      </section>
    )
  }
  if (rules.isError || events.isError) {
    return (
      <section className="page">
        <p className="error">{errorText(rules.error ?? events.error)}</p>
      </section>
    )
  }

  const fields = fieldErrors(error)
  const active = events.data?.events.filter((event) => event.status === 'triggered') ?? []

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    const formElement = event.currentTarget
    const form = new FormData(formElement)
    const type = String(form.get('ruleType'))
    const body: Record<string, unknown> = {
      name: String(form.get('name')).trim(),
      ruleType: type,
      deviceId: String(form.get('deviceId') || ''),
      durationSeconds: Number(form.get('durationSeconds') || 0),
      enabled: true,
    }
    if (type === 'metric_threshold') {
      body.metricKey = String(form.get('metricKey')).trim()
      body.operator = String(form.get('operator'))
      body.thresholdValue = Number(form.get('thresholdValue'))
    }
    setPending(true)
    try {
      await api(`/api/v1/projects/${projectId}/alerts`, { method: 'POST', body: JSON.stringify(body) })
      formElement.reset()
      setRuleType('metric_threshold')
      await queryClient.invalidateQueries({ queryKey: ['alerts', projectId] })
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  return (
    <section className="page">
      <p className="eyebrow">{project?.name ?? 'Project'}</p>
      <h1>Alerts</h1>
      <p className="lede">A threshold or an offline device opens an alert here, then closes when the condition clears.</p>
      <h2>Active</h2>
      {active.length === 0 ? <p className="muted">No active alerts.</p> : null}
      {active.length > 0 ? (
        <ul className="metric-list">
          {active.map((event) => (
            <li key={event.id}>
              <span>
                <span className="pill triggered">Triggered</span> {event.message}
              </span>
              <span className="muted">{formatWhen(event.triggeredAt, now)}</span>
            </li>
          ))}
        </ul>
      ) : null}
      <h2>Rules</h2>
      {(rules.data?.alerts.length ?? 0) === 0 ? <p className="muted">No alert rules yet.</p> : null}
      {rules.data?.alerts.map((rule) => (
        <RuleRow
          key={rule.id}
          rule={rule}
          write={write}
          onChanged={() => {
            void queryClient.invalidateQueries({ queryKey: ['alerts', projectId] })
            void queryClient.invalidateQueries({ queryKey: ['alert-events', projectId] })
          }}
        />
      ))}
      {write ? (
        <form className="form" onSubmit={(event) => void create(event)}>
          <h2>New rule</h2>
          <label>
            Name
            <input name="name" required />
            {fields.name ? <span className="error">{fields.name}</span> : null}
          </label>
          <label>
            Type
            <select name="ruleType" value={ruleType} onChange={(event) => setRuleType(event.target.value)}>
              <option value="metric_threshold">Metric threshold</option>
              <option value="device_offline">Device offline</option>
            </select>
          </label>
          <label>
            Device
            <select name="deviceId" defaultValue="">
              <option value="">Any device</option>
              {devices.data?.devices.map((device) => (
                <option key={device.id} value={device.id}>
                  {device.name}
                </option>
              ))}
            </select>
          </label>
          {ruleType === 'metric_threshold' ? (
            <>
              <label>
                Metric key
                <input name="metricKey" required pattern="[A-Za-z][A-Za-z0-9_]*" placeholder="temperature" />
                {fields.metricKey ? <span className="error">{fields.metricKey}</span> : null}
              </label>
              <label>
                Operator
                <select name="operator" defaultValue=">">
                  {['>', '>=', '<', '<=', '==', '!='].map((op) => (
                    <option key={op} value={op}>
                      {op}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Threshold
                <input name="thresholdValue" type="number" step="any" required />
                {fields.thresholdValue ? <span className="error">{fields.thresholdValue}</span> : null}
              </label>
            </>
          ) : null}
          <label>
            Duration in seconds
            <input name="durationSeconds" type="number" min={0} defaultValue={0} />
            <span className="muted">0 alerts immediately. A longer duration waits out a brief spike.</span>
          </label>
          {error ? <p className="error">{errorText(error)}</p> : null}
          <button type="submit" disabled={pending}>
            {pending ? 'Saving…' : 'Add alert'}
          </button>
        </form>
      ) : null}
      <h2>History</h2>
      {(events.data?.events.length ?? 0) === 0 ? <p className="muted">Alerts that trigger and resolve show up here.</p> : null}
      {events.data && events.data.events.length > 0 ? (
        <ul className="metric-list">
          {events.data.events.map((event) => (
            <li key={event.id}>
              <span>
                <span className={`pill ${event.status}`}>{event.status}</span> {event.message}
              </span>
              <span className="muted">
                {formatWhen(event.triggeredAt, now)}
                {event.resolvedAt ? ` · resolved ${formatWhen(event.resolvedAt, now)}` : ''}
              </span>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  )
}

function RuleRow({ rule, write, onChanged }: { rule: AlertRule; write: boolean; onChanged: () => void }) {
  const [error, setError] = useState<unknown>(null)
  const detail =
    rule.ruleType === 'device_offline'
      ? `offline for ${rule.durationSeconds}s`
      : `${rule.metricKey} ${rule.operator} ${rule.thresholdValue} for ${rule.durationSeconds}s`

  async function toggle() {
    setError(null)
    try {
      await api(`/api/v1/alerts/${rule.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          name: rule.name,
          ruleType: rule.ruleType,
          deviceId: rule.deviceId,
          metricKey: rule.metricKey,
          operator: rule.operator,
          thresholdValue: rule.thresholdValue,
          durationSeconds: rule.durationSeconds,
          enabled: !rule.enabled,
        }),
      })
      onChanged()
    } catch (err) {
      setError(err)
    }
  }

  async function remove() {
    setError(null)
    try {
      await api(`/api/v1/alerts/${rule.id}`, { method: 'DELETE' })
      onChanged()
    } catch (err) {
      setError(err)
    }
  }

  return (
    <div className="card">
      <p>
        <strong>{rule.name}</strong> {rule.active ? <span className="pill triggered">Triggered</span> : null}{' '}
        {!rule.enabled ? <span className="pill disabled">Disabled</span> : null}
      </p>
      <p className="muted">{detail}</p>
      {error ? <p className="error">{errorText(error)}</p> : null}
      {write ? (
        <div className="row">
          <button type="button" className="secondary small" onClick={() => void toggle()}>
            {rule.enabled ? 'Disable' : 'Enable'}
          </button>
          <button type="button" className="danger small" onClick={() => void remove()}>
            Delete
          </button>
        </div>
      ) : null}
    </div>
  )
}
