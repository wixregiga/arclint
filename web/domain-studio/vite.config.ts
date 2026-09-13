import { defineConfig } from 'vite';
import { arclintBridgePlugin } from './server/arclint_bridge';

export default defineConfig({
  plugins: [arclintBridgePlugin()],
  server: { host: '127.0.0.1' },
  preview: { host: '127.0.0.1' },
});
