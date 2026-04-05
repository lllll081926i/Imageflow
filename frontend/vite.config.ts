import path from 'path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(() => {
    return {
        server: {
            port: 3000,
            host: '0.0.0.0',
        },
        build: {
            outDir: '../backend/frontend/dist',
            emptyOutDir: true,
        },
        plugins: [react()],
        resolve: {
            alias: {
                '@': path.resolve(__dirname, '.'),
            }
        }
    };
});
