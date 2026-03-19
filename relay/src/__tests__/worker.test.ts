import { describe, it, expect, beforeAll } from "vitest";
import { env, SELF } from "cloudflare:test";

describe("Worker HTTP endpoints", () => {
  describe("GET /health", () => {
    it("returns ok status", async () => {
      const response = await SELF.fetch("http://localhost/health");
      expect(response.status).toBe(200);
      const body = await response.json() as { status: string };
      expect(body.status).toBe("ok");
    });
  });

  describe("GET /unknown", () => {
    it("returns 404", async () => {
      const response = await SELF.fetch("http://localhost/unknown");
      expect(response.status).toBe(404);
    });
  });

  describe("GET /ws — auth", () => {
    it("returns 401 without params", async () => {
      const response = await SELF.fetch("http://localhost/ws");
      expect(response.status).toBe(401);
      const text = await response.text();
      expect(text).toContain("missing");
    });

    it("returns 401 with missing sig", async () => {
      const response = await SELF.fetch("http://localhost/ws?bin=abc123");
      expect(response.status).toBe(401);
    });

    it("returns 401 with missing bin", async () => {
      const response = await SELF.fetch("http://localhost/ws?sig=abc123");
      expect(response.status).toBe(401);
    });

    it("returns 401 for unknown user", async () => {
      const response = await SELF.fetch("http://localhost/ws?bin=unknown&sig=abcd");
      expect(response.status).toBe(401);
      const text = await response.text();
      expect(text).toContain("unknown user");
    });

    it("returns 401 for invalid signature", async () => {
      // First sync a user
      await SELF.fetch("http://localhost/sync", {
        method: "POST",
        body: JSON.stringify({
          bin_hash: "test_bin_hash123",
          pubkey: "4cb5abf6ad79fbf5abbccafcc269d85cd2651ed4b885b5869f241aedf0a5ba29",
          match_hash: "test_match_hash1",
        }),
      });

      // Try with wrong sig
      const response = await SELF.fetch(
        "http://localhost/ws?bin=test_bin_hash123&sig=" + "00".repeat(64)
      );
      expect(response.status).toBe(401);
      const text = await response.text();
      expect(text).toContain("invalid signature");
    });
  });

  describe("POST /sync", () => {
    it("stores user data", async () => {
      const response = await SELF.fetch("http://localhost/sync", {
        method: "POST",
        body: JSON.stringify({
          bin_hash: "sync_test_bin",
          pubkey: "aabbccdd" + "00".repeat(28),
          match_hash: "sync_test_match",
        }),
      });
      expect(response.status).toBe(200);
      const body = await response.json() as { ok: boolean };
      expect(body.ok).toBe(true);
    });

    it("rejects missing fields", async () => {
      const response = await SELF.fetch("http://localhost/sync", {
        method: "POST",
        body: JSON.stringify({ bin_hash: "only_bin" }),
      });
      expect(response.status).toBe(400);
    });

    it("rejects GET", async () => {
      const response = await SELF.fetch("http://localhost/sync");
      expect(response.status).toBe(405);
    });
  });

  describe("POST /match", () => {
    it("stores match pair", async () => {
      const response = await SELF.fetch("http://localhost/match", {
        method: "POST",
        body: JSON.stringify({
          bin_hash_1: "alice_bin",
          bin_hash_2: "bob_bin",
        }),
      });
      expect(response.status).toBe(200);
      const body = await response.json() as { ok: boolean; match: string };
      expect(body.ok).toBe(true);
      expect(body.match).toContain("alice_bin");
    });

    it("rejects missing fields", async () => {
      const response = await SELF.fetch("http://localhost/match", {
        method: "POST",
        body: JSON.stringify({ bin_hash_1: "only_one" }),
      });
      expect(response.status).toBe(400);
    });

    it("rejects GET", async () => {
      const response = await SELF.fetch("http://localhost/match");
      expect(response.status).toBe(405);
    });
  });
});
