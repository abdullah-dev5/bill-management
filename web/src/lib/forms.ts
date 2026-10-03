import type { FieldValues, Path, UseFormSetError } from 'react-hook-form'
import { toast } from 'sonner'
import { ApiError } from '@/api/client'

/**
 * Shows API validation errors on their form fields; anything else becomes a toast.
 * `fieldMap` renames API fields to form fields when they differ.
 */
export function applyServerErrors<T extends FieldValues>(
  err: unknown,
  setError: UseFormSetError<T>,
  fieldMap: Record<string, Path<T>> = {},
): void {
  if (err instanceof ApiError && Object.keys(err.fields).length > 0) {
    let shown = false
    for (const [field, message] of Object.entries(err.fields)) {
      const name = (fieldMap[field] ?? field) as Path<T>
      setError(name, { type: 'server', message })
      shown = true
    }
    if (shown) return
  }
  toast.error(err instanceof Error ? err.message : 'Something went wrong.')
}
