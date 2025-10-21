import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import {viteSingleFile} from 'vite-plugin-singlefile'

// See https://vitejs.dev/config/ for more details.
export default defineConfig({
    plugins: [vue(), viteSingleFile()],
})
