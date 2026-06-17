/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        brand: '#4F46E5', // indigo
        'brand-600': '#4338CA',
        'brand-soft': '#EEF0FF',
        wa: '#22C55E', // WhatsApp green
        'wa-600': '#16A34A',
        navy: '#0F172A',
        rail: '#0E1730', // dark nav rail
        ink: '#0F172A',
        muted: '#64748B',
        panel: '#F4F5F7', // app background
        hair: '#E8EAEE', // hairline borders
      },
      boxShadow: {
        card: '0 1px 2px rgba(16,24,40,.05), 0 1px 3px rgba(16,24,40,.05)',
        pop: '0 12px 32px rgba(16,24,40,.14)',
        rail: '0 8px 24px rgba(79,70,229,.35)',
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
