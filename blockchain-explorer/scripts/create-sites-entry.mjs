import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const output = resolve(root, "dist/server/index.js");

const worker = `const securityHeaders = {
  "Referrer-Policy": "strict-origin-when-cross-origin",
  "X-Content-Type-Options": "nosniff",
  "X-Frame-Options": "SAMEORIGIN",
};

function secured(response) {
  const headers = new Headers(response.headers);
  for (const [name, value] of Object.entries(securityHeaders)) headers.set(name, value);
  return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
}

export default {
  async fetch(request, env) {
    if (!env.ASSETS || typeof env.ASSETS.fetch !== "function") {
      return new Response("PoDL Explorer asset binding is unavailable", { status: 503 });
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
