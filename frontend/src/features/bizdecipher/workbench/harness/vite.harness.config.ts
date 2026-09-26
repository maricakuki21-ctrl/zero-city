import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

const frontendRoot = resolve(__dirname, '../../../../..')

export default defineConfig({
  root: frontendRoot,
  plugins: [vue()],
  resolve: {
    alias: [
      { find: '@/components/layout/AppLayout.vue', replacement: resolve(frontendRoot, 'src/features/bizdecipher/workbench/harness/AppLayoutHarnessStub.vue') },
      { find: '@/api/client', replacement: resolve(frontendRoot, 'src/features/bizdecipher/workbench/harness/ApiClientHarnessStub.ts') },
      { find: '@', replacement: resolve(frontendRoot, 'src') },
      { find: 'vue-i18n', replacement: 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js' }
    ]
  },
  define: {
    __INTLIFY_JIT_COMPILATION__: true
  },
  server: {
    host: '127.0.0.1',
    port: 4173,
    open: '/src/features/bizdecipher/workbench/harness/index.html'
  }
})
