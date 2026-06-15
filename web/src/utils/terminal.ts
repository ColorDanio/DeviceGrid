import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebglAddon } from '@xterm/addon-webgl'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { SearchAddon } from '@xterm/addon-search'
import '@xterm/xterm/css/xterm.css'

/**
 * Ghostty-inspired dark theme — 256-color palette with true-color feel.
 * Based on Ghostty's default "night" palette.
 */
export const ghosttyTheme = {
  background: '#0e0e1c',
  foreground: '#c7c7e6',
  cursor: '#d0d0e8',
  cursorAccent: '#0e0e1c',
  selectionBackground: 'rgba(208, 208, 232, 0.2)',
  selectionInactiveBackground: 'rgba(208, 208, 232, 0.1)',

  // Normal ANSI colors — Ghostty defaults
  black: '#000000',
  red: '#e84040',
  green: '#52c464',
  yellow: '#e8a64a',
  blue: '#5a78e8',
  magenta: '#b250e8',
  cyan: '#4ad4d4',
  white: '#d0d0e8',

  // Bright ANSI colors
  brightBlack: '#4a4a66',
  brightRed: '#ff6b6b',
  brightGreen: '#6de878',
  brightYellow: '#ffc857',
  brightBlue: '#8a9eff',
  brightMagenta: '#d074ff',
  brightCyan: '#74e8e8',
  brightWhite: '#ffffff',
}

export interface TerminalOptions {
  fontSize?: number
  fontFamily?: string
  rows?: number
  cols?: number
}

export function createTerminal(container: HTMLElement, opts: TerminalOptions = {}) {
  const term = new Terminal({
    fontSize: opts.fontSize ?? 14,
    fontFamily: opts.fontFamily ?? "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'SF Mono', Menlo, monospace",
    lineHeight: 1.15,
    letterSpacing: 0,
    allowProposedApi: true,
    scrollback: 10000,
    cursorBlink: true,
    cursorStyle: 'bar',
    cursorWidth: 2,
    theme: ghosttyTheme,
    drawBoldTextInBrightColors: false,
    minimumContrastRatio: 4.5,
    scrollSensitivity: 3,
    smoothScrollDuration: 100,
    macOptionIsMeta: true,
    convertEol: false,
  })

  // Fit addon — must be loaded before WebGL for proper sizing
  const fit = new FitAddon()
  term.loadAddon(fit)

  // Unicode 11 for proper wide character handling (emoji, CJK)
  const unicode11 = new Unicode11Addon()
  term.loadAddon(unicode11)
  term.unicode.activeVersion = '11'

  // Web links — clickable URLs
  term.loadAddon(new WebLinksAddon())

  // Search addon — Ctrl+F search
  const search = new SearchAddon()
  term.loadAddon(search)

  // Open in container first, then fit, then enable WebGL
  term.open(container)
  fit.fit()

  // Enable WebGL renderer for GPU acceleration
  // Must be loaded AFTER open() — it replaces the DOM renderer
  try {
    const webgl = new WebglAddon()
    webgl.onContextLoss(() => {
      webgl.dispose()
    })
    term.loadAddon(webgl)
  } catch {
    // Fall back to canvas renderer if WebGL is unavailable
  }

  return { term, fit, search }
}
