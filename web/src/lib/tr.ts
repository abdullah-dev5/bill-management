import i18next from 'i18next'

/**
 * Translation for plain helper functions. The app's full key typing (react-i18next) is too
 * deep for TypeScript once it is passed around as a parameter, so helpers use this instead.
 */
export const tr = i18next.t.bind(i18next) as unknown as (key: string, options?: Record<string, unknown>) => string
