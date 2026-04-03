/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        canvas: {
          page:     '#F0EBE3',
          header:   '#E8E2D8',
          sidebar:  '#F0EBE3',
          active:   '#E8E0D4',
          pane:     '#DDD5C8',
          terminal: '#0F0E0C',
          right:    '#F7F4EF',
          modal:    '#FAF8F4',
          input:    '#F5F2EC',
          border:   '#DDD5C8',
          primary:  '#1C1A16',
          muted:    '#8A8278',
          faint:    '#A09888',
          term:     '#C8C4BC',
          amber:    '#F59E0B',
          green:    '#4CAF74',
          red:      '#E53935',
          terra:    '#C4784A',
        },
      },
      fontFamily: {
        ui:   ['-apple-system', 'BlinkMacSystemFont', 'Inter', 'sans-serif'],
        mono: ['SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
    },
  },
  plugins: [],
}
