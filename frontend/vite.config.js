import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import { nodePolyfills } from 'vite-plugin-node-polyfills'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  // Load environment variables from the backend directory
  const backendEnv = loadEnv(mode, path.resolve(__dirname, '../backend'), '')
  const env = loadEnv(mode, process.cwd(), '')
  const targetPort = backendEnv.PORT || 4001;
  const target = `http://localhost:${targetPort}`;

  return {
    plugins: [
      react(),
      nodePolyfills({
        // To add only specific polyfills, add them here. If no option is passed, adds all.
        include: ['buffer', 'process', 'util', 'events', 'stream'],
        globals: {
          Buffer: true,
          global: true,
          process: true,
        },
      }),
    ],
    server: {
      port: parseInt(process.env.PORT || env.PORT || 5173),
      proxy: {
        '/api': target,
        '/uploads': target,
        '/socket.io': {
          target: target,
          ws: true,
          changeOrigin: true,
        },
      },
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks: {
            'vendor-react': ['react', 'react-dom', 'react-router-dom'],
            'vendor-ui': ['@headlessui/react', 'lucide-react', 'framer-motion', 'sonner'],
            'vendor-charts': ['chart.js', 'react-chartjs-2'],
            'vendor-utils': ['lodash', 'dompurify', 'zod', 'socket.io-client', '@tanstack/react-query'],
            'vendor-misc': ['emoji-picker-react', '@hello-pangea/dnd']
          }
        }
      },
      chunkSizeWarningLimit: 1000
    }
  }
})
