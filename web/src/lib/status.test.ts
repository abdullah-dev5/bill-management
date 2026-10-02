import { statusTone } from './status'

const today = new Date(2026, 9, 3) // 3 Oct 2026

describe('statusTone', () => {
  it('flags unpaid bills due within three days as due soon', () => {
    expect(statusTone({ status: 'unpaid', dueDate: '2026-10-05' }, today)).toBe('dueSoon')
    expect(statusTone({ status: 'unpaid', dueDate: '2026-10-03' }, today)).toBe('dueSoon')
  })
  it('keeps unpaid bills further out as unpaid', () => {
    expect(statusTone({ status: 'unpaid', dueDate: '2026-10-08' }, today)).toBe('unpaid')
    expect(statusTone({ status: 'unpaid' }, today)).toBe('unpaid')
  })
  it('maps server statuses', () => {
    expect(statusTone({ status: 'paid', dueDate: '2026-10-04' }, today)).toBe('paid')
    expect(statusTone({ status: 'overdue' }, today)).toBe('overdue')
    expect(statusTone({ status: 'partially_paid', dueDate: '2026-10-20' }, today)).toBe('partial')
    expect(statusTone({ status: 'draft' }, today)).toBe('review')
    expect(statusTone({ status: 'cancelled' }, today)).toBe('cancelled')
  })
})
