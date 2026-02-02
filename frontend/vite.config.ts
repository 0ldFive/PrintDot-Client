import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Icons from 'unplugin-icons/vite'
import Components from 'unplugin-vue-components/vite'
import IconsResolver from 'unplugin-icons/resolver'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    Components({
      resolvers: [
        IconsResolver({
          prefix: 'i', // Prefix for components, e.g. <i-material-symbols-print />
          enabledCollections: ['material-symbols']
        })
      ]
    }),
    Icons({
      autoInstall: true
    })
  ]
})
