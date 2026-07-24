import { defineConfig } from 'vite';

const proxyTarget = process.env.VITE_PROXY_TARGET ?? 'http://127.0.0.1:8080';
const wsTarget = proxyTarget.replace(/^http/, 'ws');
const port = Number(process.env.VITE_PORT ?? 5175);

export default defineConfig({
  appType: 'spa',
  server: {
    host: '0.0.0.0',
    port,
    strictPort: true,
    hmr: {
      clientPort: port,
    },
    watch: {
      usePolling: true,
      interval: 500,
    },
    proxy: {
      '/rooms': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/health': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/ws': {
        target: wsTarget,
        ws: true,
      },
    },
  },
  optimizeDeps: {
    include: ['phaser'],
  },
  plugins: [
    {
      name: 'no-stale-sw',
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          const url = req.url?.split('?')[0] ?? '';
          if (
            url === '/sw.js' ||
            url === '/dev-sw.js' ||
            url === '/service-worker.js' ||
            url === '/manifest.webmanifest' ||
            url === '/manifest.json'
          ) {
            res.statusCode = 404;
            res.setHeader('Content-Type', 'text/plain');
            res.end('Not found');
            return;
          }
          next();
        });
      },
    },
  ],
});
