<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { GetSettings, SaveSettings, Restart } from '../../wailsjs/go/main/App'

const { t, locale } = useI18n()

const settings = ref({
  language: 'zh-CN',
  autoStart: false,
  remoteServer: '',
  remoteUser: '',
  remotePassword: ''
})

const saveStatus = ref('')

onMounted(async () => {
  try {
    const s = await GetSettings()
    settings.value = s
    locale.value = s.language
  } catch (e) {
    console.error(e)
  }
})

const save = async () => {
  try {
    // Save settings first
    await SaveSettings(settings.value)
    
    // Show restart message
    saveStatus.value = t('settings.saved')
    
    // Wait briefly and restart
    setTimeout(async () => {
      await Restart()
    }, 1500)
    
  } catch (e) {
    console.error(e)
    saveStatus.value = 'Error saving settings'
  }
}
</script>

<template>
  <div class="h-screen w-screen bg-gray-50 flex flex-col p-6">
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

      <!-- Remote Server -->
      <div class="bg-white p-4 rounded-lg border border-gray-200 shadow-sm space-y-4">
        <h3 class="text-sm font-semibold text-gray-800 border-b border-gray-100 pb-2">{{ t('settings.remotePrint') }}</h3>
        
        <div>
          <label class="block text-xs font-medium text-gray-500 mb-1">{{ t('settings.serverAddress') }}</label>
          <input v-model="settings.remoteServer" type="text" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" placeholder="https://print.example.com" />
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-xs font-medium text-gray-500 mb-1">{{ t('settings.username') }}</label>
            <input v-model="settings.remoteUser" type="text" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" />
          </div>
          <div>
            <label class="block text-xs font-medium text-gray-500 mb-1">{{ t('settings.password') }}</label>
            <input v-model="settings.remotePassword" type="password" class="w-full border border-gray-300 rounded-md p-2 text-sm focus:ring-blue-500 focus:border-blue-500" />
          </div>
        </div>
      </div>

    </div>

    <div class="mt-6 flex items-center justify-between">
      <span class="text-green-600 text-sm font-medium transition-opacity duration-500" :class="saveStatus ? 'opacity-100' : 'opacity-0'">
        {{ saveStatus }}
      </span>
      <button @click="save" class="bg-blue-600 text-white px-6 py-2 rounded-md hover:bg-blue-700 transition-colors shadow-sm font-medium text-sm">
        {{ t('settings.save') }}
      </button>
    </div>
  </div>
</template>