import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: (id) => {
          // monaco-editor is bundled locally (see src/monacoSetup.ts); keep it
          // out of the main chunk so startup parse cost stays unchanged.
          if (id.includes('node_modules/monaco-editor')) {
            return 'monaco-editor'
          }
        },
      },
    },
  },
})
