import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import { writeFileSync } from "node:fs";

function keepDistPlaceholder(): Plugin {
  return {
    name: "keep-dist-placeholder",
    closeBundle() {
      writeFileSync("dist/.gitkeep", "");
    },
  };
}

export default defineConfig({
  plugins: [react(), keepDistPlaceholder()],
  server: {
    port: 5173,
    strictPort: true,
    hmr: { host: "localhost", port: 5173, clientPort: 5173 },
  },
  build: { outDir: "dist", emptyOutDir: true, target: "es2022", chunkSizeWarningLimit: 900 },
});
