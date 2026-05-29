import type { ApiEnvelope } from '~/types/api'
import { ApiError } from '~/types/api'

export function useApi() {
  // Get auth store lazily (avoid circular deps at module load)
  function getCsrfToken(): string {
    const authStore = useAuthStore()
    return authStore.csrfToken
  }

  async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {}
    const upper = method.toUpperCase()
    if (upper !== 'GET' && upper !== 'HEAD') {
      const csrf = getCsrfToken()
      if (csrf)
        headers['X-CSRF-Token'] = csrf
    }

    try {
      const response = await $fetch<ApiEnvelope<T>>(path, {
        method,
        headers,
        body: body !== undefined ? body : undefined,
      })
      if (!response.success) {
        const err = response.errors?.[0]
        throw new ApiError(err?.code ?? 'ERR_UNKNOWN', err?.message ?? 'Request failed')
      }
      return response.data as T
    }
    catch (e) {
      if (e instanceof ApiError)
        throw e
      // ofetch throws FetchError on non-2xx
      const msg = e instanceof Error ? e.message : 'Network error'
      throw new ApiError('ERR_NETWORK', msg)
    }
  }

  return {
    get: <T>(path: string) => call<T>('GET', path),
    post: <T>(path: string, body?: unknown) => call<T>('POST', path, body),
    patch: <T>(path: string, body?: unknown) => call<T>('PATCH', path, body),
    del: <T>(path: string, body?: unknown) => call<T>('DELETE', path, body),
  }
}
