import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'

export type ThemeMode = 'dark' | 'light' | 'system'
export type AccentColor = 'blue' | 'teal' | 'green' | 'orange' | 'rose' | 'cyan'

interface AccentDef {
  name: string
  primary: string
  light: string
  dark: string
  glow: string
  tint: string
}

export const ACCENTS: Record<AccentColor, AccentDef> = {
  blue:   { name: '科技蓝', primary: '#2563eb', light: '#60a5fa', dark: '#1d4ed8', glow: 'rgba(37,99,235,0.06)',  tint: 'rgba(37,99,235,0.08)' },
  teal:   { name: '青松绿', primary: '#0d9488', light: '#2dd4bf', dark: '#0f766e', glow: 'rgba(13,148,136,0.06)',  tint: 'rgba(13,148,136,0.08)' },
  green:  { name: '森林绿', primary: '#16a34a', light: '#4ade80', dark: '#15803d', glow: 'rgba(22,163,74,0.06)',   tint: 'rgba(22,163,74,0.08)' },
  orange: { name: '活力橙', primary: '#ea580c', light: '#fb923c', dark: '#c2410c', glow: 'rgba(234,88,12,0.06)',   tint: 'rgba(234,88,12,0.08)' },
  rose:   { name: '玫瑰红', primary: '#e11d48', light: '#fb7185', dark: '#be123c', glow: 'rgba(225,29,72,0.06)',   tint: 'rgba(225,29,72,0.08)' },
  cyan:   { name: '天青色', primary: '#0891b2', light: '#22d3ee', dark: '#0e7490', glow: 'rgba(8,145,178,0.06)',   tint: 'rgba(8,145,178,0.08)' },
}

export const useThemeStore = defineStore('theme', () => {
  const storedMode = (localStorage.getItem('dg_theme') as ThemeMode) || 'dark'
  const storedAccent = (localStorage.getItem('dg_accent') as AccentColor) || 'blue'
  const mode = ref<ThemeMode>(storedMode)
  const accent = ref<AccentColor>(storedAccent)
  const systemDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)

  const isDark = computed(() => mode.value === 'dark' || (mode.value === 'system' && systemDark.value))

  function applyAccent() {
    const a = ACCENTS[accent.value]
    const root = document.documentElement
    const rgb = hexToRgb(a.primary)
    root.style.setProperty('--accent', a.primary)
    root.style.setProperty('--accent-light', a.light)
    root.style.setProperty('--accent-dark', a.dark)
    root.style.setProperty('--accent-rgb', rgb)
    root.style.setProperty('--accent-glow', a.glow)
    root.style.setProperty('--accent-tint', a.tint)
    root.style.setProperty('--accent-shadow', `0 0 20px rgba(${rgb}, 0.2)`)
    root.style.setProperty('--dg-glow-1', a.glow)
    root.style.setProperty('--dg-info-bg', a.tint)
    root.style.setProperty('--dg-border-bright', `rgba(${rgb}, 0.35)`)
    root.style.setProperty('--dg-table-row-hover', `rgba(${rgb}, 0.06)`)
  }

  function apply() {
    const html = document.documentElement
    html.setAttribute('data-theme', isDark.value ? 'dark' : 'light')
    applyAccent()
  }

  function setMode(m: ThemeMode) { mode.value = m; localStorage.setItem('dg_theme', m); apply() }
  function setAccent(a: AccentColor) { accent.value = a; localStorage.setItem('dg_accent', a); applyAccent() }

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    systemDark.value = e.matches
    if (mode.value === 'system') apply()
  })

  watch(isDark, () => apply(), { immediate: true })

  return { mode, accent, isDark, setMode, setAccent, apply }
})

function hexToRgb(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `${r}, ${g}, ${b}`
}
