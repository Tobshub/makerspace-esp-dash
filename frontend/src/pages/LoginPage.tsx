import { useState, type FormEvent } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { api, errorText, fieldErrors, type User } from '../api/client'
import { useSession } from '../useSession'

export function LoginPage() {
  const { user, signIn } = useSession()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)

  if (user) return <Navigate to="/dashboard" replace />

  async function onSubmit(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    setError(null)
    try {
      const path = mode === 'login' ? '/api/v1/auth/login' : '/api/v1/auth/register'
      const body = await api<{ token: string; user: User }>(path, {
        method: 'POST',
        body: JSON.stringify(
          mode === 'login' ? { email, password } : { email, name, password },
        ),
      })
      signIn(body.token, body.user)
    } catch (err) {
      setError(err)
    } finally {
      setPending(false)
    }
  }

  const fields = fieldErrors(error)

  return (
    <section className="page">
      <p className="eyebrow">Account</p>
      <h1>{mode === 'login' ? 'Sign in' : 'Create an account'}</h1>
      <p className="lede">
        <Link to="/">Back home</Link>
      </p>
      <form className="form" onSubmit={onSubmit}>
        {error ? <p className="error">{errorText(error)}</p> : null}
        <label>
          Email
          <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required />
          {fields.email ? <span className="field-error">{fields.email}</span> : null}
        </label>
        {mode === 'register' ? (
          <label>
            Name
            <input value={name} onChange={(event) => setName(event.target.value)} required />
            {fields.name ? <span className="field-error">{fields.name}</span> : null}
          </label>
        ) : null}
        <label>
          Password
          <input
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            type="password"
            required
            minLength={8}
          />
          {fields.password ? <span className="field-error">{fields.password}</span> : null}
        </label>
        <button type="submit" disabled={pending}>
          {pending ? 'Working…' : mode === 'login' ? 'Sign in' : 'Create account'}
        </button>
      </form>
      <p>
        <button
          type="button"
          className="secondary"
          onClick={() => {
            setMode(mode === 'login' ? 'register' : 'login')
            setError(null)
          }}
        >
          {mode === 'login' ? 'Need an account?' : 'Already have an account?'}
        </button>
      </p>
    </section>
  )
}
