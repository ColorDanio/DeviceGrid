<template>
  <div class="theme-switcher" @click="cycle">
    <svg v-if="theme.mode === 'dark'" viewBox="0 0 24 24" width="16" height="16" fill="none"><path d="M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
    <svg v-else-if="theme.mode === 'light'" viewBox="0 0 24 24" width="16" height="16" fill="none"><circle cx="12" cy="12" r="4" stroke="currentColor" stroke-width="1.8"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
    <svg v-else viewBox="0 0 24 24" width="16" height="16" fill="none"><rect x="3" y="3" width="18" height="18" rx="3" stroke="currentColor" stroke-width="1.8"/><path d="M15 3v18" stroke="currentColor" stroke-width="1.8"/></svg>
  </div>
</template>

<script setup lang="ts">
import { useThemeStore } from '@/stores/theme'
const theme = useThemeStore()
function cycle() {
  if (theme.mode === 'dark') theme.setMode('light')
  else if (theme.mode === 'light') theme.setMode('system')
  else theme.setMode('dark')
}
</script>

<style scoped lang="scss">
.theme-switcher {
  width: 34px; height: 34px; border-radius: 8px; border: 1px solid var(--dg-border);
  background: var(--dg-bg-2); color: var(--dg-text-dim); cursor: pointer;
  display: flex; align-items: center; justify-content: center; transition: all 0.2s;
  &:hover { border-color: var(--accent); color: var(--accent); }
}
</style>
