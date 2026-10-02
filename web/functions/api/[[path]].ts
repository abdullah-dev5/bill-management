/**
 * Cloudflare Pages Function: proxies /api/* to the Go API on Render.
 *
 * Serving the API from the same origin as the app keeps the session cookie first-party
 * (SameSite=Lax works, no third-party-cookie blocking). Set API_ORIGIN in the Pages project,
 * e.g. https://bill-management-api.onrender.com
 */
interface Env {
  API_ORIGIN: string
}

export const onRequest = async ({ request, env }: { request: Request; env: Env }): Promise<Response> => {
  const url = new URL(request.url)
  const target = new URL(url.pathname + url.search, env.API_ORIGIN)

  const headers = new Headers(request.headers)
  headers.delete('host')
  const clientIP = request.headers.get('CF-Connecting-IP')
  if (clientIP) headers.set('X-Real-IP', clientIP)
  headers.set('X-Forwarded-Host', url.host)
  headers.set('X-Forwarded-Proto', 'https')

  const hasBody = request.method !== 'GET' && request.method !== 'HEAD'
  return fetch(target, {
    method: request.method,
    headers,
    body: hasBody ? request.body : undefined,
    redirect: 'manual',
  })
}
