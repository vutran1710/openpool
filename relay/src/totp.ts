// TOTP verification using tweetnacl (ed25519)
// Same algorithm as Go: sha256(binHash + matchHash + timeWindow) → ed25519 verify

import nacl from "tweetnacl";

export async function totpVerify(
  binHash: string,
  matchHash: string,
  sigHex: string,
  pubkeyHex: string
): Promise<boolean> {
  const sigBytes = hexToBytes(sigHex);
  if (sigBytes.length !== 64) return false;

  const pubBytes = hexToBytes(pubkeyHex);
  if (pubBytes.length !== 32) return false;

  const now = Math.floor(Date.now() / 1000 / 300);
  for (const tw of [now, now - 1, now + 1]) {
    const msg = await totpMessage(binHash, matchHash, tw);
    if (nacl.sign.detached.verify(new Uint8Array(msg), sigBytes, pubBytes)) {
      return true;
    }
  }

  return false;
}

async function totpMessage(binHash: string, matchHash: string, tw: number): Promise<ArrayBuffer> {
  const input = new TextEncoder().encode(binHash + matchHash + tw.toString());
  return crypto.subtle.digest("SHA-256", input);
}

export function hexToBytes(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.substring(i, i + 2), 16);
  }
  return bytes;
}

export function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}
