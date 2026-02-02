<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { GetUsageGuide } from '../../wailsjs/go/main/App'
import MarkdownIt from 'markdown-it'

const content = ref('')
const md = new MarkdownIt()

onMounted(async () => {
  try {
    const markdown = await GetUsageGuide()
    content.value = md.render(markdown)
  } catch (e) {
    console.error(e)
    content.value = '<p class="text-red-500">Failed to load usage guide.</p>'
  }
})
</script>

<template>
  <div class="h-screen w-screen bg-white flex flex-col">
    <div class="flex-1 overflow-y-auto p-8 prose prose-sm max-w-none prose-slate">
      <div v-html="content"></div>
    </div>
  </div>
</template>

<style>
@reference "tailwindcss";

/* Add some basic markdown styling overrides if needed */
.prose h1 {
  @apply text-2xl font-bold mb-4 pb-2 border-b border-gray-200 text-gray-800;
}
.prose h2 {
  @apply text-xl font-bold mt-6 mb-3 text-gray-800;
}
.prose h3 {
  @apply text-lg font-bold mt-4 mb-2 text-gray-800;
}
.prose p {
  @apply mb-4 leading-relaxed text-gray-600;
}
.prose ul {
  @apply list-disc list-inside mb-4 pl-4 text-gray-600;
}
.prose code {
  @apply bg-gray-100 px-1 py-0.5 rounded text-sm font-mono text-pink-600;
}
.prose pre {
  @apply bg-gray-900 text-gray-100 p-4 rounded-md overflow-x-auto mb-4 text-sm font-mono;
}
.prose pre code {
  @apply bg-transparent p-0 text-gray-100;
}
</style>