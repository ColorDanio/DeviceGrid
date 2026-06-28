<template>
  <span v-if="svg" class="svg-icon" v-html="sanitized"></span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ svg: string }>()

// Sanitize: strip <script> tags and on* event handlers
const sanitized = computed(() => {
  if (!props.svg) return ''
  let s = props.svg
  // Remove script tags
  s = s.replace(/<script[^>]*>[\s\S]*?<\/script>/gi, '')
  // Remove on* event handlers
  s = s.replace(/\son\w+="[^"]*"/gi, '')
  s = s.replace(/\son\w+='[^']*'/gi, '')
  // Remove javascript: URLs
  s = s.replace(/javascript:/gi, '')
  return s
})
</script>

<style scoped>
.svg-icon { display: inline-flex; align-items: center; }
.svg-icon :deep(svg) { display: block; }
</style>
