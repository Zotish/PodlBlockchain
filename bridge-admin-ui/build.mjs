import fs from "node:fs/promises";
import path from "node:path";

const root = new URL(".", import.meta.url);
const distDir = new URL("./dist/", root);

const filesToCopy = ["index.html", "app.js", "styles.css"];

await fs.rm(distDir, { recursive: true, force: true });
await fs.mkdir(distDir, { recursive: true });

for (const file of filesToCopy) {
  const from = new URL(file, root);
  const to = new URL(file, distDir);
  await fs.copyFile(from, to);
}

const defaultNodeUrl = (process.env.BRIDGE_ADMIN_NODE_URL || "https://chain.178-105-133-94.sslip.io").trim();
const runtimeConfig = `window.__BRIDGE_ADMIN_CONFIG__ = ${JSON.stringify(
  { defaultNodeUrl },
  null,
  2
)};\n`;

await fs.writeFile(new URL("./runtime-config.js", distDir), runtimeConfig, "utf8");
