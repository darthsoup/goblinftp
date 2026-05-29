import type { FileInfo } from '~/types/api'
import { defineStore } from 'pinia'
import { ApiError } from '~/types/api'

export const useFilesStore = defineStore('files', () => {
  const currentPath = ref('/')
  const files = ref<FileInfo[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const selected = ref<Set<string>>(new Set())

  async function list(path?: string) {
    const api = useApi()
    const target = path ?? currentPath.value
    loading.value = true
    error.value = null
    try {
      const result = await api.get<FileInfo[]>(`/api/files?path=${encodeURIComponent(target)}`)
      files.value = result
      currentPath.value = target
      selected.value = new Set()
    }
    catch (e) {
      error.value = e instanceof ApiError ? e.message : 'Failed to list directory'
    }
    finally {
      loading.value = false
    }
  }

  async function navigate(path: string) {
    await list(path)
  }

  async function navigateUp() {
    const parts = currentPath.value.split('/').filter(Boolean)
    parts.pop()
    const parent = parts.length > 0 ? `/${parts.join('/')}` : '/'
    await navigate(parent)
  }

  async function downloadFile(filePath: string): Promise<void> {
    const api = useApi()
    const data = await api.post<{ token: string }>('/api/files/download-token', { path: filePath })
    const url = `/api/files/download?path=${encodeURIComponent(filePath)}&token=${data.token}`
    window.open(url, '_blank')
  }

  function toggleSelection(name: string) {
    const next = new Set(selected.value)
    if (next.has(name))
      next.delete(name)
    else next.add(name)
    selected.value = next
  }

  function clearSelection() {
    selected.value = new Set()
  }

  function $reset() {
    currentPath.value = '/'
    files.value = []
    loading.value = false
    error.value = null
    selected.value = new Set()
  }

  const pathSegments = computed(() => {
    const parts = currentPath.value.split('/').filter(Boolean)
    return parts.reduce(
      (acc, part, i) => {
        acc.push({ label: part, path: `/${parts.slice(0, i + 1).join('/')}` })
        return acc
      },
      [] as Array<{ label: string, path: string }>,
    )
  })

  return {
    currentPath,
    files,
    loading,
    error,
    selected,
    pathSegments,
    list,
    navigate,
    navigateUp,
    downloadFile,
    toggleSelection,
    clearSelection,
    $reset,
  }
})
