import test from "node:test";
import assert from "node:assert/strict";
import {
  parseClientCommand,
  buildClientCommand,
  createEmptyProfile,
} from "../src/parser.ts";
test("imports enrollment command and derives WebSocket endpoint", () => {
  const parsed = parseClientCommand(
    "frp-manager client --api-url https://panel.example --join-token token",
  );
  assert.equal(parsed.joinToken, "token");
  assert.equal(parsed.rpcUrl, "wss://panel.example");
  assert.equal(parsed.clientId, "");
});
test("preserves legacy credentials and quoted values through round trip", () => {
  const profile = {
    ...createEmptyProfile("test"),
    clientId: "node-1",
    secret: 'a "quoted" \\ secret',
    apiUrl: "https://panel.example",
    rpcUrl: "wss://panel.example",
  };
  const parsed = parseClientCommand(buildClientCommand(profile));
  assert.equal(parsed.secret, profile.secret);
  assert.equal(parsed.clientId, profile.clientId);
});
test("rejects incomplete quotes, server commands and missing credentials", () => {
  assert.throws(() => parseClientCommand('frp-manager client -j "abc'));
  assert.throws(() => parseClientCommand("frp-manager server -j abc"));
  assert.throws(() =>
    parseClientCommand("frp-manager client --api-url https://panel.example"),
  );
});
