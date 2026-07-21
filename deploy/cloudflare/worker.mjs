const MAX_BODY_BYTES = 1024 * 1024;
const MAX_CLOCK_SKEW_SECONDS = 300;
const NONCE_TTL_SECONDS = 600;
const TARGETS = new Set(["mihomo", "v2rayn", "manifest"]);
const HEX_64 = /^[a-f0-9]{64}$/;
const PUBLICATION_ID = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const nodePublishQueues = new Map();

export default {
  async fetch(request, env) {
    try {
      return await route(request, env);
    } catch {
      return jsonResponse({ status: "ERROR" }, 500);
    }
  },
};

async function route(request, env) {
  const url = new URL(request.url);
  if (url.pathname === "/healthz") {
    if (request.method !== "GET" && request.method !== "HEAD") {
      return methodNotAllowed("GET, HEAD");
    }
    return responseForMethod(request, JSON.stringify({ status: "PASS", schema_version: 1 }) + "\n", {
      "Content-Type": "application/json; charset=utf-8",
      "Cache-Control": "no-store",
    });
  }

  const subscription = url.pathname.match(/^\/s\/([^/]+)\/(mihomo|v2rayn|manifest)$/);
  if (subscription) {
    if (request.method !== "GET" && request.method !== "HEAD") {
      return methodNotAllowed("GET, HEAD");
    }
    return serveSubscription(request, env, subscription[1], subscription[2]);
  }

  const api = url.pathname.match(/^\/api\/v1\/nodes\/([^/]+)\/(publish|activate|rollback|read-token\/rotate|read-token\/revoke|status)$/);
  if (!api) {
    return notFound();
  }
  if (decodeURIComponent(api[1]) !== env.NODE_ID) {
    return notFound();
  }
  if ((api[2] === "status" && request.method !== "GET") || (api[2] !== "status" && request.method !== "POST")) {
    return methodNotAllowed(api[2] === "status" ? "GET" : "POST");
  }

  const body = request.method === "POST" ? await readLimitedBody(request) : new Uint8Array();
  if (body === null) {
    return jsonResponse({ status: "REJECTED", reason: "BODY_TOO_LARGE" }, 413);
  }
  if (request.method === "POST" && !/^application\/json(?:\s*;|$)/i.test(request.headers.get("Content-Type") || "")) {
    return jsonResponse({ status: "REJECTED", reason: "UNSUPPORTED_CONTENT_TYPE" }, 415);
  }
  const authenticated = await authenticateNodeRequest(request, env, body);
  if (!authenticated) {
    return jsonResponse({ status: "REJECTED" }, 401);
  }

  switch (api[2]) {
    case "publish":
      return serializeNodePublish(env.NODE_ID, () => publishArtifacts(body, env));
    case "activate":
      return activatePublication(body, env);
    case "rollback":
      return activatePublication(body, env);
    case "read-token/rotate":
      return rotateReadToken(body, env);
    case "read-token/revoke":
      return revokeReadToken(body, env);
    case "status":
      return publicationStatus(env);
    default:
      return notFound();
  }
}

async function publishArtifacts(body, env) {
  let payload;
  try {
    payload = JSON.parse(new TextDecoder().decode(body));
  } catch {
    return jsonResponse({ status: "REJECTED", reason: "INVALID_JSON" }, 400);
  }
  const validation = validatePublication(payload, env.NODE_ID);
  if (validation !== "") {
    return jsonResponse({ status: "REJECTED", reason: validation }, 400);
  }

  const decoded = new Map();
  for (const artifact of payload.artifacts) {
    let bytes;
    try {
      bytes = base64ToBytes(artifact.content_base64);
    } catch {
      return jsonResponse({ status: "REJECTED", reason: "INVALID_BASE64" }, 400);
    }
    if ((await sha256Hex(bytes)) !== artifact.sha256) {
      return jsonResponse({ status: "REJECTED", reason: "DIGEST_MISMATCH" }, 400);
    }
    decoded.set(artifact.target, { bytes, artifact });
  }

  const currentText = await env.SUBSCRIPTIONS.get("publication/current");
  const current = parseJSONOrNull(currentText);
  if (current && Number.isInteger(current.node_revision)) {
    if (current.node_revision > payload.node_revision) {
      return jsonResponse({ status: "REJECTED", reason: "STALE_REVISION" }, 409);
    }
    if (current.node_revision === payload.node_revision) {
      if (current.publication_id === payload.publication_id && await storedPublicationMatches(payload, env)) {
        return jsonResponse({
          status: "COMMITTED",
          publication_id: current.publication_id,
          node_revision: current.node_revision,
          published_at: current.published_at,
          targets: [...TARGETS],
        });
      }
      return jsonResponse({ status: "REJECTED", reason: "REVISION_CONFLICT" }, 409);
    }
  }

  const publishedAt = new Date().toISOString();
  for (const [target, item] of decoded) {
    const record = artifactRecord(payload, item.artifact, publishedAt);
    const key = revisionKey(target, payload.publication_id);
    const existing = parseJSONOrNull(await env.SUBSCRIPTIONS.get(key));
    if (existing && !artifactRecordsEquivalent(existing, record)) {
      return jsonResponse({ status: "REJECTED", reason: "IMMUTABLE_REVISION_CONFLICT" }, 409);
    }
    if (!existing) {
      await env.SUBSCRIPTIONS.put(key, JSON.stringify(record));
    }
  }
  for (const target of TARGETS) {
    const recordText = await env.SUBSCRIPTIONS.get(revisionKey(target, payload.publication_id));
    if (!recordText) {
      return jsonResponse({ status: "DEGRADED", publication_id: payload.publication_id }, 503);
    }
  }

  await env.SUBSCRIPTIONS.put(`node/${env.NODE_ID}/candidate/${payload.publication_id}`, JSON.stringify({
    schema_version: 1,
    node_id: env.NODE_ID,
    node_revision: payload.node_revision,
    ruleset_revision: payload.ruleset_revision,
    publication_id: payload.publication_id,
    status: "VALIDATED",
  }));

  // Re-check after candidate writes. This prevents a slower stale request in
  // the same isolate from replacing a revision that won the race.
  const latest = parseJSONOrNull(await env.SUBSCRIPTIONS.get("publication/current"));
  if (latest && latest.node_revision >= payload.node_revision) {
    if (latest.node_revision === payload.node_revision && latest.publication_id === payload.publication_id && await storedPublicationMatches(payload, env)) {
      return jsonResponse({ status: "COMMITTED", publication_id: latest.publication_id, node_revision: latest.node_revision, published_at: latest.published_at, targets: [...TARGETS] });
    }
    return jsonResponse({ status: "REJECTED", reason: "STALE_REVISION" }, 409);
  }
  for (const target of TARGETS) {
    const recordText = await env.SUBSCRIPTIONS.get(revisionKey(target, payload.publication_id));
    await env.SUBSCRIPTIONS.put(currentKey(target), recordText);
  }
  await env.SUBSCRIPTIONS.put("publication/current", JSON.stringify({
    schema_version: 1,
    node_id: payload.node_id,
    node_revision: payload.node_revision,
    ruleset_revision: payload.ruleset_revision,
    publication_id: payload.publication_id,
    published_at: publishedAt,
    targets: [...TARGETS],
  }));
  await env.SUBSCRIPTIONS.put(`node/${env.NODE_ID}/active`, JSON.stringify({
    schema_version: 1,
    node_id: env.NODE_ID,
    node_revision: payload.node_revision,
    ruleset_revision: payload.ruleset_revision,
    publication_id: payload.publication_id,
    activated_at: publishedAt,
  }));
  return jsonResponse({
    status: "COMMITTED",
    publication_id: payload.publication_id,
    node_revision: payload.node_revision,
    published_at: publishedAt,
    targets: [...TARGETS],
  });
}

function serializeNodePublish(nodeID, operation) {
  const previous = nodePublishQueues.get(nodeID) || Promise.resolve();
  const current = previous.catch(() => undefined).then(operation);
  nodePublishQueues.set(nodeID, current);
  return current.finally(() => {
    if (nodePublishQueues.get(nodeID) === current) {
      nodePublishQueues.delete(nodeID);
    }
  });
}

async function storedPublicationMatches(payload, env) {
  for (const artifact of payload.artifacts) {
    const stored = parseJSONOrNull(await env.SUBSCRIPTIONS.get(revisionKey(artifact.target, payload.publication_id)));
    if (!stored || stored.sha256 !== artifact.sha256 || stored.node_revision !== payload.node_revision || stored.ruleset_revision !== payload.ruleset_revision) {
      return false;
    }
  }
  return true;
}

function artifactRecordsEquivalent(left, right) {
  for (const key of ["schema_version", "node_id", "node_revision", "ruleset_revision", "publication_id", "target", "media_type", "sha256", "content_base64", "renderer", "renderer_version", "compatibility_profile"]) {
    if (left?.[key] !== right?.[key]) return false;
  }
  return true;
}

function parseJSONOrNull(value) {
  if (!value) return null;
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
}

async function activatePublication(body, env) {
  let input;
  try {
    input = JSON.parse(new TextDecoder().decode(body));
  } catch {
    return jsonResponse({ status: "REJECTED", reason: "INVALID_JSON" }, 400);
  }
  if (!input || !PUBLICATION_ID.test(input.publication_id || "")) {
    return jsonResponse({ status: "REJECTED", reason: "INVALID_PUBLICATION_ID" }, 400);
  }
  const records = new Map();
  for (const target of TARGETS) {
    const value = await env.SUBSCRIPTIONS.get(revisionKey(target, input.publication_id));
    if (!value) {
      return jsonResponse({ status: "NOT_FOUND" }, 404);
    }
    records.set(target, value);
  }
  for (const [target, value] of records) {
    await env.SUBSCRIPTIONS.put(currentKey(target), value);
  }
  const manifest = JSON.parse(records.get("manifest"));
  await env.SUBSCRIPTIONS.put("publication/current", JSON.stringify({
    schema_version: 1,
    node_id: env.NODE_ID,
    node_revision: manifest.node_revision,
    ruleset_revision: manifest.ruleset_revision,
    publication_id: input.publication_id,
    published_at: new Date().toISOString(),
    targets: [...TARGETS],
  }));
  return jsonResponse({ status: "ACTIVATED", publication_id: input.publication_id });
}

async function serveSubscription(request, env, encodedToken, target) {
  let token;
  try {
    token = decodeURIComponent(encodedToken);
  } catch {
    return notFound();
  }
  if (!(await validReadToken(token, env))) {
    return notFound();
  }
  const recordText = await env.SUBSCRIPTIONS.get(currentKey(target));
  if (!recordText) {
    return notFound();
  }
  let record;
  try {
    record = JSON.parse(recordText);
  } catch {
    return jsonResponse({ status: "DEGRADED" }, 503);
  }
  const etag = `"${record.sha256}"`;
  const headers = {
    "Content-Type": record.media_type,
    "Cache-Control": "private, no-store",
    ETag: etag,
    "Last-Modified": new Date(record.published_at).toUTCString(),
    "X-VPSKit-Client-Revision": String(record.node_revision),
    "X-VPSKit-Ruleset-Revision": String(record.ruleset_revision),
    "X-VPSKit-Renderer-Version": String(record.renderer_version),
    "X-Content-Type-Options": "nosniff",
  };
  if (request.headers.get("If-None-Match") === etag) {
    return new Response(null, { status: 304, headers });
  }
  return responseForMethod(request, base64ToBytes(record.content_base64), headers);
}

async function rotateReadToken(body, env) {
  let input;
  try {
    input = JSON.parse(new TextDecoder().decode(body));
  } catch {
    return jsonResponse({ status: "REJECTED", reason: "INVALID_JSON" }, 400);
  }
  const overlapUntil = Date.parse(input?.overlap_until || "");
  if (!HEX_64.test(input?.new_token_hash || "") || !Number.isFinite(overlapUntil)) {
    return jsonResponse({ status: "REJECTED", reason: "INVALID_ROTATION" }, 400);
  }
  const now = Date.now();
  // A zero-overlap rotation is encoded by the client as "now". Permit a
  // small transport delay, but never allow a meaningful backdated window.
  if (overlapUntil < now - 30_000 || overlapUntil > now + 7 * 24 * 60 * 60 * 1000) {
    return jsonResponse({ status: "REJECTED", reason: "INVALID_OVERLAP" }, 400);
  }
  const current = await currentReadTokenHash(env);
  if (current && overlapUntil > now) {
    await env.SUBSCRIPTIONS.put("read-token/previous", JSON.stringify({ hash: current, expires_at: new Date(overlapUntil).toISOString() }));
  } else {
    await env.SUBSCRIPTIONS.delete("read-token/previous");
  }
  await env.SUBSCRIPTIONS.put("read-token/initialized", "1");
  await env.SUBSCRIPTIONS.put("read-token/current-hash", input.new_token_hash);
  return jsonResponse({ status: "ROTATED", overlap_until: new Date(overlapUntil).toISOString() });
}

async function revokeReadToken(body, env) {
  let input;
  try {
    input = JSON.parse(new TextDecoder().decode(body));
  } catch {
    return jsonResponse({ status: "REJECTED", reason: "INVALID_JSON" }, 400);
  }
  if (!HEX_64.test(input?.token_hash || "")) {
    return jsonResponse({ status: "REJECTED", reason: "INVALID_TOKEN_HASH" }, 400);
  }
  const current = await currentReadTokenHash(env);
  if (constantTimeEqual(input.token_hash, current)) {
    // The marker prevents INITIAL_READ_TOKEN_HASH from becoming active again
    // after its matching KV value is revoked.
    await env.SUBSCRIPTIONS.put("read-token/initialized", "1");
    await env.SUBSCRIPTIONS.delete("read-token/current-hash");
  }
  const previousText = await env.SUBSCRIPTIONS.get("read-token/previous");
  if (previousText) {
    try {
      const previous = JSON.parse(previousText);
      if (constantTimeEqual(input.token_hash, previous.hash || "")) {
        await env.SUBSCRIPTIONS.delete("read-token/previous");
      }
    } catch {
      await env.SUBSCRIPTIONS.delete("read-token/previous");
    }
  }
  return jsonResponse({ status: "REVOKED" });
}

async function publicationStatus(env) {
  const value = await env.SUBSCRIPTIONS.get("publication/current");
  if (!value) {
    return jsonResponse({ status: "EMPTY" });
  }
  return new Response(value, { headers: { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "no-store" } });
}

async function authenticateNodeRequest(request, env, body) {
  const timestampText = request.headers.get("X-VPSKit-Timestamp") || "";
  const nonce = request.headers.get("X-VPSKit-Nonce") || "";
  const signature = request.headers.get("X-VPSKit-Signature") || "";
  const timestamp = Number(timestampText);
  if (!Number.isInteger(timestamp) || Math.abs(Math.floor(Date.now() / 1000) - timestamp) > MAX_CLOCK_SKEW_SECONDS) {
    return false;
  }
  if (!/^[a-f0-9]{32}$/.test(nonce) || !HEX_64.test(signature) || typeof env.NODE_PUBLISH_SECRET !== "string" || env.NODE_PUBLISH_SECRET.length < 32) {
    return false;
  }
  const digest = await sha256Hex(body);
  const canonical = [timestampText, nonce, request.method, new URL(request.url).pathname, digest].join("\n");
  const expected = await hmacHex(env.NODE_PUBLISH_SECRET, canonical);
  if (!constantTimeEqual(signature, expected)) {
    return false;
  }
  const nonceKey = `nonce/${env.NODE_ID}/${nonce}`;
  if (await env.SUBSCRIPTIONS.get(nonceKey)) {
    return false;
  }
  await env.SUBSCRIPTIONS.put(nonceKey, "1", { expirationTtl: NONCE_TTL_SECONDS });
  return true;
}

async function validReadToken(token, env) {
  if (!token || token.length < 43) {
    return false;
  }
  const digest = await sha256Hex(new TextEncoder().encode(token));
  const current = await currentReadTokenHash(env);
  if (constantTimeEqual(digest, current)) {
    return true;
  }
  const previousText = await env.SUBSCRIPTIONS.get("read-token/previous");
  if (!previousText) {
    return false;
  }
  try {
    const previous = JSON.parse(previousText);
    return Date.parse(previous.expires_at) >= Date.now() && constantTimeEqual(digest, previous.hash || "");
  } catch {
    return false;
  }
}

async function currentReadTokenHash(env) {
  const current = await env.SUBSCRIPTIONS.get("read-token/current-hash");
  if (current !== null) {
    return current;
  }
  if (await env.SUBSCRIPTIONS.get("read-token/initialized")) {
    return "";
  }
  return env.INITIAL_READ_TOKEN_HASH || "";
}

function validatePublication(payload, nodeID) {
  if (!payload || payload.schema_version !== 1 || payload.node_id !== nodeID) return "INVALID_IDENTITY";
  if (!Number.isInteger(payload.node_revision) || payload.node_revision < 1) return "INVALID_REVISION";
  if (!Number.isInteger(payload.ruleset_revision) || payload.ruleset_revision < 0) return "INVALID_RULESET_REVISION";
  if (!PUBLICATION_ID.test(payload.publication_id || "")) return "INVALID_PUBLICATION_ID";
  if (!Array.isArray(payload.artifacts) || payload.artifacts.length !== TARGETS.size) return "INVALID_ARTIFACT_COUNT";
  const targets = new Set();
  for (const artifact of payload.artifacts) {
    if (!artifact || !TARGETS.has(artifact.target) || targets.has(artifact.target)) return "INVALID_TARGET";
    targets.add(artifact.target);
    if (!HEX_64.test(artifact.sha256 || "") || typeof artifact.content_base64 !== "string") return "INVALID_ARTIFACT";
    if (typeof artifact.media_type !== "string" || artifact.media_type.length < 3 || artifact.media_type.length > 128) return "INVALID_MEDIA_TYPE";
    if (!Number.isInteger(artifact.renderer_version) || artifact.renderer_version < 1) return "INVALID_RENDERER";
  }
  return "";
}

function artifactRecord(payload, artifact, publishedAt) {
  return {
    schema_version: 1,
    node_id: payload.node_id,
    node_revision: payload.node_revision,
    ruleset_revision: payload.ruleset_revision,
    publication_id: payload.publication_id,
    published_at: publishedAt,
    target: artifact.target,
    media_type: artifact.media_type,
    sha256: artifact.sha256,
    content_base64: artifact.content_base64,
    renderer: artifact.renderer,
    renderer_version: artifact.renderer_version,
    compatibility_profile: artifact.compatibility_profile,
  };
}

function currentKey(target) {
  return `artifact/${target}/current`;
}

function revisionKey(target, publicationID) {
  return `artifact/${target}/revision/${publicationID}`;
}

async function readLimitedBody(request) {
  const length = Number(request.headers.get("Content-Length") || "0");
  if (length > MAX_BODY_BYTES) return null;
  const bytes = new Uint8Array(await request.arrayBuffer());
  return bytes.length > MAX_BODY_BYTES ? null : bytes;
}

async function sha256Hex(bytes) {
  return bytesToHex(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)));
}

async function hmacHex(secret, message) {
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  return bytesToHex(new Uint8Array(await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(message))));
}

function bytesToHex(bytes) {
  return [...bytes].map((value) => value.toString(16).padStart(2, "0")).join("");
}

function base64ToBytes(value) {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

function constantTimeEqual(left, right) {
  if (typeof left !== "string" || typeof right !== "string" || left.length !== right.length) return false;
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) difference |= left.charCodeAt(index) ^ right.charCodeAt(index);
  return difference === 0;
}

function responseForMethod(request, body, headers) {
  return new Response(request.method === "HEAD" ? null : body, { status: 200, headers });
}

function jsonResponse(value, status = 200) {
  return new Response(JSON.stringify(value) + "\n", { status, headers: { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "no-store" } });
}

function methodNotAllowed(allow) {
  return new Response(null, { status: 405, headers: { Allow: allow, "Cache-Control": "no-store" } });
}

function notFound() {
  return new Response(null, { status: 404, headers: { "Cache-Control": "no-store" } });
}
