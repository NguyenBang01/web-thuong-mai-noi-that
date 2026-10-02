// @ts-check
import { defineConfig } from 'astro/config';

import react from '@astrojs/react';

import tailwindcss from '@tailwindcss/vite';

// https://astro.build/config
export default defineConfig({
  output: 'server', // <-- Thêm duy nhất dòng này để bật SSR cho toàn bộ trang

  integrations: [react()],

  vite: {
    plugins: [tailwindcss()]
  }
});