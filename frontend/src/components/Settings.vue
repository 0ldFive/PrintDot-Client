<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { GetSettings, SaveSettings, Restart, GetLogPort, GetRemoteForwarderStatus, DisconnectRemoteForwarder, ConnectRemoteForwarder } from '../../wailsjs/go/main/App'
import { main } from '../../wailsjs/go/models'

const { t, locale } = useI18n()

const settings = ref(new main.AppSettings({
  language: 'zh-CN',
  autoStart: false,
  remoteAutoConnect: true,
  remoteServer: '',
  remoteAuthUrl: '',
  remoteWsUrl: '',
  remoteClientId: '',
  remoteSecretKey: '',
  remoteClientName: '',
  windowWidth: 0,
  windowHeight: 0,
  windowX: 0,
  windowY: 0,
  maximized: false
}))

const logPort = ref(0)
const isConnecting = ref(false)
const isDisconnecting = ref(false)
const isSyncing = ref(false)
const hasLoaded = ref(false)
let autoSaveTimer: number | null = null

type RemoteStatus = {
  connected: boolean
  lastError: string
  lastChange: number
  autoReconnect?: boolean
}

const remoteStatus = ref<RemoteStatus>({
  connected: false,
  lastError: '',
  lastChange: 0
})

let remoteStatusTimer: number | null = null
let remoteStatusStream: EventSource | null = null

const refreshRemoteStatus = async () => {
  try {
    if (logPort.value > 0) {
      const resp = await fetch(`http://localhost:${logPort.value}/api/forwarder/status`)
      if (resp.ok) {
        remoteStatus.value = await resp.json()
        if (typeof remoteStatus.value.autoReconnect === 'boolean') {
          setSettingsSilently(() => {
            settings.value.remoteAutoConnect = remoteStatus.value.autoReconnect as boolean
          })
        }
        return
      }
    }
    remoteStatus.value = await GetRemoteForwarderStatus()
    if (typeof remoteStatus.value.autoReconnect === 'boolean') {
      setSettingsSilently(() => {
        settings.value.remoteAutoConnect = remoteStatus.value.autoReconnect as boolean
      })
    }
  } catch (e) {
    console.error(e)
  }
}

const setSettingsSilently = (update: () => void) => {
  isSyncing.value = true
  try {
    update()
  } finally {
    isSyncing.value = false
  }
}

onMounted(async () => {
  try {
    const s = await GetSettings()
    setSettingsSilently(() => {
      settings.value = s
      locale.value = s.language
    })
    logPort.value = await GetLogPort()
    await refreshRemoteStatus()
    if (logPort.value > 0 && 'EventSource' in window) {
      remoteStatusStream = new EventSource(`http://localhost:${logPort.value}/api/forwarder/stream`)
      remoteStatusStream.onmessage = (event) => {
        try {
          remoteStatus.value = JSON.parse(event.data)
          if (typeof remoteStatus.value.autoReconnect === 'boolean') {
            settings.value.remoteAutoConnect = remoteStatus.value.autoReconnect
          }
        } catch (e) {
          console.error(e)
        }
      }
      remoteStatusStream.onerror = () => {
        if (remoteStatusStream) {
          remoteStatusStream.close()
          remoteStatusStream = null
        }
        if (remoteStatusTimer === null) {
          remoteStatusTimer = window.setInterval(refreshRemoteStatus, 3000)
        }
      }
    } else {
      remoteStatusTimer = window.setInterval(refreshRemoteStatus, 3000)
    }
  } catch (e) {
    console.error(e)
  }
  hasLoaded.value = true
})

onBeforeUnmount(() => {
  if (remoteStatusTimer !== null) {
    window.clearInterval(remoteStatusTimer)
    remoteStatusTimer = null
  }
  if (autoSaveTimer !== null) {
    window.clearTimeout(autoSaveTimer)
    autoSaveTimer = null
  }
  if (remoteStatusStream) {
    remoteStatusStream.close()
    remoteStatusStream = null
  }
})

const isSaving = ref(false)

const saveSettings = async () => {
  if (isSaving.value) return
  isSaving.value = true
  try {
    // Save settings
    await SaveSettings(settings.value)
    
    // Update locale immediately in this window
    locale.value = settings.value.language
    
    // Notify main process to reload settings
    try {
      if (logPort.value > 0) {
        await fetch(`http://localhost:${logPort.value}/api/reload`, { method: 'POST' })
      }
    } catch (e) {
      console.log("Main process reload trigger failed", e)
    }

  } catch (e) {
    console.error(e)
  } finally {
    isSaving.value = false
  }
}

watch(settings, () => {
  if (isSyncing.value || !hasLoaded.value) return
  if (autoSaveTimer !== null) {
    window.clearTimeout(autoSaveTimer)
  }
  autoSaveTimer = window.setTimeout(() => {
    saveSettings()
  }, 400)
}, { deep: true })

const disconnectRemote = async () => {
  if (isDisconnecting.value || !remoteStatus.value.connected) return
  isDisconnecting.value = true
  try {
    settings.value.remoteAutoConnect = false
    await SaveSettings(settings.value)
    if (logPort.value > 0) {
      await fetch(`http://localhost:${logPort.value}/api/forwarder/disconnect`, { method: 'POST' })
      await fetch(`http://localhost:${logPort.value}/api/reload`, { method: 'POST' })
    } else {
      await DisconnectRemoteForwarder()
    }
    await refreshRemoteStatus()
  } catch (e) {
    console.error(e)
  } finally {
    isDisconnecting.value = false
  }
}

const connectRemote = async () => {
  if (isConnecting.value || remoteStatus.value.connected) return
  isConnecting.value = true
  try {
    if (logPort.value > 0) {
      await fetch(`http://localhost:${logPort.value}/api/forwarder/connect`, { method: 'POST' })
    } else {
      await ConnectRemoteForwarder()
    }
    await refreshRemoteStatus()
  } catch (e) {
    console.error(e)
  } finally {
    isConnecting.value = false
  }
}

const toggleRemote = async () => {
  if (remoteStatus.value.connected) {
    await disconnectRemote()
  } else {
    await connectRemote()
  }
}
</script>

<template>
  <div class="h-screen w-screen bg-gray-50 flex flex-col p-4 relative">
    
    <h1 class="text-2xl font-bold mb-6 text-gray-800 flex items-center gap-2">
      <i-material-symbols-settings-outline />
      {{ t('settings.title') }}
    </h1>

    <div class="space-y-6 flex-1 overflow-y-auto">
      
      <!-- Language -->
      <div class="bg-white p-4 rounded-lg border border-gray-200 shadow-sm">
        <label class="block text-sm font-medium text-gray-700 mb-2 flex items-center gap-2">
          <i-material-symbols-language class="text-gray-500" />
          {{ t('settings.language') }}
        </label>
        <select v-model="settings.language" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500">
          <option value="zh-CN">简体中文</option>
          <option value="en">English</option>
        </select>
      </div>

      <!-- Auto Start -->
      <div class="bg-white p-4 rounded-lg border border-gray-200 shadow-sm flex items-center justify-between">
        <label class="text-sm font-medium text-gray-700 flex items-center gap-2">
          <i-material-symbols-power class="text-gray-500" />
          {{ t('settings.autoStart') }}
        </label>
        <label class="relative inline-flex items-center cursor-pointer">
          <input type="checkbox" v-model="settings.autoStart" class="sr-only peer">
          <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
        </label>
      </div>

      <!-- Forwarding Service -->
      <div class="bg-white p-4 rounded-lg border border-gray-200 shadow-sm space-y-4">
        <h3 class="text-sm font-semibold text-gray-800 border-b border-gray-100 pb-2 flex items-center gap-2">
          <i-material-symbols-cloud-sync class="text-gray-600" />
          <span class="w-2.5 h-2.5 rounded-full" :class="remoteStatus.connected ? 'bg-green-500' : 'bg-red-500'"></span>
          {{ t('settings.forwarding') }}
        </h3>

        <div class="flex items-center justify-between">
          <label class="text-sm font-medium text-gray-700 flex items-center gap-2">
            <i-material-symbols-sync class="text-gray-500" />
            {{ t('settings.autoConnect') }}
          </label>
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" v-model="settings.remoteAutoConnect" class="sr-only peer">
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>

        <div>
          <button
            @click="toggleRemote"
            :disabled="isConnecting || isDisconnecting"
            class="w-full py-2 px-4 font-semibold rounded-md shadow-sm transition-colors duration-200 text-sm disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            :class="remoteStatus.connected ? 'bg-red-500 hover:bg-red-600 text-white' : 'bg-blue-600 hover:bg-blue-700 text-white'"
          >
            <i-material-symbols-stop v-if="remoteStatus.connected" />
            <i-material-symbols-play-arrow v-else />
            {{ remoteStatus.connected ? (isDisconnecting ? t('settings.disconnecting') : t('settings.disconnect')) : (isConnecting ? t('settings.connecting') : t('settings.connect')) }}
          </button>
        </div>

        <p v-if="remoteStatus.lastError" class="text-xs text-red-500">
          <span class="font-medium">{{ t('settings.lastError') }}:</span>
          <span class="break-words">{{ remoteStatus.lastError }}</span>
        </p>
        
        <div>
          <label class="block text-xs font-medium text-gray-500 mb-1 flex items-center gap-1">
            <i-material-symbols-lock-outline class="text-gray-400" />
            {{ t('settings.authAddress') }}
          </label>
          <input v-model="settings.remoteAuthUrl" type="text" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" placeholder="http://server:8080/api/client/login" />
        </div>

        <div>
          <label class="block text-xs font-medium text-gray-500 mb-1 flex items-center gap-1">
            <i-material-symbols-link class="text-gray-400" />
            {{ t('settings.wsAddress') }}
          </label>
          <input v-model="settings.remoteWsUrl" type="text" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" placeholder="ws://server:8081/ws/client" />
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label class="block text-xs font-medium text-gray-500 mb-1 flex items-center gap-1">
              <i-material-symbols-badge class="text-gray-400" />
              {{ t('settings.clientId') }}
            </label>
            <input v-model="settings.remoteClientId" type="text" disabled class="w-full border border-gray-200 bg-gray-100 text-gray-500 rounded-md p-2 text-sm cursor-not-allowed" :title="t('settings.deviceIdReadonly')" />
            <p class="text-[11px] text-gray-400 mt-1">{{ t('settings.deviceIdReadonly') }}</p>
          </div>
          <div>
            <label class="block text-xs font-medium text-gray-500 mb-1 flex items-center gap-1">
              <i-material-symbols-key class="text-gray-400" />
              {{ t('settings.secretKey') }}
            </label>
            <input v-model="settings.remoteSecretKey" type="password" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" />
          </div>
        </div>

        <div>
          <label class="block text-xs font-medium text-gray-500 mb-1 flex items-center gap-1">
            <i-material-symbols-badge class="text-gray-400" />
            {{ t('settings.clientName') }}
          </label>
          <input v-model="settings.remoteClientName" type="text" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" />
        </div>
      </div>

    </div>

  </div>
</template>