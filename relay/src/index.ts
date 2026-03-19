// Cloudflare Worker — handles TOTP auth, WebSocket upgrade, match/sync endpoints
// Message routing is DO-to-DO (no Worker involvement after upgrade)

import { totpVerify } from "./totp";

export { UserSession } from "./session";

interface Env {
  RELAY_KV: KVNamespace;
  USER_SESSION: DurableObjectNamespace;
  POOL_SALT: string;
  POOL_URL: string;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    switch (url.pathname) {
      case "/ws":
        return handleWebSocket(request, url, env);
      case "/health":
        return Response.json({ status: "ok", pool: env.POOL_URL });
      case "/match":
        return handleMatch(request, env);
      case "/sync":
        return handleSync(request, env);
      default:
        return new Response("not found", { status: 404 });
    }
  },
};

// WebSocket upgrade with TOTP auth
async function handleWebSocket(request: Request, url: URL, env: Env): Promise<Response> {
  const binHash = url.searchParams.get("bin");
  const sig = url.searchParams.get("sig");

  if (!binHash || !sig) {
    return new Response("missing bin or sig", { status: 401 });
  }

  // Look up user in KV
  const userData = await env.RELAY_KV.get(`user:${binHash}`, "json") as {
    pubkey: string;
    match_hash: string;
  } | null;

  if (!userData) {
    return new Response("unknown user", { status: 401 });
  }

  // Verify TOTP
  const valid = await totpVerify(binHash, userData.match_hash, sig, userData.pubkey);
  if (!valid) {
    return new Response("invalid signature", { status: 401 });
  }

  // Must be a WebSocket upgrade request
  const upgradeHeader = request.headers.get("Upgrade");
  if (!upgradeHeader || upgradeHeader.toLowerCase() !== "websocket") {
    return new Response("auth ok — send WebSocket upgrade to connect", { status: 200 });
  }

  // Forward to user's Durable Object
  const doId = env.USER_SESSION.idFromName(binHash);
  const doStub = env.USER_SESSION.get(doId);

  const doUrl = new URL(request.url);
  doUrl.pathname = "/ws";
  return doStub.fetch(new Request(doUrl.toString(), request));
}

// Store match pair — called by GitHub Action
async function handleMatch(request: Request, env: Env): Promise<Response> {
  if (request.method !== "POST") {
    return new Response("method not allowed", { status: 405 });
  }

  const { bin_hash_1, bin_hash_2 } = await request.json() as {
    bin_hash_1: string;
    bin_hash_2: string;
  };

  if (!bin_hash_1 || !bin_hash_2) {
    return new Response("missing bin hashes", { status: 400 });
  }

  // Store match pair
  const matchKey = [bin_hash_1, bin_hash_2].sort().join(":");
  await env.RELAY_KV.put(`match:${matchKey}`, "1");

  return Response.json({ ok: true, match: matchKey });
}

// Store user data — called to sync user index
async function handleSync(request: Request, env: Env): Promise<Response> {
  if (request.method !== "POST") {
    return new Response("method not allowed", { status: 405 });
  }

  const { bin_hash, pubkey, match_hash } = await request.json() as {
    bin_hash: string;
    pubkey: string;
    match_hash: string;
  };

  if (!bin_hash || !pubkey || !match_hash) {
    return new Response("missing fields", { status: 400 });
  }

  await env.RELAY_KV.put(`user:${bin_hash}`, JSON.stringify({ pubkey, match_hash }));

  return Response.json({ ok: true });
}
