import { useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ApiError,
  api,
  canWrite,
  errorText,
  fieldErrors,
  projectControls,
  projectDevices,
  type ControlDefinition,
  type DeviceCommand,
} from '../api/client'
import { commandStatusLabel } from '../format'
import { liveInterval } from '../live'
import { useSession } from '../useSession'

const types = [
  { id: 'button', label: 'Button' },
  { id: 'toggle', label: 'Toggle' },
  { id: 'slider', label: 'Slider' },
] as const

export function ControlsPage() {
  const { projectId = '' } = useParams()
  const { projects, teams } = useSession()
  const project = projects.find((item) => item.id === projectId)
  const write = canWrite(teams.find((team) => team.id === project?.teamId)?.role)
  const queryClient = useQueryClient()
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const controls = useQuery({
    queryKey: ['controls', projectId],
    queryFn: () => projectControls(projectId),
    refetchInterval: liveInterval(),
  })
  const devices = useQuery({
    queryKey: ['devices', projectId],
    queryFn: () => projectDevices(projectId),
    refetchInterval: liveInterval(),
  })

  if (controls.isLoading || devices.isLoading) {
    return (
      <section className="page">
        <p>Loading…</p>
      </section>
    )
  }
  if (controls.isError || devices.isError) {
    return (
      <section className="page">
        <p className="error">{errorText(controls.error ?? devices.error)}</p>
      </section>
    )
  }

  const fields = fieldErrors(error)
  const deviceList = devices.data?.devices ?? []

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    const formElement = event.currentTarget
    const form = new FormData(formElement)
    const controlType = String(form.get('controlType'))
    const body: Record<string, unknown> = {
      key: String(form.get('key')).trim(),
      name: String(form.get('name')).trim(),
      controlType,
      command: String(form.get('command')).trim(),
      configuration: configurationFrom(controlType, form),
    }
    setPending(true)
    try {
      await api(`/api/v1/projects/${projectId}/controls`, { method: 'POST', body: JSON.stringify(body) })
      formElement.reset()
      await queryClient.invalidateQueries({ queryKey: ['controls', projectId] })
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  return (
    <section className="page">
      <p className="eyebrow">{project?.name ?? 'Project'}</p>
      <h1>Controls</h1>
      <p className="lede">A control sends a command to a device. The status shows whether the device acknowledged it.</p>
      {deviceList.length === 0 ? <p className="muted">Add a device before sending a command.</p> : null}
      {(controls.data?.controls.length ?? 0) === 0 ? (
        <p className="muted">No controls yet. Add a button, toggle, or slider.</p>
      ) : (
        <div className="card-grid">
          {controls.data?.controls.map((control) => (
            <ControlCard
              key={control.id}
              control={control}
              devices={deviceList}
              write={write}
              onChanged={() => void queryClient.invalidateQueries({ queryKey: ['controls', projectId] })}
            />
          ))}
        </div>
      )}
      {write ? (
        <form className="form" onSubmit={(event) => void create(event)}>
          <h2>New control</h2>
          <label>
            Name
            <input name="name" required />
            {fields.name ? <span className="error">{fields.name}</span> : null}
          </label>
          <label>
            Key
            <input name="key" required pattern="[A-Za-z][A-Za-z0-9_]*" />
            {fields.key ? <span className="error">{fields.key}</span> : null}
          </label>
          <label>
            Type
            <select name="controlType" defaultValue="button">
              {types.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Command
            <input name="command" required pattern="[A-Za-z][A-Za-z0-9_]*" placeholder="set_pump" />
            {fields.command ? <span className="error">{fields.command}</span> : null}
          </label>
          <label>
            On payload JSON
            <textarea name="onPayload" placeholder='{"enabled": true}' />
          </label>
          <label>
            Off payload JSON
            <textarea name="offPayload" placeholder='{"enabled": false}' />
          </label>
          <div className="filters">
            <label>
              Min
              <input name="min" type="number" defaultValue={0} />
            </label>
            <label>
              Max
              <input name="max" type="number" defaultValue={100} />
            </label>
            <label>
              Step
              <input name="step" type="number" defaultValue={1} />
            </label>
            <label>
              Payload key
              <input name="payloadKey" placeholder="speed" />
            </label>
          </div>
          {fields.configuration ? <p className="error">{fields.configuration}</p> : null}
          {error && !(error instanceof ApiError && error.fields) ? <p className="error">{errorText(error)}</p> : null}
          <button type="submit" disabled={pending}>
            {pending ? 'Saving…' : 'Add control'}
          </button>
        </form>
      ) : (
        <p className="muted">Viewers can see controls. Members can add them and send commands.</p>
      )}
    </section>
  )
}

function configurationFrom(controlType: string, form: FormData) {
  if (controlType === 'toggle') {
    return {
      onPayload: parseObject(String(form.get('onPayload')), { enabled: true }),
      offPayload: parseObject(String(form.get('offPayload')), { enabled: false }),
    }
  }
  if (controlType === 'slider') {
    return {
      min: Number(form.get('min') || 0),
      max: Number(form.get('max') || 100),
      step: Number(form.get('step') || 1),
      payloadKey: String(form.get('payloadKey') || 'value').trim(),
    }
  }
  return { payload: parseObject(String(form.get('onPayload')), {}) }
}

function parseObject(text: string, fallback: Record<string, unknown>) {
  const trimmed = text.trim()
  if (!trimmed) return fallback
  const parsed = JSON.parse(trimmed) as unknown
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new ApiError(400, 'VALIDATION_ERROR', 'Payload must be a JSON object')
  }
  return parsed as Record<string, unknown>
}

function ControlCard({
  control,
  devices,
  write,
  onChanged,
}: {
  control: ControlDefinition
  devices: { id: string; name: string; status: string }[]
  write: boolean
  onChanged: () => void
}) {
  const usable = devices.filter((device) => device.status !== 'disabled')
  const [deviceId, setDeviceId] = useState(usable[0]?.id ?? '')
  const [commandId, setCommandId] = useState('')
  const [on, setOn] = useState(false)
  const [slider, setSlider] = useState(control.configuration.min ?? 0)
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const command = useQuery({
    queryKey: ['command', commandId],
    queryFn: () => api<{ command: DeviceCommand }>(`/api/v1/commands/${commandId}`),
    enabled: commandId !== '',
    refetchInterval: (query) => {
      const status = query.state.data?.command.status
      if (!status || status === 'pending' || status === 'published') return 1000
      return false
    },
  })

  async function send(payload: Record<string, unknown>) {
    if (!deviceId) return
    setError(null)
    setPending(true)
    try {
      const saved = await api<{ command: DeviceCommand }>(`/api/v1/devices/${deviceId}/commands`, {
        method: 'POST',
        body: JSON.stringify({ command: control.command, payload }),
      })
      setCommandId(saved.command.id)
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  async function remove() {
    setError(null)
    try {
      await api(`/api/v1/controls/${control.id}`, { method: 'DELETE' })
      onChanged()
    } catch (err) {
      setError(err)
    }
  }

  const status = command.data?.command.status
  const config = control.configuration

  return (
    <article className="card widget">
      <h2>{control.name}</h2>
      <p className="muted">
        {control.controlType} · {control.command}
      </p>
      <label>
        Device
        <select value={deviceId} onChange={(event) => setDeviceId(event.target.value)}>
          {usable.length === 0 ? <option value="">No device</option> : null}
          {usable.map((device) => (
            <option key={device.id} value={device.id}>
              {device.name}
            </option>
          ))}
        </select>
      </label>
      {control.controlType === 'button' ? (
        <button type="button" disabled={!write || pending || !deviceId} onClick={() => void send(config.payload ?? {})}>
          {control.name}
        </button>
      ) : null}
      {control.controlType === 'toggle' ? (
        <button
          type="button"
          disabled={!write || pending || !deviceId}
          onClick={() => {
            const next = !on
            setOn(next)
            void send(next ? (config.onPayload ?? { enabled: true }) : (config.offPayload ?? { enabled: false }))
          }}
        >
          {on ? 'On' : 'Off'}
        </button>
      ) : null}
      {control.controlType === 'slider' ? (
        <form
          className="form"
          onSubmit={(event) => {
            event.preventDefault()
            void send({ [config.payloadKey ?? 'value']: slider })
          }}
        >
          <label>
            {slider}
            <input
              type="range"
              min={config.min ?? 0}
              max={config.max ?? 100}
              step={config.step ?? 1}
              value={slider}
              onChange={(event) => setSlider(Number(event.target.value))}
            />
          </label>
          <button type="submit" disabled={!write || pending || !deviceId}>
            Set
          </button>
        </form>
      ) : null}
      {status ? <p className={`pill ${status}`}>{commandStatusLabel(status)}</p> : <p className="muted">Not sent</p>}
      {command.data?.command.errorMessage ? <p className="error">{command.data.command.errorMessage}</p> : null}
      {error ? <p className="error">{errorText(error)}</p> : null}
      {write ? (
        <button type="button" className="secondary small" onClick={() => void remove()}>
          Remove
        </button>
      ) : null}
    </article>
  )
}
