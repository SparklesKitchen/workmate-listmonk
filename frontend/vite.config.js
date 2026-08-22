import vue from '@vitejs/plugin-vue2';
import { defineConfig, loadEnv } from 'vite';

const path = require('path');

// https://vitejs.dev/config/
export default defineConfig(({ _, mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  return {
    plugins: [vue()],
    // WorkMate serves the owner console below its authenticated SaaS Admin
    // namespace. Keep the default for ordinary upstream installations.
    base: env.LISTMONK_ADMIN_BASE_PATH || '/admin',
    mode,
    define: {
      'import.meta.env.VUE_APP_ROOT_URL': JSON.stringify(env.LISTMONK_API_ROOT_URL || '/'),
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        bulma: require.resolve('bulma/bulma.sass'),
      },
    },
    build: {
      assetsDir: 'static',
    },
    server: {
      port: env.LISTMONK_FRONTEND_PORT || 8080,
      proxy: {
        '^/$': {
          target: env.LISTMONK_API_URL || 'http://127.0.0.1:9000',
        },
        '^/(api|webhooks|subscription|public|health)': {
          target: env.LISTMONK_API_URL || 'http://127.0.0.1:9000',
        },
        '^/admin/login': {
          target: env.LISTMONK_API_URL || 'http://127.0.0.1:9000',
        },
        '^/(admin\/custom\.(css|js))': {
          target: env.LISTMONK_API_URL || 'http://127.0.0.1:9000',
        },
      },
    },
  };
});
