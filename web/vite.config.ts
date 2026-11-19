import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Two pages: the setup page at / and the overlay OBS loads at /watchers.
// `npm run dev` proxies the API to a running app on the default port.
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      input: { index: 'index.html', watchers: 'watchers.html' },
    },
  },
  server: {
    proxy: { '/api': 'http://localhost:7341' },
  },
});
