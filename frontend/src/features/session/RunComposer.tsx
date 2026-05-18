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
import type { FormEvent } from "react";
import type { CodexReasoningEffort, CodexServiceTier, CreateRunMode, Run, Task } from "../../types";
import { modelOptions, reasoningEffortOptions } from "../../shared/constants";
import { useI18n } from "../../shared/i18n";
import { InlineNotice } from "../../shared/ui";

export function RunComposer(props: {
  task: Task;
  runs: Run[];
  message: string;
  onMessageChange: (value: string) => void;
  mode: CreateRunMode;
  onModeChange: (value: CreateRunMode) => void;
  codexModel: string;
  onCodexModelChange: (value: string) => void;
  reasoningEffort: CodexReasoningEffort;
  onReasoningEffortChange: (value: CodexReasoningEffort) => void;
  serviceTier: CodexServiceTier;
  onServiceTierChange: (value: CodexServiceTier) => void;
  goalMode: boolean;
  onGoalModeChange: () => void;
  contextCount: number;
  disabled: boolean;
  canInterrupt: boolean;
  canCompact: boolean;
  canChangeGoal: boolean;
  canMarkDone: boolean;
  markingDone: boolean;
  onMarkDone: () => void;
  blockedReason?: string;
  submitting: boolean;
  onSubmit: (event: FormEvent) => void;
  onCompact: () => void;
  onInterrupt: () => void;
  interrupting: boolean;
}) {
  const { t } = useI18n();
  const hasObservedSession = Boolean(props.task.codex_session_id || props.runs.some((run) => run.codex_session_id));
  const showMissingSessionHint = props.mode === "resume" && !hasObservedSession;
  const messageReady = Boolean(props.message.trim()) && !showMissingSessionHint;

  return (
    <form className="composer" onSubmit={props.onSubmit}>
      <div className="composerToolbar">
        <div>
          <label htmlFor="run-message">{t("session.instruction")}</label>
          <p>{t("composer.contextSelected", { count: props.contextCount })}</p>
        </div>
        <label className="selectLabel" htmlFor="run-mode">
          {t("composer.mode")}
          <select
            id="run-mode"
            value={props.mode}
            onChange={(event) => props.onModeChange(event.target.value as CreateRunMode)}
            disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
          >
            <option value="auto">auto</option>
            <option value="resume">resume</option>
            <option value="new">new</option>
          </select>
        </label>
        <label className="selectLabel" htmlFor="run-model">
          {t("composer.model")}
          <input
            id="run-model"
            list="codex-model-options"
            value={props.codexModel}
            onChange={(event) => props.onCodexModelChange(event.target.value)}
            disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
            placeholder={t("composer.modelDefault")}
          />
          <datalist id="codex-model-options">
            {modelOptions.filter(Boolean).map((model) => (
              <option key={model} value={model} />
            ))}
          </datalist>
        </label>
        <label className="selectLabel" htmlFor="run-reasoning-effort">
          {t("composer.reasoning")}
          <select
            id="run-reasoning-effort"
            value={props.reasoningEffort}
            onChange={(event) => props.onReasoningEffortChange(event.target.value as CodexReasoningEffort)}
            disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
          >
            {reasoningEffortOptions.map((effort) => (
              <option key={effort || "default"} value={effort}>
                {effort || t("composer.modelDefault")}
              </option>
            ))}
          </select>
        </label>
        <div className="composerSwitchGroup" aria-label="Codex controls">
          <button
            className={`optionToggle ${props.serviceTier === "fast" ? "isSelected" : ""}`}
            type="button"
            onClick={() => props.onServiceTierChange(props.serviceTier === "fast" ? "" : "fast")}
            disabled={props.submitting || props.interrupting}
            title={t("composer.fastTitle")}
          >
            <Zap size={14} />
            {t("composer.fast")}
          </button>
          <button
            className={`optionToggle ${props.goalMode ? "isSelected" : ""}`}
            type="button"
            onClick={props.onGoalModeChange}
            disabled={!props.canChangeGoal || props.submitting || props.interrupting}
            title={t("composer.goalTitle")}
          >
            <ClipboardList size={14} />
            {t("composer.goal")}
          </button>
          <button
            className="ghostButton compact"
            type="button"
            onClick={props.onCompact}
            disabled={!props.canCompact || props.submitting || props.interrupting}
            title={t("composer.compactTitle")}
          >
            <FileText size={14} />
            {t("composer.compact")}
          </button>
        </div>
      </div>
      <textarea
        id="run-message"
        className="instructionInput"
        value={props.message}
        onChange={(event) => props.onMessageChange(event.target.value)}
        disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
        placeholder={t("composer.placeholder")}
        rows={6}
      />
      {props.blockedReason ? <InlineNotice tone="info">{props.blockedReason}</InlineNotice> : null}
      {showMissingSessionHint ? <InlineNotice tone="danger">{t("composer.noSession")}</InlineNotice> : null}
      <div className="composerActions">
        <span className="targetHint">{t("composer.targetHint")}</span>
        <button
          className="primaryButton"
          type="submit"
          disabled={props.disabled || props.submitting || props.interrupting || !messageReady}
        >
          {props.submitting ? <Loader2 className="spin" size={16} /> : <Play size={16} />}
          {t("composer.send")}
        </button>
        {props.canInterrupt ? (
          <button
            className="interruptButton"
            type="button"
            onClick={props.onInterrupt}
            disabled={props.submitting || props.interrupting || !messageReady}
          >
            {props.interrupting ? <Loader2 className="spin" size={16} /> : <Square size={16} />}
            {t("composer.interrupt")}
          </button>
        ) : null}
        <button
          className="doneButton"
          type="button"
          onClick={props.onMarkDone}
          disabled={!props.canMarkDone || props.submitting || props.interrupting || props.markingDone}
          title={t("complete.markDoneTitle")}
        >
          {props.markingDone ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
          {t("complete.markDone")}
        </button>
      </div>
    </form>
  );
}
