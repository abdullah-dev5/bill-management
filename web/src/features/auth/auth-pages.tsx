import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { Link, Outlet, useNavigate, useSearch } from '@tanstack/react-router'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { api, unwrap } from '@/api/client'
import { keys } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Field, Input } from '@/components/ui/field'
import { applyServerErrors } from '@/lib/forms'
import { clearOfflineData } from '@/lib/pwa'

export function AuthLayout() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center px-4 py-10">
      <div className="mb-8 flex items-center gap-2 text-xl font-semibold text-ink">
        <img src="/favicon.svg" alt="" className="size-10" />
        {t('app.name')}
      </div>
      <div className="w-full max-w-md rounded-card bg-card p-6 sm:p-8">
        <Outlet />
      </div>
    </div>
  )
}

function AuthHeading({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div className="mb-6">
      <h1 className="text-2xl font-semibold">{title}</h1>
      {subtitle && <p className="mt-1 text-sm text-muted">{subtitle}</p>}
    </div>
  )
}

const signInSchema = z.object({
  email: z.string().trim().min(1),
  password: z.string().min(1),
})

export function SignInPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const search = useSearch({ from: '/public/sign-in' })
  const form = useForm<z.infer<typeof signInSchema>>({ resolver: zodResolver(signInSchema) })
  const [error, setError] = useState<string>()

  const onSubmit = form.handleSubmit(async (values) => {
    setError(undefined)
    try {
      clearOfflineData()
      const me = await unwrap(api.POST('/auth/login', { body: values }))
      qc.setQueryData(keys.me, me)
      await navigate({ to: search.redirect?.startsWith('/') ? search.redirect : '/' })
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.errorTitle'))
    }
  })

  return (
    <>
      <AuthHeading title={t('auth.signInTitle')} subtitle={t('auth.signInSubtitle')} />
      <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
        {search.reset && !error && (
          <p role="status" className="rounded-tile bg-[var(--st-paid-bg)] p-3 text-sm text-[var(--st-paid-fg)]">{t('auth.resetDone')}</p>
        )}
        {error && <p role="alert" className="rounded-tile bg-[var(--st-overdue-bg)] p-3 text-sm text-[var(--st-overdue-fg)]">{error}</p>}
        <Field label={t('auth.email')} error={form.formState.errors.email && t('validation.required')}>
          {(p) => <Input {...p} type="email" autoComplete="email" {...form.register('email')} />}
        </Field>
        <Field label={t('auth.password')} error={form.formState.errors.password && t('validation.required')}>
          {(p) => <Input {...p} type="password" autoComplete="current-password" {...form.register('password')} />}
        </Field>
        <Link to="/forgot-password" className="self-end text-sm font-semibold text-ink underline-offset-4 hover:underline">
          {t('auth.forgot')}
        </Link>
        <Button type="submit" loading={form.formState.isSubmitting}>
          {t('auth.signIn')}
        </Button>
      </form>
      <p className="mt-6 text-center text-sm text-muted">
        {t('auth.noAccount')}{' '}
        <Link to="/sign-up" className="font-semibold text-ink underline underline-offset-4">
          {t('auth.signUp')}
        </Link>
      </p>
    </>
  )
}

export function SignUpPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const schema = z.object({
    displayName: z.string().trim().min(1, t('validation.required')),
    email: z.string().trim().email(t('validation.email')),
    password: z.string().min(10, t('validation.password')),
  })
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({ resolver: zodResolver(schema) })
  const { errors } = form.formState

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      clearOfflineData()
      const me = await unwrap(api.POST('/auth/signup', { body: values }))
      qc.setQueryData(keys.me, me)
      await navigate({ to: '/onboarding' })
    } catch (err) {
      applyServerErrors(err, form.setError)
    }
  })

  return (
    <>
      <AuthHeading title={t('auth.signUpTitle')} subtitle={t('auth.signUpSubtitle')} />
      <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
        <Field label={t('auth.name')} error={errors.displayName?.message}>
          {(p) => <Input {...p} autoComplete="name" {...form.register('displayName')} />}
        </Field>
        <Field label={t('auth.email')} error={errors.email?.message}>
          {(p) => <Input {...p} type="email" autoComplete="email" {...form.register('email')} />}
        </Field>
        <Field label={t('auth.password')} hint={t('auth.passwordHint')} error={errors.password?.message}>
          {(p) => <Input {...p} type="password" autoComplete="new-password" {...form.register('password')} />}
        </Field>
        <Button type="submit" loading={form.formState.isSubmitting}>
          {t('auth.signUp')}
        </Button>
      </form>
      <p className="mt-6 text-center text-sm text-muted">
        {t('auth.haveAccount')}{' '}
        <Link to="/sign-in" className="font-semibold text-ink underline underline-offset-4">
          {t('auth.signIn')}
        </Link>
      </p>
    </>
  )
}

export function ForgotPasswordPage() {
  const { t } = useTranslation()
  const [sent, setSent] = useState(false)
  const form = useForm<{ email: string }>()

  const onSubmit = form.handleSubmit(async ({ email }) => {
    try {
      await unwrap(api.POST('/auth/password-reset/request', { body: { email } }))
      setSent(true)
    } catch (err) {
      applyServerErrors(err, form.setError)
    }
  })

  return (
    <>
      <AuthHeading title={t('auth.forgotTitle')} subtitle={sent ? undefined : t('auth.forgotSubtitle')} />
      {sent ? (
        <p role="status" className="rounded-tile bg-[var(--st-paid-bg)] p-4 text-sm text-[var(--st-paid-fg)]">
          {t('auth.linkSent')}
        </p>
      ) : (
        <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
          <Field label={t('auth.email')} error={form.formState.errors.email?.message}>
            {(p) => <Input {...p} type="email" autoComplete="email" {...form.register('email', { required: true })} />}
          </Field>
          <Button type="submit" loading={form.formState.isSubmitting}>
            {t('auth.sendLink')}
          </Button>
        </form>
      )}
      <Link to="/sign-in" className="mt-6 block text-center text-sm font-semibold text-ink underline underline-offset-4">
        {t('auth.backToSignIn')}
      </Link>
    </>
  )
}

export function ResetPasswordPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { token } = useSearch({ from: '/public/reset-password' })
  const schema = z.object({ password: z.string().min(10, t('validation.password')) })
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema) })

  const onSubmit = form.handleSubmit(async ({ password }) => {
    try {
      await unwrap(api.POST('/auth/password-reset/confirm', { body: { token: token ?? '', password } }))
      await navigate({ to: '/sign-in', search: { reset: '1' } })
    } catch (err) {
      applyServerErrors(err, form.setError, { token: 'password' })
    }
  })

  if (!token) {
    return (
      <>
        <AuthHeading title={t('auth.resetTitle')} />
        <p className="text-sm text-muted">{t('auth.resetMissingToken')}</p>
        <Link to="/forgot-password" className="mt-6 block text-center text-sm font-semibold text-ink underline underline-offset-4">
          {t('auth.sendLink')}
        </Link>
      </>
    )
  }
  return (
    <>
      <AuthHeading title={t('auth.resetTitle')} />
      <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
        <Field label={t('auth.newPassword')} hint={t('auth.passwordHint')} error={form.formState.errors.password?.message}>
          {(p) => <Input {...p} type="password" autoComplete="new-password" {...form.register('password')} />}
        </Field>
        <Button type="submit" loading={form.formState.isSubmitting}>
          {t('auth.setPassword')}
        </Button>
      </form>
    </>
  )
}
