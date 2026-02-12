<script lang="ts" setup>
import { reactive, ref, onMounted, onUnmounted, computed } from 'vue'
import { GetPrinters, StartServer, StopServer, GetAppMode, GetLogPort, GetSettings } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import Help from './components/Help.vue'
import Settings from './components/Settings.vue'
import { useI18n } from 'vue-i18n'

const { t, locale } = useI18n()
const appMode = ref("main")
const logPort = ref(0)
const logs = ref<string[]>([])
const clientCount = ref(0)
let logPollInterval: any = null

const config = reactive({
  port: "1122",
  key: ""
})

const connectionUrl = computed(() => {
  let url = `ws://localhost:${config.port}/ws`
  if (config.key) {
    url += `?key=${encodeURIComponent(config.key)}`
  }
  return url
})

const serverStatus = ref("Stopped")
type PrinterInfo = {
  name: string
  isDefault: boolean
}

const printers = ref<PrinterInfo[]>([])
const isLoadingPrinters = ref(false)

const refreshPrinters = async () => {
  if (isLoadingPrinters.value) return
  isLoadingPrinters.value = true
  printers.value = [] // Clear list immediately
  
  // Add a minimum delay to ensure animation is visible
  const minDelay = new Promise(resolve => setTimeout(resolve, 800))
  
  try {
    const [fetchedPrinters] = await Promise.all([
      GetPrinters(),
      minDelay
    ])
    printers.value = fetchedPrinters.slice().sort((a, b) => {
      if (a.isDefault !== b.isDefault) {
        return a.isDefault ? -1 : 1
      }
      return a.name.localeCompare(b.name)
    })
  } catch (e) {
    console.error(e)
  } finally {
    isLoadingPrinters.value = false
  }
}

const toggleServer = async () => {
  if (serverStatus.value === "Running") {
    try {
      await StopServer()
      serverStatus.value = "Stopped"
    } catch (e) {
      console.error(e)
    }
  } else {
    try {
      await StartServer(config.port, config.key)
      serverStatus.value = "Running"
    } catch (e) {
      console.error(e)
    }
  }
}

const fetchLogs = async () => {
  try {
    const res = await fetch(`http://localhost:${logPort.value}/api/logs`)
    if (res.ok) {
      const data = await res.json()
      logs.value = data.reverse()
    }
  } catch (e) {
    console.error("Failed to fetch logs", e)
  }
}

const clearAllLogs = async () => {
  logs.value = []
  try {
    await fetch(`http://localhost:${logPort.value}/api/logs/clear`, { method: 'POST' })
  } catch (e) {
    console.error("Failed to clear logs", e)
  }
}

onMounted(async () => {
  appMode.value = await GetAppMode()

  // Load settings for language
  try {
    const s = await GetSettings()
    if (s && s.language) {
      locale.value = s.language
    }
  } catch (e) {
    console.error("Failed to load settings", e)
  }

  if (appMode.value === "logs") {
    logPort.value = await GetLogPort()
    fetchLogs()
    logPollInterval = setInterval(fetchLogs, 1000)
  } else if (appMode.value === "main") {
    // Main mode
    await refreshPrinters()
    await toggleServer()
    
    // Listen for client count updates
    EventsOn("client_count", (count: number) => {
      clientCount.value = count
    })


    // Listen for settings reload
    EventsOn("reload_settings", async () => {
      try {
        const s = await GetSettings()
        if (s && s.language) {
          locale.value = s.language
        }
      } catch (e) {
        console.error("Failed to reload settings", e)
      }
    })
  }
})

onUnmounted(() => {
  if (logPollInterval) clearInterval(logPollInterval)
})
</script>

<template>
  <Help v-if="appMode === 'help'" />
  <Settings v-else-if="appMode === 'settings'" />
  
  <div v-else class="h-screen w-screen overflow-hidden bg-white text-gray-900 font-sans text-left flex flex-col relative">
    
    <!-- Content Area -->
    <div class="flex-1 overflow-hidden relative">
      
      <!-- LOGS MODE UI -->
      <div v-if="appMode === 'logs'" class="w-full h-full flex flex-col">
        <header class="p-4 border-b border-gray-200 bg-gray-50 flex justify-between items-center">
          <div>
            <h1 class="text-xl font-bold text-gray-800 mb-1 flex items-center gap-2">
              <i-material-symbols-terminal class="text-gray-700" />
              {{ t('logs.title') }}
            </h1>
            <p class="text-xs text-gray-500">{{ t('logs.subtitle') }}</p>
          </div>
          <button @click="clearAllLogs" class="text-xs text-red-600 hover:bg-red-50 px-3 py-1.5 border border-red-200 rounded-md transition-colors flex items-center gap-1">
            <i-material-symbols-delete-outline />
            {{ t('logs.clearAll') }}
          </button>
        </header>
        <div class="flex-1 bg-gray-900 text-gray-300 p-4 font-mono text-xs overflow-y-auto scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
          <div v-for="(log, i) in logs" :key="i" class="border-b border-gray-800 last:border-0 pb-1 mb-1 break-words hover:bg-gray-800/50">
            {{ log }}
          </div>
          <div v-if="logs.length === 0" class="text-gray-600 italic py-4 text-center">{{ t('logs.empty') }}</div>
        </div>
      </div>

      <!-- MAIN APP UI -->
      <div v-else class="w-full h-full flex flex-col">
        <!-- Header -->
        <header 
          class="p-4 border-b border-gray-200 flex-none transition-colors duration-300"
          :class="clientCount > 0 ? 'bg-green-600 text-white' : 'bg-gray-50 text-gray-900'"
        >
          <div class="flex justify-between items-center">
            <div>
              <h1 class="text-xl font-bold mb-1 flex items-center gap-2">
                <i-material-symbols-print-connect :class="clientCount > 0 ? 'text-white' : 'text-blue-600'" />
                {{ t('main.title') }}
              </h1>
              <p class="text-xs" :class="clientCount > 0 ? 'text-green-100' : 'text-gray-500'">
                {{ t('main.subtitle') }}
              </p>
            </div>
            
            <!-- Client Count Badge -->
            <div 
              class="flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-bold transition-all"
              :class="clientCount > 0 ? 'bg-white text-green-700 shadow-sm' : 'bg-gray-200 text-gray-600'"
            >
              <i-material-symbols-devices />
              <span>{{ clientCount }} {{ t('main.clients') }}</span>
            </div>
          </div>
        </header>

        <div class="flex-1 overflow-y-auto scrollbar-hide">
          <!-- Server Control -->
          <div class="p-4 border-b border-gray-200">
            <h2 class="text-base font-semibold mb-4 flex items-center gap-2">
              <i-material-symbols-dns class="text-gray-600" />
              <span class="w-2.5 h-2.5 rounded-full" :class="serverStatus === 'Running' ? 'bg-green-500' : 'bg-red-500'"></span>
              {{ t('main.serverControl') }}
            </h2>
            
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
              <div>
                <label class="block text-xs font-medium text-gray-500 uppercase tracking-wider mb-1">{{ t('main.port') }}</label>
                <input v-model="config.port" type="text" class="w-full bg-white border border-gray-300 px-3 py-2 text-sm text-gray-800 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all rounded-md" :disabled="serverStatus === 'Running'" />
              </div>
              <div>
                <label class="block text-xs font-medium text-gray-500 uppercase tracking-wider mb-1">{{ t('main.secretKey') }}</label>
                <input v-model="config.key" type="password" class="w-full bg-white border border-gray-300 px-3 py-2 text-sm text-gray-800 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all rounded-md" :disabled="serverStatus === 'Running'" :placeholder="t('main.placeholderKey')" />
              </div>
            </div>

            <button 
              @click="toggleServer" 
              class="w-full py-2 px-4 font-semibold text-white transition-all active:opacity-90 rounded-md flex items-center justify-center gap-2"
              :class="serverStatus === 'Running' ? 'bg-red-500 hover:bg-red-600' : 'bg-blue-600 hover:bg-blue-700'"
            >
              <i-material-symbols-stop v-if="serverStatus === 'Running'" />
              <i-material-symbols-play-arrow v-else />
              {{ serverStatus === 'Running' ? t('main.stopServer') : t('main.startServer') }}
            </button>

            <div class="mt-4 p-3 bg-gray-50 border border-gray-200 rounded-md">
              <label class="block text-xs font-medium text-gray-500 uppercase tracking-wider mb-1">{{ t('main.connectionUrl') }}</label>
              <div class="flex items-center gap-2">
                <code class="flex-1 bg-white border border-gray-300 px-2 py-1.5 text-xs text-gray-600 rounded select-all font-mono break-all">
                  {{ connectionUrl }}
                </code>
              </div>
            </div>
          </div>

          <!-- Printers -->
          <div class="p-4 border-gray-200">
            <div class="flex justify-between items-center mb-4">
              <h2 class="text-base font-semibold text-gray-800 flex items-center gap-2">
                <i-material-symbols-print class="text-gray-600" />
                {{ t('main.availablePrinters') }}
              </h2>
              <button 
                @click="refreshPrinters" 
                class="text-xs bg-gray-100 hover:bg-gray-200 text-blue-600 px-3 py-1.5 border border-gray-200 transition-colors rounded-md flex items-center gap-1"
                :disabled="isLoadingPrinters"
              >
                <i-material-symbols-refresh :class="{ 'animate-spin': isLoadingPrinters }" />
                {{ t('main.refresh') }}
              </button>
            </div>
            
            <div v-if="isLoadingPrinters" class="text-gray-500 italic text-center py-6 bg-gray-50 border border-dashed border-gray-200 flex flex-col items-center gap-2">
              <span>{{ t('main.loading') }}</span>
            </div>
            <div v-else-if="printers.length === 0" class="text-gray-400 italic text-center py-6 bg-gray-50 border border-dashed border-gray-200">
              {{ t('main.noPrinters') }}
            </div>
            <ul v-else class="grid grid-cols-1 gap-0 border border-gray-200 divide-y divide-gray-200">
              <li v-for="p in printers" :key="p.name" class="px-3 py-2 flex items-center gap-2 hover:bg-gray-50 transition-colors text-sm bg-white">
                <i-material-symbols-print class="text-lg opacity-70 text-gray-500" />
                <span class="font-medium truncate text-gray-700" :title="p.name">{{ p.name }}</span>
                <span v-if="p.isDefault" class="ml-auto text-[10px] px-2 py-0.5 rounded-full bg-blue-50 text-blue-700 border border-blue-100">
                  {{ t('main.defaultPrinter') }}
                </span>
              </li>
            </ul>
          </div>
        </div>
      </div>
      
    </div>
  </div>
</template>

<style>
/* Reset some Wails default styles if needed */
body {
  margin: 0;
  background-color: #f9fafb; /* gray-50 */
}
</style>
