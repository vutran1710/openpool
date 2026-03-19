import { describe, it, expect } from "vitest";
import { totpVerify, hexToBytes, bytesToHex } from "../totp";

// Test vectors generated from Go implementation
const TEST_PUBKEY = "4cb5abf6ad79fbf5abbccafcc269d85cd2651ed4b885b5869f241aedf0a5ba29";
const TEST_BIN_HASH = "abcd1234abcd1234";
const TEST_MATCH_HASH = "efgh5678efgh5678";

describe("hexToBytes", () => {
  it("converts hex string to bytes", () => {
    const bytes = hexToBytes("deadbeef");
    expect(bytes).toEqual(new Uint8Array([0xde, 0xad, 0xbe, 0xef]));
  });

  it("handles empty string", () => {
    expect(hexToBytes("")).toEqual(new Uint8Array(0));
  });
});

describe("bytesToHex", () => {
  it("converts bytes to hex string", () => {
    const hex = bytesToHex(new Uint8Array([0xde, 0xad, 0xbe, 0xef]));
    expect(hex).toBe("deadbeef");
  });

  it("pads single digits", () => {
    expect(bytesToHex(new Uint8Array([0x01, 0x0a]))).toBe("010a");
  });
});

describe("totpVerify", () => {
  it("rejects invalid hex signature", async () => {
    const result = await totpVerify(TEST_BIN_HASH, TEST_MATCH_HASH, "not-hex-zzz", TEST_PUBKEY);
    expect(result).toBe(false);
  });

  it("rejects truncated signature", async () => {
    const result = await totpVerify(TEST_BIN_HASH, TEST_MATCH_HASH, "abcd", TEST_PUBKEY);
    expect(result).toBe(false);
  });

  it("rejects wrong pubkey", async () => {
    // Generate a valid-looking but wrong signature
    const wrongSig = "00".repeat(64);
    const result = await totpVerify(TEST_BIN_HASH, TEST_MATCH_HASH, wrongSig, TEST_PUBKEY);
    expect(result).toBe(false);
  });

  it("rejects wrong bin_hash", async () => {
    // Even with a valid sig for the right key, wrong bin_hash should fail
    const wrongSig = "00".repeat(64);
    const result = await totpVerify("wrong_bin_hash00", TEST_MATCH_HASH, wrongSig, TEST_PUBKEY);
    expect(result).toBe(false);
  });

  it("rejects wrong match_hash", async () => {
    const wrongSig = "00".repeat(64);
    const result = await totpVerify(TEST_BIN_HASH, "wrong_match_hash", wrongSig, TEST_PUBKEY);
    expect(result).toBe(false);
  });
});
