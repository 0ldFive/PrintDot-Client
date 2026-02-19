<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { GetSettings, SaveSettings, Restart, GetLogPort, GetRemoteForwarderStatus, DisconnectRemoteForwarder, ConnectRemoteForwarder } from '../../wailsjs/go/main/App'
import { WindowHide } from '../../wailsjs/runtime/runtime'
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

const saveStatus = ref('')
const logPort = ref(0)
const isConnecting = ref(false)
const isDisconnecting = ref(false)

type RemoteStatus = {
  connected: boolean
  lastError: string
  lastChange: number
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
        return
      }
    }
    remoteStatus.value = await GetRemoteForwarderStatus()
  } catch (e) {
    console.error(e)
  }
}

onMounted(async () => {
  try {
    const s = await GetSettings()
    settings.value = s
    locale.value = s.language
    logPort.value = await GetLogPort()
    await refreshRemoteStatus()
    if (logPort.value > 0 && 'EventSource' in window) {
      remoteStatusStream = new EventSource(`http://localhost:${logPort.value}/api/forwarder/stream`)
      remoteStatusStream.onmessage = (event) => {
        try {
          remoteStatus.value = JSON.parse(event.data)
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
})

onBeforeUnmount(() => {
  if (remoteStatusTimer !== null) {
    window.clearInterval(remoteStatusTimer)
    remoteStatusTimer = null
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
  saveStatus.value = t('settings.saving') || 'Saving...'
  
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

    saveStatus.value = t('settings.saved') || 'Saved'
    
    setTimeout(() => {
      saveStatus.value = ''
      isSaving.value = false
    }, 2000)
    
  } catch (e) {
    console.error(e)
    saveStatus.value = 'Error saving settings'
    isSaving.value = false
  }
}

const disconnectRemote = async () => {
  if (isDisconnecting.value || !remoteStatus.value.connected) return
  isDisconnecting.value = true
  try {
    if (logPort.value > 0) {
      await fetch(`http://localhost:${logPort.value}/api/forwarder/disconnect`, { method: 'POST' })
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
        <label class="block text-sm font-medium text-gray-700 mb-2">{{ t('settings.language') }}</label>
        <select v-model="settings.language" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500">
          <option value="zh-CN">简体中文</option>
          <option value="en">English</option>
        </select>
      </div>

      <!-- Auto Start -->
      <div class="bg-white p-4 rounded-lg border border-gray-200 shadow-sm flex items-center justify-between">
        <label class="text-sm font-medium text-gray-700">{{ t('settings.autoStart') }}</label>
        <label class="relative inline-flex items-center cursor-pointer">
          <input type="checkbox" v-model="settings.autoStart" class="sr-only peer">
          <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
        </label>
      </div>

      <!-- Forwarding Service -->
      <div class="bg-white p-4 rounded-lg border border-gray-200 shadow-sm space-y-4">
        <h3 class="text-sm font-semibold text-gray-800 border-b border-gray-100 pb-2">{{ t('settings.forwarding') }}</h3>

        <div class="flex items-center justify-between">
          <label class="text-sm font-medium text-gray-700">{{ t('settings.autoConnect') }}</label>
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" v-model="settings.remoteAutoConnect" class="sr-only peer">
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>

        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <p class="text-xs font-medium text-gray-500">{{ t('settings.forwarderStatus') }}</p>
            <p class="text-sm font-semibold" :class="remoteStatus.connected ? 'text-green-600' : 'text-gray-500'">
              {{ remoteStatus.connected ? t('settings.connected') : t('settings.disconnected') }}
            </p>
            <p v-if="remoteStatus.lastError" class="text-xs text-red-500 mt-1">
              <span class="font-medium">{{ t('settings.lastError') }}:</span>
              <span class="break-words">{{ remoteStatus.lastError }}</span>
            </p>
          </div>
          <div class="flex items-center gap-2 flex-wrap sm:flex-nowrap sm:shrink-0">
            <button
              @click="connectRemote"
              :disabled="remoteStatus.connected || isConnecting"
              class="bg-blue-600 hover:bg-blue-700 text-white font-medium py-1.5 px-3 rounded-md shadow-sm transition-colors duration-200 text-xs disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
            >
              {{ isConnecting ? t('settings.connecting') : t('settings.connect') }}
            </button>
            <button
              @click="disconnectRemote"
              :disabled="!remoteStatus.connected || isDisconnecting"
              class="bg-white hover:bg-gray-50 text-gray-700 font-medium py-1.5 px-3 rounded-md border border-gray-300 shadow-sm transition-colors duration-200 text-xs disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
            >
              {{ isDisconnecting ? t('settings.disconnecting') : t('settings.disconnect') }}
            </button>
          </div>
        </div>
        
        <div>
          <label class="block text-xs font-medium text-gray-500 mb-1">{{ t('settings.authAddress') }}</label>
          <input v-model="settings.remoteAuthUrl" type="text" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" placeholder="http://server:8080/api/client/login" />
        </div>

        <div>
          <label class="block text-xs font-medium text-gray-500 mb-1">{{ t('settings.wsAddress') }}</label>
          <input v-model="settings.remoteWsUrl" type="text" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" placeholder="ws://server:8081/ws/client" />
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-xs font-medium text-gray-500 mb-1">{{ t('settings.clientId') }}</label>
            <input v-model="settings.remoteClientId" type="text" disabled class="w-full border border-gray-200 bg-gray-100 text-gray-500 rounded-md p-2 text-sm cursor-not-allowed" :title="t('settings.deviceIdReadonly')" />
            <p class="text-[11px] text-gray-400 mt-1">{{ t('settings.deviceIdReadonly') }}</p>
          </div>
          <div>
            <label class="block text-xs font-medium text-gray-500 mb-1">{{ t('settings.secretKey') }}</label>
            <input v-model="settings.remoteSecretKey" type="password" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" />
          </div>
        </div>

        <div>
          <label class="block text-xs font-medium text-gray-500 mb-1">{{ t('settings.clientName') }}</label>
          <input v-model="settings.remoteClientName" type="text" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" />
        </div>
      </div>

    </div>

    <!-- Save Button -->
    <div class="flex items-center justify-end gap-2 pt-3 border-t border-gray-200">
      <button 
        @click="WindowHide"
        class="bg-white hover:bg-gray-50 text-gray-700 font-medium py-1.5 px-4 rounded-md border border-gray-300 shadow-sm transition-colors duration-200 text-sm"
      >
        {{ t('settings.cancel') }}
      </button>

      <button 
        @click="saveSettings" 
        :disabled="isSaving"
        class="bg-blue-600 hover:bg-blue-700 text-white font-medium py-1.5 px-4 rounded-md shadow-sm transition-colors duration-200 flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
      >
        <i-material-symbols-save-outline v-if="!isSaving" />
        <i-material-symbols-refresh v-else class="animate-spin" />
        {{ isSaving ? t('settings.saving') || 'Saving...' : t('settings.save') }}
      </button>
    </div>
  </div>
</template>