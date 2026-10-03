import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  api, unwrap, type BillCreate, type DocumentKind, type BillInput, type BillStatus, type PaymentInput, type ProfileInput, type Schemas,
} from './client'

export const keys = {
  me: ['me'] as const,
  household: ['household'] as const,
  home: (month?: string) => ['home', month ?? 'current'] as const,
  types: ['types'] as const,
  providers: (typeId?: string) => ['providers', typeId ?? 'all'] as const,
  profiles: (includeArchived = false) => ['profiles', { includeArchived }] as const,
  profile: (id: string) => ['profile', id] as const,
  bills: (filters: BillFilters) => ['bills', filters] as const,
  bill: (id: string) => ['bill', id] as const,
  document: (id: string) => ['document', id] as const,
  extraction: (documentId: string) => ['extraction', documentId] as const,
}

export interface BillFilters {
  profileId?: string
  status?: BillStatus[]
  archived?: boolean
  limit?: number
}

// --- Queries ------------------------------------------------------------------

export const meQuery = queryOptions({
  queryKey: keys.me,
  queryFn: () => unwrap(api.GET('/auth/me')),
  staleTime: 5 * 60_000,
  retry: false,
})

export const useMe = () => useQuery(meQuery)

export const useHousehold = () =>
  useQuery({ queryKey: keys.household, queryFn: () => unwrap(api.GET('/household')) })

export const useHome = (month?: string) =>
  useQuery({
    queryKey: keys.home(month),
    queryFn: () => unwrap(api.GET('/home', { params: { query: month ? { month } : {} } })),
  })

export const useTypes = () =>
  useQuery({ queryKey: keys.types, queryFn: () => unwrap(api.GET('/types')), staleTime: 10 * 60_000 })

export const useProviders = (typeId?: string) =>
  useQuery({
    queryKey: keys.providers(typeId),
    queryFn: () => unwrap(api.GET('/providers', { params: { query: typeId ? { typeId } : {} } })),
    staleTime: 10 * 60_000,
  })

export const useProfiles = (includeArchived = false) =>
  useQuery({
    queryKey: keys.profiles(includeArchived),
    queryFn: () => unwrap(api.GET('/profiles', { params: { query: { includeArchived } } })),
  })

export const useProfile = (id: string) =>
  useQuery({
    queryKey: keys.profile(id),
    queryFn: () => unwrap(api.GET('/profiles/{id}', { params: { path: { id } } })),
    enabled: !!id,
  })

export async function fetchBillsPage(filters: BillFilters, cursor?: string) {
  return unwrap(
    api.GET('/bills', {
      params: {
        query: {
          profileId: filters.profileId,
          status: filters.status?.length ? filters.status : undefined,
          archived: filters.archived || undefined,
          limit: filters.limit,
          cursor,
        },
      },
    }),
  )
}

export const useBill = (id: string) =>
  useQuery({
    queryKey: keys.bill(id),
    queryFn: () => unwrap(api.GET('/bills/{id}', { params: { path: { id } } })),
  })

// --- Mutations ------------------------------------------------------------------

/** After any write, refresh everything derived from bills and profiles. */
function useInvalidateAll() {
  const qc = useQueryClient()
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: ['home'] }),
      qc.invalidateQueries({ queryKey: ['profiles'] }),
      qc.invalidateQueries({ queryKey: ['profile'] }),
      qc.invalidateQueries({ queryKey: ['bills'] }),
      qc.invalidateQueries({ queryKey: ['bill'] }),
    ])
}

export function useCreateProfile() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: ProfileInput) => unwrap(api.POST('/profiles', { body })),
    onSuccess: invalidate,
  })
}

export function useUpdateProfile(id: string) {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: ProfileInput) => unwrap(api.PUT('/profiles/{id}', { params: { path: { id } }, body })),
    onSuccess: invalidate,
  })
}

export function useArchiveProfile() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, restore }: { id: string; restore?: boolean }) =>
      restore
        ? unwrap(api.POST('/profiles/{id}/restore', { params: { path: { id } } }))
        : unwrap(api.POST('/profiles/{id}/archive', { params: { path: { id } } })),
    onSuccess: invalidate,
  })
}

export function useCreateBill() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: BillCreate) => unwrap(api.POST('/bills', { body })),
    onSuccess: invalidate,
  })
}

export function useUpdateBill(id: string) {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: BillInput) => unwrap(api.PUT('/bills/{id}', { params: { path: { id } }, body })),
    onSuccess: invalidate,
  })
}

export function useBillAction() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'archive' | 'restore' | 'delete' }) => {
      const path = { params: { path: { id } } }
      if (action === 'archive') return unwrap(api.POST('/bills/{id}/archive', path))
      if (action === 'restore') return unwrap(api.POST('/bills/{id}/restore', path))
      return unwrap(api.DELETE('/bills/{id}', path))
    },
    onSuccess: invalidate,
  })
}

export function useAddPayment(billId: string) {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: PaymentInput) => unwrap(api.POST('/bills/{id}/payments', { params: { path: { id: billId } }, body })),
    onSuccess: invalidate,
  })
}

export function useDeletePayment() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (id: string) => unwrap(api.DELETE('/payments/{id}', { params: { path: { id } } })),
    onSuccess: invalidate,
  })
}

export function useUpdateHousehold() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: Schemas['HouseholdInput']) => unwrap(api.PUT('/household', { body })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.household })
      void qc.invalidateQueries({ queryKey: keys.me })
    },
  })
}

export function useSaveType() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id?: string; body: Schemas['BillTypeInput'] }) =>
      id
        ? unwrap(api.PUT('/types/{id}', { params: { path: { id } }, body }))
        : unwrap(api.POST('/types', { body })),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.types }),
  })
}

export function useArchiveType() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => unwrap(api.POST('/types/{id}/archive', { params: { path: { id } } })),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.types }),
  })
}

// --- Documents ------------------------------------------------------------------

/** Fresh signed links each time a document is opened (they expire after 5 minutes). */
export const useDocument = (id: string | undefined) =>
  useQuery({
    queryKey: keys.document(id ?? ''),
    queryFn: () => unwrap(api.GET('/documents/{id}', { params: { path: { id: id! } } })),
    enabled: !!id,
    staleTime: 0,
  })

/** Refreshes everything that shows documents; call after an upload finishes. */
export function useInvalidateDocuments() {
  const qc = useQueryClient()
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: ['bill'] }),
      qc.invalidateQueries({ queryKey: ['document'] }),
      qc.invalidateQueries({ queryKey: keys.household }),
    ])
}

export function useUpdateDocument() {
  const invalidate = useInvalidateDocuments()
  return useMutation({
    mutationFn: ({ id, billId, kind }: { id: string; billId: string | null; kind: DocumentKind }) =>
      unwrap(api.PUT('/documents/{id}', { params: { path: { id } }, body: { billId, kind } })),
    onSuccess: invalidate,
  })
}

export function useDeleteDocument() {
  const invalidate = useInvalidateDocuments()
  return useMutation({
    mutationFn: (id: string) => unwrap(api.DELETE('/documents/{id}', { params: { path: { id } } })),
    onSuccess: invalidate,
  })
}


// --- Extraction -------------------------------------------------------------------

/** The latest extraction of a document; polls while it is queued or running. */
export const useExtraction = (documentId: string | undefined) =>
  useQuery({
    queryKey: keys.extraction(documentId ?? ''),
    queryFn: async () => {
      const { data, response } = await api.GET('/documents/{id}/extraction', { params: { path: { id: documentId! } } })
      if (response.status === 404) return null
      if (!response.ok) throw new Error(response.statusText)
      return data!
    },
    enabled: !!documentId,
    refetchInterval: (q) => (q.state.data && ['queued', 'running'].includes(q.state.data.status) ? 1500 : false),
  })

export function useStartExtraction() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (documentId: string) => unwrap(api.POST('/documents/{id}/extraction', { params: { path: { id: documentId } } })),
    onSuccess: (data, documentId) => qc.setQueryData(keys.extraction(documentId), data),
  })
}

// --- Scanned bills & profile history ----------------------------------------------

export const useProfileHistory = (profileId: string) =>
  useQuery({
    queryKey: ['profileHistory', profileId] as const,
    queryFn: () => unwrap(api.GET('/profiles/{id}/history', { params: { path: { id: profileId } } })),
    enabled: !!profileId,
  })

export function useSaveScannedBill(documentId: string) {
  const invalidate = useInvalidateAll()
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: Schemas['ScannedBillInput']) => unwrap(api.POST('/documents/{id}/bill', { params: { path: { id: documentId } }, body })),
    onSuccess: async () => {
      await invalidate()
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['profileHistory'] }),
        qc.invalidateQueries({ queryKey: ['document'] }),
      ])
    },
  })
}
