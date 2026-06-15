/// <reference types="vite/client" />

declare module '*.scss'
declare module '*.css'

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
