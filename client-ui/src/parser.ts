import type { ClientProfile } from "./types";

export function createEmptyProfile(id: string): ClientProfile {
  const now = new Date().toISOString();
  return {
    id,
    name: "新连接",
    clientId: "",
    secret: "",
    joinToken: "",
    apiUrl: "",
    rpcUrl: "",
    command: "",
    createdAt: now,
    updatedAt: now,
  };
}

export function buildClientCommand(profile: ClientProfile): string {
  const credentials = profile.joinToken
    ? ["--join-token", profile.joinToken]
    : ["--id", profile.clientId, "--secret", profile.secret];
  return [
    "frp-manager",
    "client",
    ...credentials,
    "--api-url",
    profile.apiUrl,
    "--rpc-url",
    profile.rpcUrl,
  ]
    .map(quote)
    .join(" ");
}

export function parseClientCommand(command: string): Partial<ClientProfile> {
  const tokens = splitArgs(command);
  if (!tokens.includes("client") || tokens.includes("server"))
    throw new Error("请粘贴面板生成的客户端接入命令。");
  const secret = readOption(tokens, "--secret") || readOption(tokens, "-s");
  const clientId = readOption(tokens, "--id") || readOption(tokens, "-i");
  const joinToken =
    readOption(tokens, "--join-token") || readOption(tokens, "-j");
  const apiUrl = readOption(tokens, "--api-url");
  let rpcUrl = readOption(tokens, "--rpc-url");
  if (!joinToken && (!secret || !clientId))
    throw new Error("命令缺少接入令牌，或客户端 ID 与密钥。");
  if (!apiUrl) throw new Error("命令缺少面板地址 --api-url。");
  const endpoint = new URL(apiUrl);
  if (
    !["http:", "https:"].includes(endpoint.protocol) ||
    endpoint.username ||
    endpoint.password
  )
    throw new Error("面板地址应为 HTTP 或 HTTPS URL，且不能包含用户名密码。");
  if (!rpcUrl) rpcUrl = apiUrl.replace(/^http/, "ws");
  if (!["grpc:", "ws:", "wss:"].includes(new URL(rpcUrl).protocol))
    throw new Error("RPC 地址应使用 grpc、ws 或 wss。");
  return {
    clientId,
    secret,
    joinToken,
    apiUrl,
    rpcUrl,
    name: clientId || endpoint.hostname,
    command: command.trim(),
  };
}

function readOption(tokens: string[], name: string): string {
  for (let i = 0; i < tokens.length; i++) {
    if (tokens[i] === name)
      return tokens[i + 1]?.startsWith("-") ? "" : (tokens[i + 1] ?? "");
    if (tokens[i].startsWith(name + "="))
      return tokens[i].slice(name.length + 1);
  }
  return "";
}

function splitArgs(input: string): string[] {
  const tokens: string[] = [];
  let current = "",
    delimiter = "",
    started = false;
  for (let i = 0; i < input.length; i++) {
    const char = input[i];
    if (
      char === "\\" &&
      delimiter === '"' &&
      ['"', "\\"].includes(input[i + 1])
    ) {
      current += input[++i];
      started = true;
      continue;
    }
    if ((char === '"' || char === "'") && !delimiter) {
      delimiter = char;
      started = true;
      continue;
    }
    if (char === delimiter) {
      delimiter = "";
      continue;
    }
    if (/\s/.test(char) && !delimiter) {
      if (started) tokens.push(current);
      current = "";
      started = false;
      continue;
    }
    current += char;
    started = true;
  }
  if (delimiter) throw new Error("命令中的引号没有闭合，请重新复制完整命令。");
  if (started) tokens.push(current);
  return tokens;
}
function quote(value: string): string {
  if (/^[A-Za-z0-9_./:@=-]+$/.test(value)) return value;
  return '"' + value.replaceAll("\\", "\\\\").replaceAll('"', '\\"') + '"';
}
