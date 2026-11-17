import React, { useEffect, useState } from "react";
import { AppWindow, PanelLeftOpen, RefreshCw } from "lucide-react";
import { api } from "./api";
import { RemoteView } from "./views";
import { useWindowVisibility } from "./window-visibility";
import "./claude-desktop.css";

// A second RFB endpoint beside Claude Desktop: /guest-desktop/{vnc-credentials,vnc}.
// The view reuses the desktop transport; only the guest-side session differs.
// Until that session exists the pane explains what to start instead of retrying
// a connection that cannot succeed.
export function GuestDesktop({
  visible,
  remoteRef,
  status,
  onStatus,
  onFallback,
  sidebarCollapsed,
  onExpand,
}) {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState("probing");
  const [error, setError] = useState("");
  const windowVisible = useWindowVisibility();
  const screenVisible = visible && windowVisible;
  const vmRunning = Boolean(status?.vm_running);

  useEffect(() => {
    if (!screenVisible || !vmRunning) return;
    let disposed = false;
    setState("probing");
    setError("");
    api("/guest-desktop/vnc-credentials")
      .then(() => {
        if (!disposed) setState("ready");
      })
      .catch((e) => {
        if (disposed) return;
        setError(e.message || "");
        setState("unavailable");
      });
    return () => {
      disposed = true;
    };
  }, [screenVisible, vmRunning, attempt]);

  return (
    <section className="claude-desktop-view guest-desktop-view" aria-label="客机桌面">
      <header className="claude-desktop-toolbar">
        {sidebarCollapsed && (
          <button className="icon-button" aria-label="展开侧栏" title="展开侧栏" onClick={onExpand}>
            <PanelLeftOpen size={17} />
          </button>
        )}
        <AppWindow size={18} />
        <strong>客机桌面</strong>
        <span className="desktop-toolbar-spacer" />
        <button
          className="icon-button"
          aria-label="重新连接客机桌面"
          title="重新连接"
          onClick={() => setAttempt((value) => value + 1)}
        >
          <RefreshCw size={15} />
        </button>
      </header>
      <div className="desktop-screen">
        {state === "ready" && vmRunning ? (
          <RemoteView
            ref={remoteRef}
            desktop
            desktopBase="/guest-desktop"
            visible={visible}
            keyboard={status.keyboard}
            guestPlatform={status.guest_system}
            onStatus={onStatus}
            onFallback={onFallback}
          />
        ) : (
          <div className="desktop-empty">
            <AppWindow size={36} />
            <h2>{vmRunning ? "客机桌面尚未启动" : "环境尚未启动"}</h2>
            <p>
              {vmRunning
                ? "客机图形会话未运行。启动后这块画面会出现在这里，输入与剪贴板沿用上方工具栏。"
                : "启动 Cage 环境后即可打开客机桌面。"}
            </p>
            {error && <p className="desktop-note">{error}</p>}
            <div className="desktop-actions">
              <button className="quiet-button" disabled={!vmRunning} onClick={() => setAttempt((value) => value + 1)}>
                <RefreshCw size={15} />
                重新读取
              </button>
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
