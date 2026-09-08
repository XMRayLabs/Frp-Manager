export type BridgeAction =
  "apply" | "install" | "start" | "stop" | "restart" | "run" | "uninstall";

export type ClientProfile = {
  id: string;
  name: string;
  clientId: string;
  secret: string;
  joinToken?: string;
  apiUrl: string;
  rpcUrl: string;
  command: string;
  binaryPath?: string;
  allowInsecure?: boolean;
  createdAt: string;
  updatedAt: string;
};

export type RuntimeStatus = {
  platform: string;
  serviceStatus: string;
  binaryPath?: string;
  dataDir?: string;
  batteryOptimizationExempt?: boolean;
};

export type ActionResult = {
  ok: boolean;
  message: string;
  output?: string;
  status?: RuntimeStatus;
};

export type FrpManagerBridge = {
  getProfiles: () => Promise<ClientProfile[]>;
  saveProfile: (profile: ClientProfile) => Promise<ClientProfile[]>;
  deleteProfile: (id: string) => Promise<ClientProfile[]>;
  getRuntimeStatus: () => Promise<RuntimeStatus>;
  chooseBinary: () => Promise<string | null>;
  runAction: (
    action: BridgeAction,
    profile: ClientProfile,
  ) => Promise<ActionResult>;
  openDataDir: () => Promise<void>;
};
