<script setup lang="ts">
import { ApiError } from '~/types/api'

const authStore = useAuthStore()
const filesStore = useFilesStore()
const { t } = useI18n()

const form = reactive({
  protocol: 'ftp',
  host: '',
  port: 21,
  username: '',
  password: '',
  passive: true,
})

const error = ref<string | null>(null)
const loading = ref(false)

watch(() => form.protocol, (proto) => {
  form.port = proto === 'sftp' ? 22 : 21
})

async function handleSubmit() {
  if (!form.host || !form.username)
    return
  loading.value = true
  error.value = null
  try {
    await authStore.connect({ ...form })
    await filesStore.list(authStore.initialDirectory)
  }
  catch (e) {
    error.value = e instanceof ApiError ? e.message : t('error.connectionFailed')
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900">
    <div class="w-full max-w-md bg-white dark:bg-gray-800 rounded-xl shadow-lg p-8">
      <div class="flex items-center justify-center gap-2 mb-6">
        <UIcon name="i-heroicons-server" class="w-8 h-8 text-primary-500" />
        <h1 class="text-2xl font-bold">
          GoblinFTP
        </h1>
      </div>

      <UAlert
        v-if="error"
        color="error"
        :description="error"
        class="mb-4"
      />

      <form class="space-y-4" @submit.prevent="handleSubmit">
        <!-- Protocol -->
        <div>
          <label class="block text-sm font-medium mb-1">{{ t('login.protocol') }}</label>
          <select
            v-model="form.protocol"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
          >
            <option v-for="type in authStore.allowedTypes" :key="type" :value="type">
              {{ type.toUpperCase() }}
            </option>
          </select>
        </div>

        <!-- Host -->
        <div>
          <label class="block text-sm font-medium mb-1">{{ t('login.host') }}</label>
          <UInput
            v-model="form.host"
            :placeholder="t('login.hostPlaceholder')"
            required
            class="w-full"
          />
        </div>

        <!-- Port -->
        <div>
          <label class="block text-sm font-medium mb-1">{{ t('login.port') }}</label>
          <UInput
            v-model.number="form.port"
            type="number"
            min="1"
            max="65535"
            required
            class="w-full"
          />
        </div>

        <!-- Username -->
        <div>
          <label class="block text-sm font-medium mb-1">{{ t('login.username') }}</label>
          <UInput
            v-model="form.username"
            autocomplete="username"
            required
            class="w-full"
          />
        </div>

        <!-- Password -->
        <div>
          <label class="block text-sm font-medium mb-1">{{ t('login.password') }}</label>
          <UInput
            v-model="form.password"
            type="password"
            autocomplete="current-password"
            class="w-full"
          />
        </div>

        <UButton
          type="submit"
          :loading="loading"
          class="w-full justify-center"
          block
        >
          {{ t('login.connect') }}
        </UButton>
      </form>
    </div>
  </div>
</template>
