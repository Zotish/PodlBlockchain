export function securityHeaders(env = {}) {
  const defaults = { API_BASE: 'https://api.178-105-133-94.sslip.io', CHAIN_BASE: 'https://chain.178-105-133-94.sslip.io', WALLET_BASE: 'https://wallet.178-105-133-94.sslip.io', DEX_REGISTRY_API: 'https://dex-api.178-105-133-94.sslip.io' };
  const origins = new Set();
  for (const [key, fallback] of Object.entries(defaults)) {
    const url = new URL(env[`REACT_APP_${key}`] || env[`VITE_${key}`] || fallback);
    if (url.username || url.password || url.protocol !== 'https:') throw new Error(`${key} must be an HTTPS endpoint without credentials`);
    origins.add(url.origin);
  }
  return {
    'Content-Security-Policy': `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data: https:; font-src 'self' https://fonts.gstatic.com; connect-src 'self' ${[...origins].sort().join(' ')}; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; upgrade-insecure-requests`,
    'Referrer-Policy': 'no-referrer',
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=(), payment=(), usb=()',
    'Strict-Transport-Security': 'max-age=31536000',
  };
}
