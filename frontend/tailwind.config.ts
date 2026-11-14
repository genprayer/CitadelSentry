import type { Config } from 'tailwindcss'

export default <Partial<Config>>{
  darkMode: 'class',
  content: [
    './app/**/*.{vue,js,ts,jsx,tsx}',
    './components/**/*.{vue,js,ts,jsx,tsx}',
    './layouts/**/*.{vue,js,ts,jsx,tsx}',
    './pages/**/*.{vue,js,ts,jsx,tsx}',
    './app.vue'
  ],
  theme: {
    extend: {
      colors: {
        // Citadel-WebProject official design tokens
        background: 'rgb(var(--color-background) / <alpha-value>)',
        surface: 'rgb(var(--color-surface) / <alpha-value>)',
        accent: 'rgb(var(--color-accent) / <alpha-value>)',
        'accent-secondary': 'rgb(var(--color-accent-secondary) / <alpha-value>)',
        highlight: 'rgb(var(--color-highlight) / <alpha-value>)',
        primary: 'rgb(var(--color-primary) / <alpha-value>)',
        'ui-danger': 'rgb(var(--color-ui-danger) / <alpha-value>)',
        'ui-warning': 'rgb(var(--color-ui-warning) / <alpha-value>)',
        'ui-success': 'rgb(var(--color-ui-success) / <alpha-value>)',
        squad: {
          yellow: '#FFCC00',
          orange: '#FF9900',
          dark: '#1A1A1A',
          'dark-grey': '#2b2b2b'
        },
        brand: {
          50: '#fffbeb',
          100: '#fef3c7',
          200: '#fde68a',
          300: '#fcd34d',
          400: '#fbbf24',
          500: '#FF9900', // Citadel / Squad Orange
          600: '#ea580c',
          700: '#c2410c',
          800: '#9a3412',
          900: '#7c2d12',
          950: '#431407'
        }
      },
      fontFamily: {
        // Official Citadel font stack
        sans: ['Orbitron', 'Tektur', 'sans-serif'],
        display: ['Orbitron', 'Tektur', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace']
      },
      keyframes: {
        'soft-glow': {
          '0%, 100%': {
            boxShadow: '0 0 15px -5px rgba(var(--color-accent), 0.4), inset 0 0 4px 1px rgba(var(--color-accent), 0.1)'
          },
          '50%': {
            boxShadow: '0 0 25px 0px rgba(var(--color-accent), 0.5), inset 0 0 4px 1px rgba(var(--color-accent), 0.2)'
          }
        },
        'glow-border': {
          '0%, 100%': {
            borderColor: 'rgb(var(--color-accent-secondary) / 0.2)',
            boxShadow: '0 0 10px -5px rgb(var(--color-accent-secondary) / 0.3)'
          },
          '50%': {
            borderColor: 'rgb(var(--color-accent) / 0.5)',
            boxShadow: '0 0 20px 0px rgb(var(--color-accent) / 0.3)'
          }
        }
      },
      animation: {
        'soft-glow': 'soft-glow 4s ease-in-out infinite',
        'glow-border': 'glow-border 6s ease-in-out infinite'
      }
    }
  }
}
