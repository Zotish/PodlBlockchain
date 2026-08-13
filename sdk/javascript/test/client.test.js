import test from "node:test";
import assert from "node:assert/strict";
import { PoDLClient, PoDLError, controlSigningPayload } from "../src/index.js";

test("client sends versioned suitability request", async () => {
  let seen;
  const client = new PoDLClient("http://node/", { fetchImpl: async (url, options) => {
    seen = { url, options };
    return { ok: true, text: async () => '{"eligible":true}' };
  }});
  const result = await client.suitability({ loss_tolerance_bps: 1000 });
  assert.equal(seen.url, "http://node/v2/product/suitability");
  assert.equal(seen.options.method, "POST");
  assert.equal(result.eligible, true);
});

test("control signing payload commits nonce, type and extra data", () => {
  const payload = controlSigningPayload({ from: "0xAA", to: "0xBB", value: "0", gas: 21000, gas_price: 1, chain_id: 50341, timestamp: 10, nonce: 2, type: "oracle_update", extra_data_hex: "7b7d" });
  assert.match(payload, /PODL-CONTROL-TX-V2/);
  assert.match(payload, /"nonce":2/);
  assert.match(payload, /"extra_data_hex":"7b7d"/);
});

test("client exposes structured HTTP errors", async () => {
  const client = new PoDLClient("http://node", { fetchImpl: async () => ({ ok: false, status: 429, text: async () => '{"error":"limited"}' }) });
  await assert.rejects(() => client.protocolStatus(), (error) => error instanceof PoDLError && error.status === 429);
});

test("client exposes concentrated-position lifecycle calls", async () => {
  const bodies = [];
  const client = new PoDLClient("http://node", { fetchImpl: async (_url, options) => {
    bodies.push(JSON.parse(options.body));
    return { ok: true, text: async () => '{"ok":true}' };
  }});
  await client.mintConcentratedPosition({ pool: "0xpool", from: "0xowner", lowerSqrtX18: 1, upperSqrtX18: 2, amount0: 3, amount1: 4 });
  await client.transferConcentratedPosition({ pool: "0xpool", from: "0xowner", id: "position_1", to: "0xnext" });
  await client.collectConcentratedPositionFees({ pool: "0xpool", from: "0xnext", id: "position_1" });
  await client.burnConcentratedPosition({ pool: "0xpool", from: "0xnext", id: "position_1" });
  assert.deepEqual(bodies.map((body) => body.function), ["MintConcentratedPosition", "TransferPosition", "CollectPositionFees", "BurnConcentratedPosition"]);
  assert.deepEqual(bodies[0].args, ["1", "2", "3", "4"]);
});
