import { mkdirSync, writeFileSync, readFileSync, readdirSync } from "node:fs";
import { createHash } from "node:crypto";
import { loadEnv } from 'vite';
import { securityHeaders } from './security-policy.mjs';
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const output = resolve(root, "dist/server/index.js");

const releaseHeaders = securityHeaders({ ...loadEnv('production', root, ['VITE_', 'REACT_APP_']), ...process.env });
const worker = `const securityHeaders = ${JSON.stringify(releaseHeaders)};

function secured(response) {
  const headers = new Headers(response.headers);
  for (const [name, value] of Object.entries(securityHeaders)) headers.set(name, value);
  if ((headers.get('content-type') || '').includes('text/html')) headers.set('Cache-Control', 'no-store');
  return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
}

export default {
  async fetch(request, env) {
    if (!env.ASSETS || typeof env.ASSETS.fetch !== "function") {
      return secured(new Response("PoDL Explorer asset binding is unavailable", { status: 503 }));
    }

    const response = await env.ASSETS.fetch(request);
    if (response.status !== 404 || request.method !== "GET") return secured(response);

    const acceptsHtml = (request.headers.get("accept") || "").includes("text/html");
    if (!acceptsHtml) return secured(response);

    const fallbackUrl = new URL(request.url);
    fallbackUrl.pathname = "/index.html";
    fallbackUrl.search = "";
    return secured(await env.ASSETS.fetch(new Request(fallbackUrl, request)));
  },
};
`;

mkdirSync(dirname(output), { recursive: true });
writeFileSync(output, worker, "utf8");

// Integrity hashes bind HTML imports to this build. They do not replace a
// trusted deployment account or independent review of the release itself.
const indexPath = resolve(root, 'dist/index.html');
let html = readFileSync(indexPath, 'utf8');
html = html.replace(/<(script|link)\b[^>]*(?:src|href)="(\/assets\/[^"?#]+)"[^>]*>/g, (tag, kind, url) => {
  const digest = createHash('sha384').update(readFileSync(resolve(root, 'dist', '.' + url))).digest('base64');
  return tag.replace(/\s+integrity="[^"]*"/g, '').replace(/>$/, ` integrity="sha384-${digest}">`);
});
writeFileSync(indexPath, html);
const manifest = {};
for (const file of ['index.html', 'server/index.js', ...readdirSync(resolve(root, 'dist/assets')).filter((name) => !name.endsWith('.map')).map((name) => `assets/${name}`)]) {
  manifest[file] = createHash('sha256').update(readFileSync(resolve(root, 'dist', file))).digest('hex');
}
writeFileSync(resolve(root, 'dist/release-integrity.json'), JSON.stringify({ version: 1, sha256: manifest }, null, 2));
