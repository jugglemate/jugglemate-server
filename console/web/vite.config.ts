import { defineConfig } from "vite-plus";
import react from "@vitejs/plugin-react-swc";
import { tanstackRouter } from "@tanstack/router-plugin/vite";

export default defineConfig({
  base: "/jmateconsole/",
  plugins: [
    tanstackRouter({
      routesDirectory: "./src/routes",
      generatedRouteTree: "./src/routeTree.gen.ts",
    }),
    react(),
  ],
  resolve: {
    alias: {
      "@": "/src",
    },
  },
  server: {
    proxy: {
      "/jmate": {
        target: process.env.VITE_API_PROXY_TARGET ?? "http://localhost:8050",
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: (id: string) => {
          if (id.includes("node_modules/antd")) {
            return "vendor-antd";
          }
          if (id.includes("@tanstack/react-router") || id.includes("@tanstack/react-query")) {
            return "vendor-tanstack";
          }
          if (id.includes("lucide-react")) {
            return "vendor-ui";
          }
        },
      },
    },
    chunkSizeWarningLimit: 1024,
  },
  staged: {
    "*": "vp check --fix",
  },
  lint: { options: { typeAware: true, typeCheck: true } },
});
