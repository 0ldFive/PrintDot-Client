import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import zh from './locales/zh.json'

const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN', // default locale
  fallbackLocale: 'en',
  messages: {
    'en': en,
    'zh-CN': zh
  }
})

export default i18n