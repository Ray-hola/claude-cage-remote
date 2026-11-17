import React, { useEffect, useRef, useState } from "react";
import { AppWindow, Download, PanelLeftOpen, Power, RefreshCw } from "lucide-react";
import { api } from "./api";
import { SafetyMessage } from "./safety-message";
import { RemoteView } from "./views";
import { useWindowVisibility } from "./window-visibility";
import "./claude-desktop.css";

// A second RFB endpoint beside Claude Desktop: a full Xfce session in a Linux
// guest, installed and started on request like Claude Desktop itself.
export function GuestDesktop({
  status,
  visible,
  remoteRef,
  onStatus,
  onFallback,
  onConfirm,
  onDetails,
  sidebarCollapsed,
  onExpand,
}) {
  const windowVisible = useWindowVisibility();
  const screenVisible = visible && windowVisible;
  const [desktop, setDesktop] = useState(null);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [jobID, setJobID] = useState("");
  const opened = useRef(false);
  const ownJob = status.job?.kind?.startsWith("guest-desktop-");
  const busy = submitting || Boolean(status.job?.running) || Boolean(jobID);
  const refresh = async (valid = () => true) => {
    try {
      const value = await api("/guest-desktop");
      if (!valid()) return null;
      setDesktop(value);
      return value;
    } catch (e) {
      if (valid()) setError(e.message);
      return null;
    }
  };
  const run = async (action) => {
    setSubmitting(true);
    setError("");
    try {
      const job = await api(`/guest-desktop/${action}`, {});
      setJobID(job.id);
    } catch (e) {
      setError(e.message);
    } finally {
      setSubmitting(false);
    }
  };
  useEffect(() => {
    if (!screenVisible || !status.vm_running || status.job?.running) return;
    let disposed = false;
    let timer;
    const poll = async () => {
      const value = await refresh(() => !disposed);
      if (disposed) return;
      if (value && !opened.current) {
        opened.current = true;
        if (value.installed && !value.running && !value.failed && !jobID) await run("start");
      }
      if (!disposed) timer = setTimeout(poll, 5000);
    };
    poll();
    return () => {
      disposed = true;
      clearTimeout(timer);
    };
  }, [screenVisible, status.vm_running, status.job?.running]);
  useEffect(() => {
    if (!jobID || status.job?.id !== jobID || status.job.running) return;
    setJobID("");
    if (status.job.error) {
      setError(status.job.error);
      return;
    }
    if (status.job.kind === "guest-desktop-install") run("start");
    else refresh();
  }, [status.job]);
  return (
    <section className="claude-desktop-view" aria-label="客机桌面">
      <header className="claude-desktop-toolbar">
        {sidebarCollapsed && (
          <button className="icon-button" aria-label="展开侧栏" onClick={onExpand}>
            <PanelLeftOpen size={17} />
          </button>
        )}
        <AppWindow size={18} />
        <strong>客机桌面</strong>
        <span className="desktop-version">Xfce</span>
        <span className="desktop-toolbar-spacer" />
        {desktop?.running && (
          <button
            className="quiet-button"
            disabled={busy}
            onClick={async () => {
              if (await onConfirm({
                title: "关闭客机桌面？",
                body: "桌面里打开的程序会一起关闭，请先保存工作。浏览器、终端和 Claude Desktop 继续运行。",
                confirm: "关闭桌面",
              })) run("stop");
            }}
          >
            <Power size={15} />
            关闭桌面
          </button>
        )}
        <button className="icon-button" aria-label="刷新客机桌面状态" disabled={busy} onClick={() => refresh()}>
          <RefreshCw size={15} />
        </button>
      </header>
      {(error || desktop?.failed) && (
        <div className="connection-banner" role="alert">
          <SafetyMessage message={error || (desktop?.failure_reason === "oom-kill"
            ? "环境内存不足，客机桌面已被系统关闭。请在设置中检查计算资源后重新打开。"
            : "客机桌面未能启动，可重试打开或修复安装。")} />
          <button onClick={onDetails}>操作详情</button>
        </div>
      )}
      {busy && ownJob && (
        <div className="desktop-progress" role="status">
          <RefreshCw size={16} className="spin" />
          {status.job.message}
          <button className="text-button" onClick={onDetails}>查看进度</button>
        </div>
      )}
      <div className="desktop-screen">
        {desktop?.running && status.vm_running ? (
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
            <h2>{desktop?.installed ? "客机桌面" : "在 Cage 中使用完整桌面"}</h2>
            <p>运行没有独立窗格的图形程序，例如文件管理器或你正在开发的桌面应用。</p>
            {!desktop?.installed && <p>首次使用时从系统软件源安装精简的 Xfce，不增加 Cage 默认安装包体积，也不随环境启动。</p>}
            <div className="desktop-actions">
              {desktop && (
                <button className="primary" disabled={busy || !status.vm_running} onClick={() => run(desktop.installed ? "start" : "install")}>
                  {desktop.installed ? <AppWindow size={16} /> : <Download size={16} />}
                  {desktop.installed ? "打开客机桌面" : "下载并安装"}
                </button>
              )}
              {desktop?.installed && (
                <button className="quiet-button" disabled={busy} onClick={() => run("install")}>修复安装</button>
              )}
              {!desktop && (
                <button className="quiet-button" disabled={busy} onClick={() => refresh()}>重新读取状态</button>
              )}
            </div>
            <p className="desktop-note">
              桌面里打开的网页链接会交给 Cage 浏览器。请不要在客机桌面另装浏览器登录 Claude：Cage 只为自己的浏览器管理时区、语言等环境。
            </p>
          </div>
        )}
      </div>
    </section>
  );
}
