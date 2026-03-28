import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    include: ['src/test/**/*.test.{ts,tsx}'],
    setupFiles: 'src/test/setup.ts'
  }
});
