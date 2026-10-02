import type { Bill } from '@/api/client'
import { daysUntil } from './dates'

/** Visual states — docs/10-design-system.md §2.2. "dueSoon" is an unpaid bill due within 3 days. */
export type StatusTone = 'paid' | 'unpaid' | 'dueSoon' | 'overdue' | 'partial' | 'review' | 'cancelled'

export const DUE_SOON_DAYS = 3

export function statusTone(bill: Pick<Bill, 'status' | 'dueDate'>, today = new Date()): StatusTone {
  switch (bill.status) {
    case 'paid':
      return 'paid'
    case 'overdue':
      return 'overdue'
    case 'cancelled':
      return 'cancelled'
    case 'draft':
      return 'review'
    case 'partially_paid':
    case 'unpaid': {
      if (bill.dueDate) {
        const d = daysUntil(bill.dueDate, today)
        if (d >= 0 && d <= DUE_SOON_DAYS) return 'dueSoon'
      }
      return bill.status === 'partially_paid' ? 'partial' : 'unpaid'
    }
  }
}

export const isOpen = (b: Pick<Bill, 'status'>) =>
  b.status === 'unpaid' || b.status === 'partially_paid' || b.status === 'overdue'
