import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { es } from './es'

export const resources = {
  es: { translation: es },
}

void i18n.use(initReactI18next).init({
  lng: 'es',
  fallbackLng: 'es',
  resources,
  // React already escapes what it renders.
  interpolation: { escapeValue: false },
})

declare module 'i18next' {
  // Makes t('health.ok') type-checked against the Spanish texts.
  interface CustomTypeOptions {
    resources: (typeof resources)['es']
  }
}
