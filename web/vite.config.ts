import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Three pages: the setup page at /, the watchers overlay at /watchers, and the
// combined chat at /chat.
// `npm run dev` proxies the API to a running app on the default port.
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      input: { index: 'index.html', watchers: 'watchers.html', chat: 'chat.html' },
    },
  },
  server: {
    proxy: { '/api': 'http://localhost:7341' },
  },
});
