<script lang="ts" setup>
import { reactive, ref, onMounted } from 'vue'
import { GetPrinters, StartServer, StopServer } from '../wailsjs/go/main/App'

const config = reactive({
  port: "1122",
  key: ""
})

const serverStatus = ref("Stopped")
const printers = ref<string[]>([])

const refreshPrinters = async () => {
  try {
    printers.value = await GetPrinters()
  } catch (e) {
    console.error(e)
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

onMounted(async () => {
  // Initial load
  await refreshPrinters()
  
  // Auto-start server
  await toggleServer()
})
</script>

<template>
  <div class="h-screen w-screen overflow-hidden bg-white text-gray-900 font-sans text-left flex flex-col relative">
    
    <!-- Scrollable Content Area -->
    <div class="flex-1 overflow-y-auto scrollbar-hide">
      <div class="w-full">
      
      <!-- Header -->
      <header class="p-4 border-b border-gray-200 bg-gray-50">
        <h1 class="text-xl font-bold text-gray-800 mb-1 flex items-center gap-2">
          <i-material-symbols-print-connect class="text-blue-600" />
          Print Bridge Client
        </h1>
        <p class="text-xs text-gray-500">WebSocket Printer Bridge</p>
      </header>

      <!-- Server Control -->
      <div class="p-4 border-b border-gray-200">
        <h2 class="text-base font-semibold mb-4 flex items-center gap-2">
          <i-material-symbols-dns class="text-gray-600" />
          <span class="w-2.5 h-2.5 rounded-full" :class="serverStatus === 'Running' ? 'bg-green-500' : 'bg-red-500'"></span>
          Server Control
        </h2>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label class="block text-xs font-medium text-gray-500 uppercase tracking-wider mb-1">Port</label>
            <input v-model="config.port" type="text" class="w-full bg-white border border-gray-300 px-3 py-2 text-sm text-gray-800 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all rounded-md" :disabled="serverStatus === 'Running'" />
          </div>
          <div>
            <label class="block text-xs font-medium text-gray-500 uppercase tracking-wider mb-1">Secret Key (Optional)</label>
            <input v-model="config.key" type="password" class="w-full bg-white border border-gray-300 px-3 py-2 text-sm text-gray-800 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all rounded-md" :disabled="serverStatus === 'Running'" placeholder="Leave empty for no auth" />
          </div>
        </div>

        <button 
          @click="toggleServer" 
          class="w-full py-2 px-4 font-semibold text-white transition-all active:opacity-90 rounded-md flex items-center justify-center gap-2"
          :class="serverStatus === 'Running' ? 'bg-red-500 hover:bg-red-600' : 'bg-blue-600 hover:bg-blue-700'"
        >
          <i-material-symbols-stop v-if="serverStatus === 'Running'" />
          <i-material-symbols-play-arrow v-else />
          {{ serverStatus === 'Running' ? 'Stop Server' : 'Start Server' }}
        </button>
      </div>

      <!-- Printers -->
      <div class="p-4 border-gray-200">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-base font-semibold text-gray-800 flex items-center gap-2">
            <i-material-symbols-print class="text-gray-600" />
            Available Printers
          </h2>
          <button @click="refreshPrinters" class="text-xs bg-gray-100 hover:bg-gray-200 text-blue-600 px-3 py-1.5 border border-gray-200 transition-colors rounded-md flex items-center gap-1">
            <i-material-symbols-refresh />
            Refresh
          </button>
        </div>
        
        <div v-if="printers.length === 0" class="text-gray-400 italic text-center py-6 bg-gray-50 border border-dashed border-gray-200">
          No printers found.
        </div>
        <ul v-else class="grid grid-cols-1 gap-0 border border-gray-200 divide-y divide-gray-200">
          <li v-for="p in printers" :key="p" class="px-3 py-2 flex items-center gap-2 hover:bg-gray-50 transition-colors text-sm bg-white">
            <i-material-symbols-print class="text-lg opacity-70 text-gray-500" />
            <span class="font-medium truncate text-gray-700" :title="p">{{ p }}</span>
          </li>
        </ul>
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
