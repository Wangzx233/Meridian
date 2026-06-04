import {
  AlertTriangle,
  Archive,
  Bell,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  Circle,
  ClipboardList,
  Copy,
  Download,
  Edit3,
  FileText,
  FolderKanban,
  FolderOpen,
  History,
  Loader2,
  LogOut,
  Mail,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  Play,
  Plus,
  RefreshCw,
  Save,
  Search,
  Server as ServerIcon,
  Settings as SettingsIcon,
  Square,
  Terminal,
  TerminalSquare,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import { Fragment, useEffect, useRef, useState } from "react";
import type { Run, RunEvent } from "../../types";
import { useRunEventStream } from "../../hooks";
import { eventSummary, formatDateTime, runDuration, shortId } from "../../utils";
import { useI18n } from "../../shared/i18n";
import type { LoadState } from "../../shared/loadState";
import { MarkdownContent } from "../../shared/MarkdownContent";
import { EmptyState, Fact, InlineNotice, LoadBoundary, PanelHeader, StatusBadge } from "../../shared/ui";

export function RunOutputWorkspace(props: {
  run: Run | null;
  active: boolean;
  runs: Run[];
  runsState: LoadState;
  selectedRunId: string | null;
  onSelectRun: (runId: string) => void;
  activeRun: Run | null;
  onCancelRun: (runId: string) => void;
  cancelingRun: boolean;
  onProjectFileLinkClick?: (href: string) => boolean;
}) {
  const { events, state } = useRunEventStream(props.run?.id ?? null, Boolean(props.run));
  const outputRef = useRef<HTMLDivElement | null>(null);
  const [renderMarkdown, setRenderMarkdown] = useState(true);
  const visibleEvents = visibleOutputEvents(events);

  useEffect(() => {
    const node = outputRef.current;
    if (!node) {
      return;
    }
    node.scrollTop = node.scrollHeight;
  }, [visibleEvents.length, props.run?.id]);

  if (!props.run) {
    return (
      <div className="workspaceEmpty">
        <EmptyState icon={<TerminalSquare size={24} />} title="No turn selected" body="Send or select a turn." />
      </div>
    );
  }

  return (
    <section className={`outputWorkspace ${props.run.error_message ? "hasRunError" : ""}`} aria-label="Codex output">
      <div className="outputToolbar">
        <div className="outputTitleBlock">
          <div className="rowTop">
            <h3>Codex output</h3>
            <StatusBadge status={props.run.status} />
          </div>
          <p>{streamStateLabel(state, props.active)} · {visibleEvents.length} events · {runDuration(props.run)}</p>
        </div>
        <div className="mobileOutputStatus" aria-label="Run status">
          <StatusBadge status={props.run.status} />
          <span>{streamStateLabel(state, props.active)}</span>
        </div>
        <div className="runPicker">
          <label className="markdownSwitch" htmlFor="render-markdown">
            <input
              id="render-markdown"
              type="checkbox"
              checked={renderMarkdown}
              onChange={(event) => setRenderMarkdown(event.target.checked)}
            />
            Markdown
          </label>
          <label className="srOnly" htmlFor="selected-run">
            Selected turn
          </label>
          <select
            id="selected-run"
            value={props.selectedRunId ?? props.run.id}
            onChange={(event) => props.onSelectRun(event.target.value)}
            disabled={props.runsState.isLoading || props.runs.length === 0}
          >
            {props.runs.map((run, index) => (
              <option key={run.id} value={run.id}>
                {turnOptionLabel(index, run.status)}
              </option>
            ))}
          </select>
          {props.activeRun ? (
            <button
              className="cancelButton inline"
              type="button"
              onClick={() => props.onCancelRun(props.activeRun!.id)}
              disabled={props.cancelingRun}
            >
              {props.cancelingRun ? <Loader2 className="spin" size={16} /> : <Square size={16} />}
              Cancel
            </button>
          ) : null}
        </div>
      </div>

      {props.run.error_message ? <InlineNotice tone="danger">{props.run.error_message}</InlineNotice> : null}

      <div ref={outputRef} className="conversationViewport" role="log" aria-live="polite" aria-label="Codex run output">
        <article className="messageBlock userMessage">
          <div className="messageRole">User</div>
          <p>{props.run.user_message}</p>
        </article>

        {visibleEvents.length === 0 ? (
          <div className="outputEmpty">
            {state === "error" ? "Unable to open the live stream. Stored events can still load on refresh." : "Waiting for Codex output."}
          </div>
        ) : (
          visibleEvents.map((event) => (
            <TranscriptEvent
              key={`${event.run_id}-${event.seq}`}
              event={event}
              renderMarkdown={renderMarkdown}
              onProjectFileLinkClick={props.onProjectFileLinkClick}
            />
          ))
        )}

        {props.run.final_message ? (
          <article className="messageBlock assistantMessage final">
            <div className="messageRole">Final</div>
            <MessageText
              text={props.run.final_message}
              markdown={renderMarkdown}
              onProjectFileLinkClick={props.onProjectFileLinkClick}
            />
          </article>
        ) : null}
      </div>
    </section>
  );
}


function TranscriptEvent(props: {
  event: RunEvent;
  renderMarkdown: boolean;
  onProjectFileLinkClick?: (href: string) => boolean;
}) {
  const view = transcriptView(props.event);
  if (view.hidden) {
    return null;
  }

  return (
    <article className={`messageBlock ${view.kind}`}>
      <div className="messageRole">{view.role}</div>
      <MessageText
        text={view.text}
        markdown={props.renderMarkdown && view.markdown}
        onProjectFileLinkClick={props.onProjectFileLinkClick}
      />
      {props.event.event_type === "codex.event" && props.event.payload.raw !== undefined ? (
        <details className="rawDetails">
          <summary>Raw event</summary>
          <pre>{JSON.stringify(props.event.payload.raw, null, 2)}</pre>
        </details>
      ) : null}
    </article>
  );
}

function turnOptionLabel(index: number, status: Run["status"]) {
  return `T${index + 1} · ${status}`;
}


function MessageText(props: { text: string; markdown: boolean; onProjectFileLinkClick?: (href: string) => boolean }) {
  return props.markdown ? (
    <MarkdownContent onLinkClick={props.onProjectFileLinkClick}>{props.text}</MarkdownContent>
  ) : (
    <pre>{props.text}</pre>
  );
}


function visibleOutputEvents(events: RunEvent[]) {
  return events.filter((event) => !(event.event_type === "process.output" && event.stream === "stderr"));
}


function shouldRenderEventMarkdown(event: RunEvent) {
  if (event.event_type === "process.output") {
    return event.stream === "stdout";
  }
  if (event.event_type === "codex.event") {
    return Boolean(codexDisplayText(event));
  }
  if (event.event_type === "run.final") {
    return Boolean(event.payload.final_message || event.payload.error_message);
  }
  return false;
}


function transcriptView(event: RunEvent) {
  if (event.event_type === "run.state") {
    return {
      role: "State",
      kind: "systemMessage",
      text: `Run state changed to ${event.payload.status ?? "unknown"}`,
      markdown: false,
      hidden: false,
    };
  }
  if (event.event_type === "run.final") {
    const text = String(event.payload.final_message ?? event.payload.error_message ?? "");
    return {
      role: "Final",
      kind: event.payload.status === "failed" ? "errorMessage" : "systemMessage",
      text: text || `Run ${event.payload.status ?? "finished"}`,
      markdown: Boolean(text),
      hidden: false,
    };
  }
  if (event.event_type === "runner.error") {
    return {
      role: "Runner",
      kind: "errorMessage",
      text: event.payload.message ? String(event.payload.message) : "Runner error",
      markdown: false,
      hidden: false,
    };
  }
  if (event.event_type === "process.output") {
    if (event.stream === "stderr") {
      return {
        role: "stderr",
        kind: "errorMessage terminalMessage",
        text: "",
        markdown: false,
        hidden: true,
      };
    }
    return {
      role: event.stream,
      kind: event.stream === "stdout" ? "assistantMessage" : "terminalMessage",
      text: eventSummary(event),
      markdown: event.stream === "stdout",
      hidden: false,
    };
  }
  const text = codexDisplayText(event) || eventSummary(event);
  return {
    role: "Codex",
    kind: "assistantMessage",
    text,
    markdown: true,
    hidden: !codexDisplayText(event),
  };
}


function codexDisplayText(event: RunEvent) {
  if (typeof event.payload.text === "string" && event.payload.text.trim()) {
    return event.payload.text.trim();
  }
  const raw = event.payload.raw;
  if (!raw || typeof raw !== "object") {
    return "";
  }
  const text = nestedText(raw, [
    ["message", "content"],
    ["delta", "text"],
    ["item", "text"],
    ["output", "text"],
  ]);
  return text.trim();
}


function nestedText(value: unknown, paths: string[][]) {
  for (const path of paths) {
    let current: unknown = value;
    for (const key of path) {
      if (!current || typeof current !== "object" || !(key in current)) {
        current = undefined;
        break;
      }
      current = (current as Record<string, unknown>)[key];
    }
    if (typeof current === "string" && current.trim()) {
      return current;
    }
  }
  return "";
}


export function PromptPanel(props: { run: Run | null; draftPrompt?: string; draftActive?: boolean }) {
  const { t } = useI18n();

  if (props.draftActive) {
    return (
      <section className="promptPanel" aria-label={t("prompt.pendingAria")}>
        <div className="detailHeader">
          <div>
            <h3>{t("prompt.pendingTitle")}</h3>
            <p>{t("prompt.pendingBody")}</p>
          </div>
        </div>
        <pre className="promptPreview">{props.draftPrompt || t("prompt.pendingEmpty")}</pre>
      </section>
    );
  }

  if (!props.run) {
    return (
      <section className="promptPanel" aria-label="Generated prompt">
        <EmptyState icon={<FileText size={22} />} title="No prompt yet" body="Select a turn." />
      </section>
    );
  }

  return (
    <section className="promptPanel" aria-label="Generated prompt">
      <div className="detailHeader">
        <div>
          <h3>Generated prompt</h3>
          <p className="mono">{props.run.id}</p>
        </div>
        <StatusBadge status={props.run.status} />
      </div>
      <div className="detailFacts">
        <Fact label="Mode" value={props.run.mode} />
        <Fact label="Model" value={props.run.codex_model || "Config default"} />
        <Fact label="Reasoning" value={props.run.codex_reasoning_effort || "Config default"} />
        <Fact label="Service" value={props.run.codex_service_tier || "Config default"} />
        <Fact label="Raw" value={props.run.raw_command ? "Yes" : "No"} />
        <Fact label="Runner" value={props.run.assigned_runner_id ?? "Not assigned"} mono />
        <Fact label="Exit" value={props.run.exit_code === null ? "Not recorded" : String(props.run.exit_code)} />
        <Fact label="Duration" value={runDuration(props.run)} />
      </div>
      <pre className="promptPreview">{props.run.generated_prompt || "Prompt not stored yet."}</pre>
    </section>
  );
}


export function RunTimeline(props: {
  runs: Run[];
  state: LoadState;
  selectedRunId: string | null;
  onSelectRun: (runId: string) => void;
}) {
  return (
    <section className="runTimeline" aria-label="Previous turns">
      <PanelHeader icon={<History size={16} />} title="Turns" detail={`${props.runs.length} runs`} />
      <LoadBoundary
        state={props.state}
        empty={props.runs.length === 0}
        emptyTitle="No runs yet"
        emptyBody="Send the first instruction."
      >
        <div className="timelineList">
          {props.runs.map((run, index) => (
            <button
              key={run.id}
              type="button"
              className={`timelineItem ${props.selectedRunId === run.id ? "isSelected" : ""}`}
              onClick={() => props.onSelectRun(run.id)}
            >
              <span className="timelineRail" aria-hidden="true">
                <Circle size={10} fill="currentColor" />
                {index < props.runs.length - 1 ? <span /> : null}
              </span>
              <span className="timelineContent">
                <span className="rowTop">
                  <span className="runMessage">{run.user_message || "Run request"}</span>
                  <StatusBadge status={run.status} />
                </span>
                <span className="rowMeta">
                  <span>{run.mode}</span>
                  <span>{formatDateTime(run.created_at)}</span>
                  <span>{runDuration(run)}</span>
                </span>
                {run.final_message ? <span className="finalPreview">{run.final_message}</span> : null}
              </span>
            </button>
          ))}
        </div>
      </LoadBoundary>
    </section>
  );
}


export function RunHistory(props: {
  runs: Run[];
  selectedRunId: string | null;
  onSelectRun: (runId: string) => void;
  state: LoadState;
  activeRun: Run | null;
  onCancelRun: (runId: string) => void;
  cancelingRun: boolean;
}) {
  return (
    <section className="historyPanel" aria-label="Run history">
      <LoadBoundary
        state={props.state}
        empty={props.runs.length === 0}
        emptyTitle="No run history"
        emptyBody="Turns appear here."
      >
        <div className="historyList">
          {props.runs.map((run) => (
            <button
              key={run.id}
              type="button"
              className={`historyRow ${props.selectedRunId === run.id ? "isSelected" : ""}`}
              onClick={() => props.onSelectRun(run.id)}
            >
              <span className="rowTop">
                <span className="mono">{shortId(run.id, 14)}</span>
                <StatusBadge status={run.status} />
              </span>
              <span className="taskDescription">{run.user_message}</span>
              <span className="rowMeta">
                <span>{run.mode}</span>
                <span>{formatDateTime(run.created_at)}</span>
              </span>
            </button>
          ))}
        </div>
      </LoadBoundary>

      {props.activeRun ? (
        <button
          className="cancelButton"
          type="button"
          onClick={() => props.onCancelRun(props.activeRun!.id)}
          disabled={props.cancelingRun}
        >
          {props.cancelingRun ? <Loader2 className="spin" size={16} /> : <Square size={16} />}
          Cancel current run
        </button>
      ) : (
        <InlineNotice tone="info">No queued or running turn.</InlineNotice>
      )}
    </section>
  );
}


export function RunDetail(props: { run: Run | null; active: boolean }) {
  const { events, state } = useRunEventStream(props.run?.id ?? null, Boolean(props.run));
  const visibleEvents = visibleOutputEvents(events);

  if (!props.run) {
    return (
      <section className="runDetailPanel" aria-label="Run detail">
        <EmptyState icon={<FileText size={22} />} title="No run selected" body="Select a run." />
      </section>
    );
  }

  return (
    <section className="runDetailPanel" aria-label="Run detail">
      <div className="detailHeader">
        <div>
          <h3>Run detail</h3>
          <p className="mono">{props.run.id}</p>
        </div>
        <StatusBadge status={props.run.status} />
      </div>

      <div className="detailFacts">
        <Fact label="Mode" value={props.run.mode} />
        <Fact label="Service" value={props.run.codex_service_tier || "Config default"} />
        <Fact label="Raw" value={props.run.raw_command ? "Yes" : "No"} />
        <Fact label="Runner" value={props.run.assigned_runner_id ?? "Not assigned"} mono />
        <Fact label="Exit" value={props.run.exit_code === null ? "Not recorded" : String(props.run.exit_code)} />
        <Fact label="Duration" value={runDuration(props.run)} />
      </div>

      <details className="promptDetails">
        <summary>Generated prompt</summary>
        <pre>{props.run.generated_prompt || "Prompt not stored yet."}</pre>
      </details>

      {props.run.error_message ? <InlineNotice tone="danger">{props.run.error_message}</InlineNotice> : null}

      <div className="streamHeader">
        <div>
          <h4>Output stream</h4>
          <p>{streamStateLabel(state, props.active)}</p>
        </div>
        <span className="eventCount">{visibleEvents.length} events</span>
      </div>

      <div className="outputViewport" role="log" aria-live="polite" aria-label="Run output events">
        {visibleEvents.length === 0 ? (
          <div className="outputEmpty">
            {state === "error" ? "Unable to open SSE stream. Stored events will still load on refresh." : "No events recorded yet."}
          </div>
        ) : (
          visibleEvents.map((event) => (
            <Fragment key={`${event.run_id}-${event.seq}`}>
              <div className={`eventLine event-${event.event_type.replace(".", "-")}`}>
                <span className="eventMeta">
                  <span className="mono">#{event.seq}</span>
                  <span>{event.event_type}</span>
                  <span>{event.stream}</span>
                </span>
                {shouldRenderEventMarkdown(event) ? (
                  <MarkdownContent compact>{eventSummary(event)}</MarkdownContent>
                ) : (
                  <pre>{eventSummary(event)}</pre>
                )}
                {event.event_type === "codex.event" && event.payload.raw !== undefined ? (
                  <details>
                    <summary>raw</summary>
                    <pre>{JSON.stringify(event.payload.raw, null, 2)}</pre>
                  </details>
                ) : null}
              </div>
            </Fragment>
          ))
        )}
      </div>

      {props.run.final_message ? (
        <div className="finalMessage">
          <h4>Final message</h4>
          <MarkdownContent compact>{props.run.final_message}</MarkdownContent>
        </div>
      ) : null}
    </section>
  );
}


function streamStateLabel(state: string, active: boolean) {
  if (state === "live") {
    return active ? "SSE live" : "SSE connected for replay";
  }
  if (state === "connecting") {
    return "Connecting to SSE";
  }
  if (state === "reconnecting") {
    return "Reconnecting with after_seq cursor";
  }
  if (state === "closed") {
    return "Stream closed after final event";
  }
  if (state === "error") {
    return "Stream error";
  }
  return "Stream idle";
}
