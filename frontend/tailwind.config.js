/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        brand: '#4F46E5', // indigo
        wa: '#22C55E', // WhatsApp green
        navy: '#0F172A',
        panel: '#F8FAFC',
        hair: '#E2E8F0',
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
