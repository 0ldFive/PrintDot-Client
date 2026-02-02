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

onMounted(() => {
  // Initial load
  refreshPrinters()
  
  // Listen for backend logs
  EventsOn("log", (msg: string) => addLog(msg))
})
</script>

<template>
  <div class="min-h-screen bg-slate-900 text-slate-200 p-8 font-sans text-left">
    <div class="max-w-4xl mx-auto space-y-8">
      
      <!-- Header -->
      <header class="text-center mb-10">
        <h1 class="text-4xl font-bold text-teal-400 mb-2">Print Bridge Client</h1>
        <p class="text-slate-400">WebSocket Printer Bridge</p>
      </header>

      <!-- Server Control -->
      <div class="bg-slate-800 rounded-lg p-6 shadow-lg border border-slate-700">
        <h2 class="text-xl font-semibold mb-4 flex items-center gap-3">
          <span class="w-3 h-3 rounded-full shadow-[0_0_10px]" :class="serverStatus === 'Running' ? 'bg-green-500 shadow-green-500/50' : 'bg-red-500 shadow-red-500/50'"></span>
          Server Control
        </h2>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
          <div>
            <label class="block text-sm font-medium text-slate-400 mb-2">Port</label>
            <input v-model="config.port" type="text" class="w-full bg-slate-700 border border-slate-600 rounded-md px-4 py-2 text-white focus:outline-none focus:border-teal-500 focus:ring-1 focus:ring-teal-500 transition-all" :disabled="serverStatus === 'Running'" />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-400 mb-2">Secret Key (Optional)</label>
            <input v-model="config.key" type="password" class="w-full bg-slate-700 border border-slate-600 rounded-md px-4 py-2 text-white focus:outline-none focus:border-teal-500 focus:ring-1 focus:ring-teal-500 transition-all" :disabled="serverStatus === 'Running'" placeholder="Leave empty for no auth" />
          </div>
        </div>

        <button 
          @click="toggleServer" 
          class="w-full py-3 px-4 rounded-md font-bold transition-all transform active:scale-95"
          :class="serverStatus === 'Running' ? 'bg-red-600 hover:bg-red-700 shadow-lg shadow-red-900/20' : 'bg-teal-600 hover:bg-teal-700 shadow-lg shadow-teal-900/20'"
        >
          {{ serverStatus === 'Running' ? 'Stop Server' : 'Start Server' }}
        </button>
      </div>

      <!-- Printers -->
      <div class="bg-slate-800 rounded-lg p-6 shadow-lg border border-slate-700">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-xl font-semibold text-slate-200">Available Printers</h2>
          <button @click="refreshPrinters" class="text-sm bg-slate-700 hover:bg-slate-600 text-teal-400 px-4 py-2 rounded-md transition-colors border border-slate-600 hover:border-teal-500/50">
            Refresh List
          </button>
        </div>
        
        <div v-if="printers.length === 0" class="text-slate-500 italic text-center py-8 bg-slate-900/50 rounded-lg">
          No printers found or list empty.
        </div>
        <ul v-else class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <li v-for="p in printers" :key="p" class="bg-slate-700/30 border border-slate-700 px-4 py-3 rounded-md flex items-center gap-3 hover:bg-slate-700/50 transition-colors">
            <span class="text-2xl">🖨️</span>
            <span class="font-medium truncate" :title="p">{{ p }}</span>
          </li>
        </ul>
      </div>

      <!-- Logs -->
      <div class="bg-slate-800 rounded-lg p-6 shadow-lg border border-slate-700">
        <h2 class="text-xl font-semibold mb-4 text-slate-200">System Logs</h2>
        <div class="bg-black/40 rounded-lg p-4 h-48 overflow-y-auto font-mono text-sm space-y-1 scrollbar-thin scrollbar-thumb-slate-600 scrollbar-track-transparent">
          <div v-for="(log, i) in logs.slice().reverse()" :key="i" class="text-slate-300 border-b border-slate-700/30 last:border-0 pb-1 break-words">
            {{ log }}
          </div>
          <div v-if="logs.length === 0" class="text-slate-600 italic">No logs yet...</div>
        </div>
      </div>

    </div>
  </div>
</template>

<style>
/* Reset some Wails default styles if needed */
body {
  margin: 0;
  background-color: #0f172a; /* slate-900 */
}
</style>
