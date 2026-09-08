export type Admin = {
  id: string;
  username: string;
  createdAt: number;
};

export type Settings = {
  centerId: string;
  centerName: string;
};

export type StartupCommandRule = {
  expect: string;
  send: string;
};

export type Host = {
  id: string;
  label: string;
  hostname: string;
  port: number;
  username: string;
  group: string;
  tags: string[];
  os: string;
  protocol: string;
  deviceType: string;
  notes: string;
  password: string;
  privateKey: string;
  passphrase: string;
  startupCommand: string;
  startupCommandRunMode: string;
  startupCommandRules: StartupCommandRule[];
  visibility: string;
  visibleKeyIds: string[];
  createdAt: number;
  updatedAt: number;
};

export type HostDraft = Omit<Host, "id" | "createdAt" | "updatedAt" | "tags"> & {
  id: string | null;
  tags: string[] | string;
};

export type APIKey = {
  id: string;
  name: string;
  keyPrefix: string;
  plaintext: string;
  permission: string;
  createdAt: number;
  lastUsedAt: number | null;
  revokedAt: number | null;
};

export type GroupNode = {
  name: string;
  path: string;
  children: Record<string, GroupNode>;
  hosts: Host[];
  totalHostCount: number;
};

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}
