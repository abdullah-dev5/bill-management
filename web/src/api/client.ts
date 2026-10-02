import createClient, { type Middleware } from 'openapi-fetch'
import type { components, paths } from './schema'

export type Schemas = components['schemas']
export type Me = Schemas['Me']
export type Household = Schemas['Household']
export type BillType = Schemas['BillType']
export type Provider = Schemas['Provider']
export type Profile = Schemas['Profile']
export type ProfileInput = Schemas['ProfileInput']
export type ProfileKind = Schemas['ProfileKind']
export type Bill = Schemas['Bill']
export type BillDetail = Schemas['BillDetail']
export type BillInput = Schemas['BillInput']
export type BillCreate = Schemas['BillCreate']
export type BillStatus = Schemas['BillStatus']
export type Payment = Schemas['Payment']
export type PaymentInput = Schemas['PaymentInput']
export type PaymentMethod = Schemas['PaymentMethod']
export type HomeSummary = Schemas['HomeSummary']
export type ApiErrorBody = Schemas['Error']

/** An error response from the API, with its machine code and per-field messages. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>
  readonly existingBillId?: string

  constructor(status: number, body: Partial<ApiErrorBody> | undefined) {
    super(body?.message ?? 'Something went wrong. Please try again.')
    this.status = status
    this.code = body?.code ?? 'unknown'
    this.fields = body?.fields ?? {}
    this.existingBillId = body?.existingBillId
  }
}

function readCookie(name: string): string | undefined {
  return document.cookie
    .split('; ')
    .find((c) => c.startsWith(name + '='))
    ?.slice(name.length + 1)
}

// Double-submit CSRF: echo the bm_csrf cookie on every state-changing request.
const csrf: Middleware = {
  onRequest({ request }) {
    if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method)) {
      const token = readCookie('bm_csrf')
      if (token) request.headers.set('X-CSRF-Token', token)
    }
    return request
  },
}

export const api = createClient<paths>({ baseUrl: '/api/v1', credentials: 'same-origin' })
api.use(csrf)

/** Unwraps an openapi-fetch result, throwing ApiError on failure. */
export async function unwrap<T>(
  promise: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const { data, error, response } = await promise
  if (!response.ok) throw new ApiError(response.status, error as Partial<ApiErrorBody>)
  return data as T
}
