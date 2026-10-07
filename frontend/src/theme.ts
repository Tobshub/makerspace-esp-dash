import { useState } from 'react'

export type Theme = 'dark' | 'light'

function readTheme(): Theme {
  return document.documentElement.getAttribute('data-theme') === 'light' ? 'light' : 'dark'
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(readTheme)

  function toggleTheme() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark'
    document.documentElement.setAttribute('data-theme', next)
    try {
      localStorage.setItem('sst-theme', next)
    } catch {
      /* storage can be unavailable */
    }
    setTheme(next)
  }

  return { theme, toggleTheme }
}
