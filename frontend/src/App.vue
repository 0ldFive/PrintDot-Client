<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { GetPrinters, StartServer, StopServer } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'

const config = reactive({
  port: "1122",
  key: ""
})

const serverStatus = ref("Stopped")
const printers = ref<string[]>([])
const logs = ref<string[]>([])

const addLog = (msg: string) => {
  logs.value.push(`[${new Date().toLocaleTimeString()}] ${msg}`)
  if (logs.value.length > 50) logs.value.shift()
}

const refreshPrinters = async () => {
  try {
    printers.value = await GetPrinters()
    addLog(`Found ${printers.value.length} printers`)
  } catch (e) {
    addLog(`Error listing printers: ${e}`)
  }
}

const toggleServer = async () => {
  if (serverStatus.value === "Running") {
    try {
      await StopServer()
      serverStatus.value = "Stopped"
      addLog("Server stopped")
    } catch (e) {
      addLog(`Error stopping server: ${e}`)
    }
  } else {
    try {
      await StartServer(config.port, config.key)
      serverStatus.value = "Running"
      addLog(`Server started on port ${config.port}`)
    } catch (e) {
      addLog(`Error starting server: ${e}`)
    }
  }
}

onMounted(async () => {
  // Initial load
  await refreshPrinters()
  
  // Auto-start server
  await toggleServer()

  // Listen for backend logs
  EventsOn("log", (msg: string) => addLog(msg))
})
</script>

<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 p-6 font-sans text-left">
    <div class="max-w-3xl mx-auto space-y-6">
      
      <!-- Header -->
      <header class="text-center mb-6">
        <h1 class="text-3xl font-bold text-blue-600 mb-1">Print Bridge Client</h1>
        <p class="text-gray-500">WebSocket Printer Bridge</p>
      </header>

      <!-- Server Control -->
      <div class="bg-white rounded-lg p-6 shadow-sm border border-gray-200">
        <h2 class="text-lg font-semibold mb-4 flex items-center gap-2">
          <span class="w-3 h-3 rounded-full shadow-sm" :class="serverStatus === 'Running' ? 'bg-green-500' : 'bg-red-500'"></span>
          Server Control
        </h2>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1">Port</label>
            <input v-model="config.port" type="text" class="w-full bg-white border border-gray-300 rounded-md px-3 py-2 text-gray-800 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all" :disabled="serverStatus === 'Running'" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1">Secret Key (Optional)</label>
            <input v-model="config.key" type="password" class="w-full bg-white border border-gray-300 rounded-md px-3 py-2 text-gray-800 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all" :disabled="serverStatus === 'Running'" placeholder="Leave empty for no auth" />
          </div>
        </div>

        <div v-if="serverStatus === 'Running'" class="mb-4 p-3 bg-blue-50 text-blue-800 border border-blue-100 rounded-md text-sm font-mono break-all">
          📡 Connection URL: <strong>ws://localhost:{{ config.port }}/ws</strong>
        </div>

        <button 
          @click="toggleServer" 
          class="w-full py-2.5 px-4 rounded-md font-semibold text-white transition-all shadow-sm active:scale-[0.98]"
          :class="serverStatus === 'Running' ? 'bg-red-500 hover:bg-red-600' : 'bg-blue-600 hover:bg-blue-700'"
        >
          {{ serverStatus === 'Running' ? 'Stop Server' : 'Start Server' }}
        </button>
      </div>

      <!-- Printers -->
      <div class="bg-white rounded-lg p-6 shadow-sm border border-gray-200">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-lg font-semibold text-gray-800">Available Printers</h2>
          <button @click="refreshPrinters" class="text-sm bg-gray-100 hover:bg-gray-200 text-blue-600 px-3 py-1.5 rounded border border-gray-200 transition-colors">
            Refresh
          </button>
        </div>
        
        <div v-if="printers.length === 0" class="text-gray-400 italic text-center py-6 bg-gray-50 rounded-lg border border-dashed border-gray-200">
          No printers found.
        </div>
        <ul v-else class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <li v-for="p in printers" :key="p" class="bg-gray-50 border border-gray-200 px-3 py-2 rounded flex items-center gap-2 hover:bg-gray-100 transition-colors text-sm">
            <span class="text-lg">🖨️</span>
            <span class="font-medium truncate text-gray-700" :title="p">{{ p }}</span>
          </li>
        </ul>
      </div>

      <!-- Logs -->
      <div class="bg-white rounded-lg p-6 shadow-sm border border-gray-200">
        <h2 class="text-lg font-semibold mb-3 text-gray-800">System Logs</h2>
        <div class="bg-gray-900 text-gray-300 rounded-lg p-4 h-40 overflow-y-auto font-mono text-xs space-y-1 scrollbar-thin scrollbar-thumb-gray-600 scrollbar-track-transparent">
          <div v-for="(log, i) in logs.slice().reverse()" :key="i" class="border-b border-gray-800 last:border-0 pb-0.5 break-words">
            {{ log }}
          </div>
          <div v-if="logs.length === 0" class="text-gray-600 italic">No logs yet...</div>
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
