/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        canvas: {
          page:     '#12141a',
          header:   '#1a1d25',
          sidebar:  '#1a1d25',
          active:   '#252832',
          pane:     '#252832',
          terminal: '#0a0b0e',
          right:    '#1c1f27',
          modal:    '#1c1f27',
          input:    '#252832',
          border:   '#252832',
          primary:  '#f9fafb',
          muted:    '#9ca3af',
          faint:    '#6b7280',
          term:     '#c8c4bc',
          amber:    '#f59e0b',
          green:    '#4caf74',
          red:      '#e53935',
          terra:    '#c4784a',
          teal:     '#14b8a6',
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
