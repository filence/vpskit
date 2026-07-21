import assert from "node:assert/strict";
import { test } from "node:test";
import worker from "./worker.mjs";

class MemoryKV {
  constructor() { this.values = new Map(); }
  async get(key) { return this.values.has(key) ? this.values.get(key) : null; }
  async put(key, value) { this.values.set(key, String(value)); }
  async delete(key) { this.values.delete(key); }
}

const NODE_ID = "node-main";
const PUBLISH_SECRET = "p".repeat(43);
const READ_TOKEN = "r".repeat(43);

test("publish, conditional GET, HEAD, token rotation, and rollback", async () => {
  const kv = new MemoryKV();
  const env = { SUBSCRIPTIONS: kv, NODE_ID, NODE_PUBLISH_SECRET: PUBLISH_SECRET, INITIAL_READ_TOKEN_HASH: await sha256Hex(text(READ_TOKEN)) };
  const first = publication("node-main-r0001", 1, "first");
  const publishResponse = await signedFetch(env, "/api/v1/nodes/node-main/publish", "POST", first);
  assert.equal(publishResponse.status, 200);
  assert.equal((await publishResponse.json()).status, "COMMITTED");

  const mihomo = await worker.fetch(new Request(`https://sub.example/s/${READ_TOKEN}/mihomo`), env);
  assert.equal(mihomo.status, 200);
  assert.equal(await mihomo.text(), "first-mihomo\n");
  assert.equal(mihomo.headers.get("X-VPSKit-Client-Revision"), "1");

  const conditional = await worker.fetch(new Request(`https://sub.example/s/${READ_TOKEN}/mihomo`, { headers: { "If-None-Match": mihomo.headers.get("ETag") } }), env);
  assert.equal(conditional.status, 304);

  const head = await worker.fetch(new Request(`https://sub.example/s/${READ_TOKEN}/v2rayn`, { method: "HEAD" }), env);
  assert.equal(head.status, 200);
  assert.equal(await head.text(), "");

  const invalid = await worker.fetch(new Request("https://sub.example/s/not-a-valid-token-value/mihomo"), env);
  assert.equal(invalid.status, 404);

  const newToken = "n".repeat(43);
  const rotate = await signedFetch(env, "/api/v1/nodes/node-main/read-token/rotate", "POST", {
    new_token_hash: await sha256Hex(text(newToken)),
    overlap_until: new Date(Date.now() + 60_000).toISOString(),
  });
  assert.equal(rotate.status, 200);
  assert.equal((await rotate.json()).status, "ROTATED");
  assert.equal((await worker.fetch(new Request(`https://sub.example/s/${newToken}/manifest`), env)).status, 200);
  assert.equal((await worker.fetch(new Request(`https://sub.example/s/${READ_TOKEN}/manifest`), env)).status, 200);

  const second = publication("node-main-r0002", 2, "second");
  assert.equal((await signedFetch(env, "/api/v1/nodes/node-main/publish", "POST", second)).status, 200);

  const idempotent = await signedFetch(env, "/api/v1/nodes/node-main/publish", "POST", second);
  assert.equal(idempotent.status, 200);
  assert.equal((await idempotent.json()).publication_id, "node-main-r0002");

  const stale = await signedFetch(env, "/api/v1/nodes/node-main/publish", "POST", first);
  assert.equal(stale.status, 409);
  const rollback = await signedFetch(env, "/api/v1/nodes/node-main/rollback", "POST", { publication_id: "node-main-r0001" });
  assert.equal(rollback.status, 200);
  const restored = await worker.fetch(new Request(`https://sub.example/s/${newToken}/mihomo`), env);
  assert.equal(await restored.text(), "first-mihomo\n");
});

test("rejects wrong signatures, replayed nonces, oversized bodies, and methods", async () => {
  const kv = new MemoryKV();
  const env = { SUBSCRIPTIONS: kv, NODE_ID, NODE_PUBLISH_SECRET: PUBLISH_SECRET, INITIAL_READ_TOKEN_HASH: await sha256Hex(text(READ_TOKEN)) };
  const payload = publication("node-main-r0001", 1, "first");
  for (const artifact of payload.artifacts) artifact.sha256 = await sha256Hex(base64ToBytes(artifact.content_base64));
  const body = text(JSON.stringify(payload));
  const bad = new Request("https://sub.example/api/v1/nodes/node-main/publish", { method: "POST", body, headers: authHeaders("wrong", "0".repeat(32), body, "/api/v1/nodes/node-main/publish") });
  assert.equal((await worker.fetch(bad, env)).status, 401);

  const nonce = "1".repeat(32);
  const requestOne = signedRequest("/api/v1/nodes/node-main/publish", "POST", body, nonce);
  assert.equal((await worker.fetch(requestOne, env)).status, 200);
  const replay = signedRequest("/api/v1/nodes/node-main/publish", "POST", body, nonce);
  assert.equal((await worker.fetch(replay, env)).status, 401);

  const tooLarge = new Request("https://sub.example/api/v1/nodes/node-main/publish", { method: "POST", body: "x", headers: { "Content-Length": String(1024 * 1024 + 1) } });
  assert.equal((await worker.fetch(tooLarge, env)).status, 413);
  assert.equal((await worker.fetch(new Request(`https://sub.example/s/${READ_TOKEN}/mihomo`, { method: "POST" }), env)).status, 405);

  const wrongType = signedRequest("/api/v1/nodes/node-main/publish", "POST", body, crypto.randomUUID().replaceAll("-", ""));
  wrongType.headers.set("Content-Type", "text/plain");
  assert.equal((await worker.fetch(wrongType, env)).status, 415);
});

test("revoking the initial token cannot reactivate its environment fallback", async () => {
  const kv = new MemoryKV();
  const env = { SUBSCRIPTIONS: kv, NODE_ID, NODE_PUBLISH_SECRET: PUBLISH_SECRET, INITIAL_READ_TOKEN_HASH: await sha256Hex(text(READ_TOKEN)) };
  const first = publication("node-main-r0001", 1, "first");
  assert.equal((await signedFetch(env, "/api/v1/nodes/node-main/publish", "POST", first)).status, 200);
  assert.equal((await worker.fetch(new Request(`https://sub.example/s/${READ_TOKEN}/manifest`), env)).status, 200);

  const revoke = await signedFetch(env, "/api/v1/nodes/node-main/read-token/revoke", "POST", {
    token_hash: await sha256Hex(text(READ_TOKEN)),
  });
  assert.equal(revoke.status, 200);
  assert.equal((await worker.fetch(new Request(`https://sub.example/s/${READ_TOKEN}/manifest`), env)).status, 404);
});

test("zero-overlap rotation invalidates the previous token", async () => {
  const kv = new MemoryKV();
  const env = { SUBSCRIPTIONS: kv, NODE_ID, NODE_PUBLISH_SECRET: PUBLISH_SECRET, INITIAL_READ_TOKEN_HASH: await sha256Hex(text(READ_TOKEN)) };
  const first = publication("node-main-r0001", 1, "first");
  assert.equal((await signedFetch(env, "/api/v1/nodes/node-main/publish", "POST", first)).status, 200);
  const newToken = "z".repeat(43);
  const rotate = await signedFetch(env, "/api/v1/nodes/node-main/read-token/rotate", "POST", {
    new_token_hash: await sha256Hex(text(newToken)),
    overlap_until: new Date().toISOString(),
  });
  assert.equal(rotate.status, 200);
  assert.equal((await worker.fetch(new Request(`https://sub.example/s/${newToken}/manifest`), env)).status, 200);
  assert.equal((await worker.fetch(new Request(`https://sub.example/s/${READ_TOKEN}/manifest`), env)).status, 404);
});

function publication(publicationID, revision, prefix) {
  const artifacts = [
    ["mihomo", "text/yaml; charset=utf-8", `${prefix}-mihomo\n`],
    ["v2rayn", "text/plain; charset=utf-8", `${prefix}-v2rayn\n`],
    ["manifest", "application/json", JSON.stringify({ publication_id: publicationID, node_revision: revision }) + "\n"],
  ];
  return {
    schema_version: 1,
    node_id: NODE_ID,
    node_revision: revision,
    ruleset_revision: 0,
    publication_id: publicationID,
    artifacts: artifacts.map(([target, media_type, content]) => ({
      target,
      media_type,
      sha256: "",
      content_base64: bytesToBase64(text(content)),
      renderer: target,
      renderer_version: 1,
      compatibility_profile: "test/v1",
    })),
  };
}

async function signedFetch(env, path, method, value) {
  const payload = structuredClone(value);
  if (Array.isArray(payload.artifacts)) {
    for (const artifact of payload.artifacts) artifact.sha256 = await sha256Hex(base64ToBytes(artifact.content_base64));
  }
  const body = text(JSON.stringify(payload));
  return worker.fetch(signedRequest(path, method, body, crypto.randomUUID().replaceAll("-", "")), env);
}

function signedRequest(path, method, body, nonce) {
  return new Request(`https://sub.example${path}`, { method, body, headers: authHeaders(PUBLISH_SECRET, nonce, body, path) });
}

function authHeaders(secret, nonce, body, path) {
  const timestamp = String(Math.floor(Date.now() / 1000));
  return new Headers({
    "Content-Type": "application/json",
    "X-VPSKit-Timestamp": timestamp,
    "X-VPSKit-Nonce": nonce,
    "X-VPSKit-Signature": hmacHexSync(secret, [timestamp, nonce, "POST", path, sha256HexSync(body)].join("\n")),
  });
}

function hmacHexSync(secret, value) {
  return globalThis.__testHmac(secret, value);
}

function sha256HexSync(value) {
  return globalThis.__testSha256(value);
}

function text(value) { return new TextEncoder().encode(value); }
function bytesToBase64(bytes) { return Buffer.from(bytes).toString("base64"); }
function base64ToBytes(value) { return new Uint8Array(Buffer.from(value, "base64")); }
async function sha256Hex(bytes) { return Buffer.from(await crypto.subtle.digest("SHA-256", bytes)).toString("hex"); }

const nodeCrypto = await import("node:crypto");
globalThis.__testHmac = (secret, value) => nodeCrypto.createHmac("sha256", secret).update(value).digest("hex");
globalThis.__testSha256 = (value) => nodeCrypto.createHash("sha256").update(value).digest("hex");
