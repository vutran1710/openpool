// Durable Object — holds one user's WebSocket connection
// Parses MessagePack frames (same wire format as Go client)

import { encode, decode } from "@msgpack/msgpack";

interface Env {
  RELAY_KV: KVNamespace;
  USER_SESSION: DurableObjectNamespace;
}

interface Frame {
  type: string;
  source_hash?: string;
  target_hash?: string;
  body?: string;
  ts?: number;
  encrypted?: boolean;
  msg_id?: string;
  pubkey?: string;
}

export class UserSession {
  state: DurableObjectState;
  env: Env;
  binHash: string = "";

  constructor(state: DurableObjectState, env: Env) {
    this.state = state;
    this.env = env;
  }

  async fetch(request: Request): Promise<Response> {
    const url = new URL(request.url);

    if (url.pathname === "/ws") {
      const pair = new WebSocketPair();
      const [client, server] = Object.values(pair);

      this.binHash = url.searchParams.get("bin") || "";
      this.state.acceptWebSocket(server);

      return new Response(null, { status: 101, webSocket: client });
    }

    if (url.pathname === "/forward") {
      // Another DO is sending a message to this user
      const data = new Uint8Array(await request.arrayBuffer());
      for (const ws of this.state.getWebSockets()) {
        try {
          ws.send(data);
        } catch {
          // dead connection
        }
      }
      return new Response("ok");
    }

    return new Response("not found", { status: 404 });
  }

  async webSocketMessage(ws: WebSocket, message: ArrayBuffer | string) {
    try {
      // Parse MessagePack frame
      const raw = typeof message === "string"
        ? new TextEncoder().encode(message)
        : new Uint8Array(message);

      const frame = decode(raw) as Frame;

      switch (frame.type) {
        case "msg":
          await this.handleMessage(frame);
          break;
        case "key_request":
          await this.handleKeyRequest(frame, ws);
          break;
        case "ack":
          break;
        default:
          this.sendError(ws, "internal_error", `unknown frame type: ${frame.type}`);
      }
    } catch (e) {
      // Parse error — try JSON fallback
      try {
        const text = typeof message === "string" ? message : new TextDecoder().decode(message);
        const frame = JSON.parse(text) as Frame;
        switch (frame.type) {
          case "msg": await this.handleMessage(frame); break;
          case "key_request": await this.handleKeyRequest(frame, ws); break;
        }
      } catch {
        // Truly malformed — ignore
      }
    }
  }

  async handleMessage(msg: Frame) {
    if (!msg.source_hash || !msg.target_hash) return;

    if (msg.source_hash !== this.binHash) {
      for (const ws of this.state.getWebSockets()) {
        this.sendError(ws, "auth_failed", "source_hash does not match session");
      }
      return;
    }

    const matched = await this.isMatched(msg.source_hash, msg.target_hash);
    if (!matched) {
      for (const ws of this.state.getWebSockets()) {
        this.sendError(ws, "not_matched", "users are not matched");
      }
      return;
    }

    const msgId = crypto.randomUUID().replace(/-/g, "").substring(0, 32);

    // Forward to target DO
    const targetId = this.env.USER_SESSION.idFromName(msg.target_hash);
    const targetStub = this.env.USER_SESSION.get(targetId);

    const outFrame: Frame = {
      type: "msg",
      msg_id: msgId,
      source_hash: msg.source_hash,
      target_hash: msg.target_hash,
      body: msg.body,
      ts: msg.ts || Math.floor(Date.now() / 1000),
      encrypted: msg.encrypted,
    };

    // Encode as MessagePack for the target client
    const encoded = encode(outFrame);

    await targetStub.fetch(new Request("http://internal/forward", {
      method: "POST",
      body: encoded,
    }));
  }

  async handleKeyRequest(req: Frame, ws: WebSocket) {
    if (!req.target_hash) {
      this.sendError(ws, "internal_error", "missing target_hash");
      return;
    }

    const matched = await this.isMatched(this.binHash, req.target_hash);
    if (!matched) {
      this.sendError(ws, "not_matched", "not matched with target");
      return;
    }

    const userData = await this.env.RELAY_KV.get(`user:${req.target_hash}`, "json") as {
      pubkey: string;
    } | null;

    if (!userData) {
      this.sendError(ws, "user_not_found", "target not found");
      return;
    }

    // Respond with MessagePack
    const response: Frame = {
      type: "key_response",
      target_hash: req.target_hash,
      pubkey: userData.pubkey,
    };
    ws.send(encode(response));
  }

  async isMatched(hash1: string, hash2: string): Promise<boolean> {
    const key = [hash1, hash2].sort().join(":");
    const result = await this.env.RELAY_KV.get(`match:${key}`);
    return result !== null;
  }

  sendError(ws: WebSocket, code: string, message: string) {
    const frame: Frame = { type: "error" };
    (frame as any).code = code;
    (frame as any).message = message;
    ws.send(encode(frame));
  }

  async webSocketClose() {}
  async webSocketError() {}
}
