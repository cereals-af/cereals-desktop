import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  plugins: [react()],
  base: './',
  resolve: {
    alias: {
      // Monaco 0.57 initializes workers before receiving the first create-data message.
      'monaco-worker-manager/worker': 'monaco-editor/editor/editor.worker',
      // Monaco 0.57 exports JS entry points but omits its icon CSS export.
      '@monaco-codicons.css': fileURLToPath(
        new URL(
          './node_modules/monaco-editor/esm/vs/base/browser/ui/codicons/codicon/codicon.css',
          import.meta.url,
        ),
      ),
    },
  },
  optimizeDeps: {
    include: [
      '@monaco-editor/react',
      'monaco-editor/editor/editor.api',
      'monaco-editor/languages/definitions/yaml/register',
      'monaco-editor/features/suggest/register',
      'monaco-editor/editor/contrib/suggest/browser/suggestController',
      'monaco-editor/features/hover/register',
      'monaco-yaml',
      'monaco-yaml/yaml.worker.js',
      'path-browserify',
    ],
  },
  server: { strictPort: true, port: Number(process.env.WAILS_VITE_PORT) || 5173 },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
});
