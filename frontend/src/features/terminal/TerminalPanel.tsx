import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { useEffect, useRef, useState } from "react";
import { api } from "../../api";
import type { Project, Server } from "../../types";
import { runnerCapabilityBlockedReason, runnerCapabilityPillLabel } from "../../shared/runnerCapabilities";
import { CapabilityPill, InlineNotice } from "../../shared/ui";

export function TerminalPanel(props: { server: Server | null; project: Project; visible: boolean }) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const resizeRef = useRef<ResizeObserver | null>(null);
  const disposedRef = useRef(false);
  const closedByUserRef = useRef(false);
  const [state, setState] = useState<"idle" | "connecting" | "ready" | "closed" | "error">("idle");
  const [connectAttempt, setConnectAttempt] = useState(0);
  const canUseTerminal = Boolean(props.server?.runner_connected && props.server.runner_capabilities?.project_terminal === true);
  const blockedReason = runnerCapabilityBlockedReason(props.server, "project_terminal", "terminal sessions");

  useEffect(() => {
    disposedRef.current = false;
    return () => {
      disposedRef.current = true;
      resizeRef.current?.disconnect();
      socketRef.current?.close();
      terminalRef.current?.dispose();
      terminalRef.current = null;
      fitRef.current = null;
      socketRef.current = null;
    };
  }, []);

  useEffect(() => {
    const node = containerRef.current;
    if (!node || !canUseTerminal || terminalRef.current || closedByUserRef.current) {
      return;
    }

    const terminal = new XTerm({
      cursorBlink: true,
      convertEol: true,
      fontFamily: 'ui-monospace, SFMono-Regular, Consolas, "Liberation Mono", monospace',
      fontSize: 13,
      theme: {
        background: "#111713",
        foreground: "#edf4ee",
        cursor: "#dce9df",
        selectionBackground: "#3f5445",
      },
    });
    const fit = new FitAddon();
    terminal.loadAddon(fit);
    terminal.open(node);
    fit.fit();
    terminal.writeln("Connecting to project terminal...");

    terminalRef.current = terminal;
    fitRef.current = fit;
    setState("connecting");

    const socket = new WebSocket(api.projectTerminalWebSocketUrl(props.project.id));
    socketRef.current = socket;

    const sendResize = () => {
      fit.fit();
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "resize", payload: { cols: terminal.cols, rows: terminal.rows } }));
      }
    };

    resizeRef.current = new ResizeObserver(sendResize);
    resizeRef.current.observe(node);

    const inputDisposable = terminal.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "input", payload: { data } }));
      }
    });

    socket.onopen = () => {
      fit.fit();
      socket.send(JSON.stringify({ type: "open", payload: { cols: terminal.cols, rows: terminal.rows } }));
    };

    socket.onmessage = (event) => {
      try {
        const message = JSON.parse(String(event.data)) as {
          type: "ready" | "output" | "exit" | "error";
          data?: string;
          workdir?: string;
          exit_code?: number;
          error?: string | null;
          message?: string;
        };
        if (message.type === "ready") {
          setState("ready");
          terminal.clear();
          terminal.writeln(`Connected: ${message.workdir ?? props.project.workdir}`);
          terminal.focus();
          return;
        }
        if (message.type === "output") {
          terminal.write(message.data ?? "");
          return;
        }
        if (message.type === "exit") {
          setState("closed");
          terminal.writeln("");
          terminal.writeln(`[terminal exited${typeof message.exit_code === "number" ? `: ${message.exit_code}` : ""}]`);
          return;
        }
        if (message.type === "error") {
          setState("error");
          terminal.writeln(`\r\n${message.message ?? message.error ?? "Terminal error."}`);
        }
      } catch {
        setState("error");
      }
    };

    socket.onerror = () => {
      if (disposedRef.current || socketRef.current !== socket) {
        return;
      }
      setState("error");
      terminal.writeln("\r\nTerminal connection failed.");
    };
    socket.onclose = () => {
      if (!disposedRef.current && socketRef.current === socket) {
        setState((value) => (value === "ready" ? "closed" : value));
      }
    };

    return () => {
      inputDisposable.dispose();
      resizeRef.current?.disconnect();
      socket.close();
      terminal.dispose();
      terminalRef.current = null;
      fitRef.current = null;
      socketRef.current = null;
    };
  }, [canUseTerminal, connectAttempt, props.project.id, props.project.workdir]);

  useEffect(() => {
    if (!canUseTerminal) {
      setState("idle");
    }
  }, [canUseTerminal]);

  useEffect(() => {
    if (props.visible) {
      requestAnimationFrame(() => {
        fitRef.current?.fit();
        terminalRef.current?.focus();
      });
    }
  }, [props.visible]);

  const closeTerminal = () => {
    closedByUserRef.current = true;
    socketRef.current?.close();
    socketRef.current = null;
    resizeRef.current?.disconnect();
    resizeRef.current = null;
    terminalRef.current?.dispose();
    terminalRef.current = null;
    fitRef.current = null;
    setState("closed");
  };

  const reconnectTerminal = () => {
    closedByUserRef.current = false;
    closeTerminal();
    closedByUserRef.current = false;
    setState("idle");
    setConnectAttempt((value) => value + 1);
  };

  return (
    <section className="terminalPanel" aria-label="Project terminal">
      <div className="toolHeader">
        <div>
          <h3>Terminal</h3>
          <p className="mono">{props.project.workdir}</p>
        </div>
        <div className="toolActions">
          <CapabilityPill available={canUseTerminal} label={canUseTerminal ? terminalStateLabel(state) : runnerCapabilityPillLabel(props.server)} />
          <button
            className="ghostButton compact"
            type="button"
            onClick={() => terminalRef.current?.clear()}
            disabled={!terminalRef.current}
          >
            Clear
          </button>
          <button
            className="ghostButton compact"
            type="button"
            onClick={state === "closed" || state === "error" ? reconnectTerminal : closeTerminal}
            disabled={!canUseTerminal || (state !== "closed" && state !== "error" && !socketRef.current)}
          >
            {state === "closed" || state === "error" ? "Reconnect" : "Close"}
          </button>
        </div>
      </div>

      {!canUseTerminal ? <InlineNotice tone="danger">{blockedReason}</InlineNotice> : null}

      <div ref={containerRef} className="terminalViewport" />
    </section>
  );
}


function terminalStateLabel(state: "idle" | "connecting" | "ready" | "closed" | "error") {
  if (state === "ready") {
    return "PTY ready";
  }
  if (state === "connecting") {
    return "Connecting";
  }
  if (state === "closed") {
    return "Closed";
  }
  if (state === "error") {
    return "Terminal error";
  }
  return "Ready";
}
