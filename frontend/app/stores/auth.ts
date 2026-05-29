import type { AuthStatus, ConnectData, ConnectRequest, SystemVars } from '~/types/api'
import { defineStore } from 'pinia'
import { ApiError } from '~/types/api'

export const useAuthStore = defineStore('auth', () => {
  const csrfToken = ref('')
  const connected = ref(false)
  const ssoAutoConnect = ref(false)
  const initialDirectory = ref('/')
  const capabilities = ref<{ disableChmod: boolean }>({ disableChmod: false })
  const systemVars = ref<SystemVars | null>(null)
  const error = ref<string | null>(null)
  const loading = ref(false)

  // Called on app mount — fetches system vars + auth status using $fetch directly
  // (no CSRF needed for these GET requests, and avoids circular dep with useApi)
  async function init() {
    try {
      const svRes = await $fetch<{ success: boolean, data?: SystemVars }>('/api/system/vars')
      if (svRes.success && svRes.data)
        systemVars.value = svRes.data
    }
    catch {}

    try {
      const statusRes = await $fetch<{ success: boolean, data?: AuthStatus }>('/api/auth/status')
      if (statusRes.success && statusRes.data) {
        connected.value = statusRes.data.connected
        ssoAutoConnect.value = statusRes.data.ssoAutoConnect
        if (statusRes.data.csrfToken)
          csrfToken.value = statusRes.data.csrfToken
      }
    }
    catch {}
  }

  // SSO auto-connect: called when ssoAutoConnect=true after init()
  async function ssoConnect() {
    loading.value = true
    error.value = null
    try {
      const api = useApi()
      const data = await api.post<ConnectData>('/api/auth/sso-connect')
      csrfToken.value = data.csrfToken
      connected.value = true
      ssoAutoConnect.value = false
      initialDirectory.value = data.initialDirectory
      capabilities.value = data.capabilities
    }
    catch (e) {
      error.value = e instanceof ApiError ? e.message : 'SSO connect failed'
    }
    finally {
      loading.value = false
    }
  }

  // Manual connect from login form
  async function connect(req: ConnectRequest) {
    loading.value = true
    error.value = null
    try {
      // POST /api/auth/connect is public (no CSRF), use $fetch directly
      const res = await $fetch<{ success: boolean, data?: ConnectData, errors?: Array<{ code: string, message: string }> }>(
        '/api/auth/connect',
        { method: 'POST', body: req },
      )
      if (!res.success) {
        const err = res.errors?.[0]
        throw new ApiError(err?.code ?? 'ERR_UNKNOWN', err?.message ?? 'Login failed')
      }
      const data = res.data!
      csrfToken.value = data.csrfToken
      connected.value = true
      initialDirectory.value = data.initialDirectory
      capabilities.value = data.capabilities
    }
    catch (e) {
      error.value = e instanceof ApiError ? e.message : 'Connection failed'
      throw e
    }
    finally {
      loading.value = false
    }
  }

  async function disconnect() {
    const api = useApi()
    try {
      await api.post('/api/auth/disconnect')
    }
    catch {}
    // Reset state regardless of API result
    csrfToken.value = ''
    connected.value = false
    ssoAutoConnect.value = false
    initialDirectory.value = '/'
  }

  const allowedTypes = computed(() =>
    systemVars.value?.connection.allowedTypes ?? ['ftp', 'sftp'],
  )

  return {
    csrfToken,
    connected,
    ssoAutoConnect,
    initialDirectory,
    capabilities,
    systemVars,
    error,
    loading,
    allowedTypes,
    init,
    ssoConnect,
    connect,
    disconnect,
  }
})
