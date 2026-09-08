import {
  Activity,
  ArrowRight,
  ChevronDown,
  FolderOpen,
  Import,
  Plus,
  Power,
  RefreshCw,
  Save,
  Server,
  Settings,
  Square,
  Trash2,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import "./App.css";
import {
  chooseBinary,
  deleteProfile,
  getProfiles,
  getRuntimeStatus,
  openDataDir,
  openPowerSettings,
  runAction,
  saveProfile,
} from "./bridge";
import {
  buildClientCommand,
  createEmptyProfile,
  parseClientCommand,
} from "./parser";
import type { BridgeAction, ClientProfile, RuntimeStatus } from "./types";
const empty = () => createEmptyProfile(crypto.randomUUID());
const describe = (error: unknown) =>
  error instanceof Error ? error.message : String(error);
const statusLabels: Record<string, string> = {
  running: "运行中",
  stopped: "已停止",
  connecting: "连接中",
  starting: "启动中",
  restarting: "重启中",
  unknown: "未知",
  "not installed": "未安装",
  "native bridge unavailable": "浏览器预览",
};

export default function App() {
  const [profiles, setProfiles] = useState<ClientProfile[]>([]);
  const [draft, setDraft] = useState<ClientProfile>(empty);
  const [dirty, setDirty] = useState(false);
  const [status, setStatus] = useState<RuntimeStatus | null>(null);
  const [command, setCommand] = useState("");
  const [logs, setLogs] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const [notice, setNotice] = useState<{ error: boolean; text: string } | null>(
    null,
  );
  const [advanced, setAdvanced] = useState(false);
  const [ready, setReady] = useState(false);
  const report = useCallback((text: string, error = false) => {
    setNotice({
      text:
        text.length > 300
          ? text.slice(0, 300) + "… 完整输出请查看操作记录。"
          : text,
      error,
    });
    setLogs((current) =>
      [`${new Date().toLocaleTimeString()}  ${text}`, ...current].slice(0, 100),
    );
  }, []);
  useEffect(() => {
    let active = true;
    getProfiles()
      .then((items) => {
        if (!active) return;
        setProfiles(items);
        if (items[0]) setDraft(items[0]);
      })
      .catch((error) => {
        if (active) report(describe(error), true);
      })
      .finally(() => {
        if (active) setReady(true);
      });
    return () => {
      active = false;
    };
  }, [report]);
  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        if (document.visibilityState === "visible" && !busyRef.current) {
          const value = await getRuntimeStatus();
          if (active) setStatus(value);
        }
      } catch (error) {
        if (active)
          setStatus((current) =>
            current ? { ...current, serviceStatus: "状态获取失败" } : null,
          );
      } finally {
        if (active) timer = setTimeout(poll, 5000);
      }
    };
    void poll();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, []);
  useEffect(() => {
    const warn = (event: BeforeUnloadEvent) => {
      if (dirty) {
        event.preventDefault();
        event.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  const change = <K extends keyof ClientProfile>(
    key: K,
    value: ClientProfile[K],
  ) => {
    setDraft((current) => ({ ...current, [key]: value }));
    setDirty(true);
  };
  const switchProfile = (next: ClientProfile) => {
    if (dirty && !window.confirm("当前连接有未保存的修改，是否放弃修改？"))
      return;
    setDraft(next);
    setDirty(false);
    setCommand("");
    setNotice(null);
  };
  const persist = async () => {
    const value = {
      ...draft,
      updatedAt: new Date().toISOString(),
      command: buildClientCommand(draft),
    };
    const items = await saveProfile(value);
    setProfiles(items);
    setDraft(value);
    setDirty(false);
    return value;
  };
  const execute = async (action: BridgeAction | "save") => {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setNotice(null);
    try {
      if (action === "save") {
        await persist();
        report("连接配置已保存");
        return;
      }
      const profile = action === "apply" ? await persist() : draft;
      const result = await runAction(action, profile);
      report(result.output || result.message, !result.ok);
      setStatus(result.status ?? (await getRuntimeStatus()));
    } catch (error) {
      report(describe(error), true);
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  };
  const remove = async () => {
    if (
      !profiles.some((item) => item.id === draft.id) ||
      !window.confirm("删除这个已保存的连接？此操作不会卸载或停止系统服务。")
    )
      return;
    setBusy(true);
    try {
      const items = await deleteProfile(draft.id);
      setProfiles(items);
      setDraft(items[0] ?? empty());
      setDirty(false);
      setCommand("");
      report("已删除连接配置");
    } catch (error) {
      report(describe(error), true);
    } finally {
      setBusy(false);
    }
  };
  const importCommand = () => {
    try {
      const parsed = parseClientCommand(command);
      setDraft((current) => ({ ...current, ...parsed }));
      setDirty(true);
      report("命令已识别，点击“保存并连接”即可接入");
    } catch (error) {
      report(describe(error), true);
    }
  };
  const native = !!status && status.platform !== "web";
  const valid =
    !!draft.apiUrl &&
    !!draft.rpcUrl &&
    !!(draft.joinToken || (draft.clientId && draft.secret));
  const running = ["running", "connecting", "starting", "restarting"].includes(
    status?.serviceStatus ?? "",
  );
  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <Server size={22} />
          </div>
          <div>
            <strong>frp-manager</strong>
            <span>设备连接工作空间</span>
          </div>
        </div>
        <button
          className="new-profile"
          disabled={busy || !ready}
          onClick={() => switchProfile(empty())}
        >
          <Plus size={17} />
          新建连接
        </button>
        <div className="sidebar-label">
          我的连接 <span>{profiles.length}</span>
        </div>
        <nav className="profile-list" aria-label="已保存的连接">
          {profiles.map((profile) => (
            <button
              disabled={busy}
              key={profile.id}
              className={profile.id === draft.id ? "profile active" : "profile"}
              onClick={() => switchProfile(profile)}
            >
              <Server size={17} />
              <span>
                <strong>{profile.name}</strong>
                <small>{profile.apiUrl}</small>
              </span>
            </button>
          ))}
          {!profiles.length && (
            <p className="sidebar-empty">
              还没有保存的连接。
              <br />
              从面板复制接入命令开始。
            </p>
          )}
        </nav>
        <div className="sidebar-note">
          节点身份由内核自动保存。
          <br />
          重启后会重新连接管理面板。
        </div>
      </aside>
      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">连接工作空间</p>
            <h1>
              {draft.name || "新连接"}
              {dirty && <span className="unsaved">未保存</span>}
            </h1>
          </div>
          <span className={running ? "pill connected" : "pill"}>
            <Activity size={15} />
            {statusLabels[status?.serviceStatus ?? ""] ||
              status?.serviceStatus ||
              "正在获取状态"}
          </span>
        </header>
        {!native && ready && (
          <div className="preview-note">
            当前为浏览器预览，可以编辑与保存配置；实际连接需要桌面或 Android
            应用。
          </div>
        )}
        {notice && (
          <div
            role={notice.error ? "alert" : "status"}
            className={notice.error ? "notice error" : "notice"}
          >
            {notice.error ? "操作未完成：" : ""}
            {notice.text}
          </div>
        )}
        <div className="connect-grid">
          <section className="panel import-panel">
            <div className="section-heading">
              <span className="step-number">01</span>
              <div>
                <h2>粘贴接入命令</h2>
                <p>在管理面板中打开“客户端 → 自动接入”，复制命令到这里。</p>
              </div>
            </div>
            <label>
              <span className="sr-only">客户端接入命令</span>
              <textarea
                disabled={busy}
                value={command}
                onChange={(event) => setCommand(event.target.value)}
                placeholder="frp-manager client --api-url https://… --join-token …"
                spellCheck={false}
              />
            </label>
            <button disabled={!command.trim() || busy} onClick={importCommand}>
              <Import size={16} />
              识别命令
              <ArrowRight size={16} />
            </button>
          </section>
          <section className="panel connection-panel">
            <div className="section-heading">
              <span className="step-number">02</span>
              <div>
                <h2>确认并连接</h2>
                <p>连接成功后，在面板中配置隧道和访问端口。</p>
              </div>
            </div>
            <fieldset disabled={busy || !ready}>
              <label>
                <span>连接名称</span>
                <input
                  value={draft.name}
                  onChange={(event) => change("name", event.target.value)}
                  placeholder="例如：家里的电脑"
                />
              </label>
              <label>
                <span>管理面板地址</span>
                <input
                  value={draft.apiUrl}
                  placeholder="https://panel.example.com"
                  onChange={(event) => {
                    const value = event.target.value;
                    setDraft((current) => ({
                      ...current,
                      apiUrl: value,
                      rpcUrl:
                        !current.rpcUrl ||
                        current.rpcUrl === current.apiUrl.replace(/^http/, "ws")
                          ? value.replace(/^http/, "ws")
                          : current.rpcUrl,
                    }));
                    setDirty(true);
                  }}
                />
              </label>
              <label>
                <span>
                  接入令牌 <small>已有 ID 和密钥可在高级设置中填写</small>
                </span>
                <input
                  type="password"
                  autoComplete="off"
                  value={draft.joinToken ?? ""}
                  onChange={(event) => change("joinToken", event.target.value)}
                />
              </label>
              <button
                className="advanced-toggle"
                aria-expanded={advanced}
                onClick={() => setAdvanced((value) => !value)}
              >
                <Settings size={16} />
                高级设置
                <ChevronDown size={15} />
              </button>
              {advanced && (
                <div className="advanced-fields">
                  <label>
                    <span>RPC 地址</span>
                    <input
                      value={draft.rpcUrl}
                      onChange={(event) => change("rpcUrl", event.target.value)}
                    />
                  </label>
                  <div className="two-col">
                    <label>
                      <span>客户端 ID</span>
                      <input
                        value={draft.clientId}
                        onChange={(event) =>
                          change("clientId", event.target.value)
                        }
                      />
                    </label>
                    <label>
                      <span>连接密钥</span>
                      <input
                        type="password"
                        value={draft.secret}
                        onChange={(event) =>
                          change("secret", event.target.value)
                        }
                      />
                    </label>
                  </div>
                  <label className="security-toggle">
                    <input
                      type="checkbox"
                      checked={draft.allowInsecure ?? false}
                      onChange={(event) =>
                        change("allowInsecure", event.target.checked)
                      }
                    />
                    <span>允许可信内网的 HTTP / WS 连接</span>
                  </label>
                  {status?.platform !== "android" && (
                    <label>
                      <span>自定义内核路径（留空自动选择）</span>
                      <div className="path-row">
                        <input
                          value={draft.binaryPath ?? ""}
                          onChange={(event) =>
                            change("binaryPath", event.target.value)
                          }
                        />
                        <button
                          aria-label="选择内核文件"
                          onClick={async () => {
                            try {
                              const file = await chooseBinary();
                              if (file) change("binaryPath", file);
                            } catch (error) {
                              report(describe(error), true);
                            }
                          }}
                        >
                          <FolderOpen size={16} />
                        </button>
                      </div>
                    </label>
                  )}
                </div>
              )}
              <div className="connect-actions">
                <button
                  className="primary"
                  disabled={!valid || !native}
                  onClick={() => execute("apply")}
                >
                  <Power size={17} />
                  {busy ? "正在处理…" : "保存并连接"}
                </button>
                <button onClick={() => execute("save")}>
                  <Save size={16} />
                  仅保存
                </button>
              </div>
            </fieldset>
          </section>
        </div>
        <section className="panel service-panel">
          <div>
            <h2>本机服务</h2>
            <p>
              所有连接配置共用一个本机服务。停止或重启会作用于当前正在运行的连接。
            </p>
          </div>
          <div className="button-row">
            <button disabled={busy || !native} onClick={() => execute("start")}>
              <Power size={16} />
              启动
            </button>
            <button disabled={busy || !native} onClick={() => execute("stop")}>
              <Square size={16} />
              停止
            </button>
            <button
              disabled={busy || !native}
              onClick={() => execute("restart")}
            >
              <RefreshCw size={16} />
              重启
            </button>
          </div>
          <details>
            <summary>维护与诊断</summary>
            <div className="maintenance">
              <button
                disabled={busy || !native}
                onClick={() => execute("install")}
              >
                仅安装服务
              </button>
              <button
                disabled={busy || !native}
                onClick={() => {
                  if (
                    window.confirm("卸载本机服务？之后需要重新安装才能启动。")
                  )
                    void execute("uninstall");
                }}
              >
                卸载服务
              </button>
              <button disabled={busy} onClick={remove}>
                <Trash2 size={15} />
                删除配置
              </button>
              <button
                onClick={() => {
                  void (
                    status?.platform === "android"
                      ? openPowerSettings()
                      : openDataDir()
                  ).catch((error) => report(describe(error), true));
                }}
              >
                <FolderOpen size={15} />
                {status?.platform === "android" ? "电池设置" : "打开数据目录"}
              </button>
            </div>
            <p className="binary-path">
              内核：{status?.binaryPath || "自动选择"}
            </p>
          </details>
        </section>
        <details className="panel log-panel">
          <summary>
            操作记录 <span>{logs.length}</span>
          </summary>
          <pre>{logs.join("\n") || "暂无操作记录。"}</pre>
        </details>
      </section>
    </main>
  );
}
