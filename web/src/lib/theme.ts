export type ThemeChoice = 'system' | 'light' | 'dark'

const KEY = 'bm-theme'

export function getTheme(): ThemeChoice {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}

/** Light/dark follow the system unless the user picked one (docs/10 §10). */
export function applyTheme(choice: ThemeChoice = getTheme()) {
  const root = document.documentElement
  if (choice === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', choice)
  try {
    if (choice === 'system') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, choice)
  } catch {
    /* storage unavailable: the choice lasts for this page only */
  }
}
