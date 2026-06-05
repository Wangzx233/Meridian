import {
  AlertTriangle,
  Archive,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronUp,
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
import { useEffect, useRef, useState } from "react";
import type { CSSProperties, FormEvent, PointerEvent as ReactPointerEvent } from "react";
import { api } from "../../api";
import type {
  CodexReasoningEffort,
  CodexServiceTier,
  ContextItem,
  ContextScope,
  ContextType,
  CreateRunMode,
  CreateRunRequest,
  CreateRunResponse,
  MarkDoneRequest,
  Project,
  Run,
  Server,
  Task,
  TaskMemoryDraft,
} from "../../types";
import { isActiveRunStatus, shortId } from "../../utils";
import type { LoadState } from "../../shared/loadState";
import type { Notice } from "../../shared/notices";
import { composerDefaultHeight, composerMaxHeight, composerMinHeight, sidePanelDefaultWidth, sidePanelMaxWidth, sidePanelMinWidth, type SidePanelTab, type WorkbenchTab } from "../../shared/constants";
import { useI18n } from "../../shared/i18n";
import { serverDisplayName } from "../../shared/serverDisplay";
import { useStoredPanelSize, useStoredString } from "../../shared/storage";
import { EmptyState, Fact, LoadingState, ResizeHandle, StatusBadge } from "../../shared/ui";
import { AgentsFilePanel } from "../agents/AgentsFilePanel";
import { ContextPanel } from "../context/ContextPanel";
import { ProjectFilesPanel, type ProjectFileOpenRequest } from "../files/ProjectFilesPanel";
import { PromptPanel, RunHistory, RunOutputWorkspace } from "../runs/RunOutputWorkspace";
import { TerminalPanel } from "../terminal/TerminalPanel";
import { RunComposer } from "./RunComposer";

export function TaskSessionPanel(props: {
  server: Server | null;
  project: Project | null;
  task: Task | null;
  taskLoading: boolean;
  runs: Run[];
  runsState: LoadState;
  activeRun: Run | null;
  selectedRun: Run | null;
  selectedRunId: string | null;
  onSelectRun: (runId: string) => void;
  contextItems: ContextItem[];
  contextState: LoadState;
  onCreateContext: (input: {
    server_id?: string | null;
    project_id?: string | null;
    scope: ContextScope;
    task_id: string | null;
    type: ContextType;
    title: string;
    content: string;
    tags: string[];
  }) => void;
  onUpdateContext: (
    id: string,
    input: {
      server_id?: string | null;
      project_id?: string | null;
      scope: ContextScope;
      task_id: string | null;
      type: ContextType;
      title: string;
      content: string;
      tags: string[];
    },
  ) => void;
  onDeleteContext: (id: string) => void;
  creatingContext: boolean;
  updatingContext: boolean;
  deletingContext: boolean;
  onCreateRun: (input: {
    message: string;
    mode: CreateRunMode;
    codex_model: string;
    codex_reasoning_effort: CodexReasoningEffort;
    codex_service_tier: CodexServiceTier;
    raw_command?: boolean;
    reminder_callback_enabled?: boolean;
    context_item_ids: string[];
    input_images?: CreateRunRequest["input_images"];
  }) => void;
  creatingRun: boolean;
  onInterruptRun: (input: {
    message: string;
    mode: CreateRunMode;
    codex_model: string;
    codex_reasoning_effort: CodexReasoningEffort;
    codex_service_tier: CodexServiceTier;
    raw_command?: boolean;
    reminder_callback_enabled?: boolean;
    context_item_ids: string[];
    input_images?: CreateRunRequest["input_images"];
  }) => void;
  interruptingRun: boolean;
  onNotice: (notice: Notice) => void;
  onCancelRun: (runId: string) => void;
  cancelingRun: boolean;
  onDraftMemory: (input: CreateRunRequest) => Promise<CreateRunResponse>;
  onMarkDone: (input: MarkDoneRequest) => void;
  markingDone: boolean;
}) {
  const { language, t } = useI18n();
  const [selectedContextIds, setSelectedContextIds] = useState<string[]>([]);
  const [message, setMessageState] = useState("");
  const [mode, setMode] = useState<CreateRunMode>("auto");
  const [codexModel, setCodexModel] = useStoredString("ctw.codexModel", "");
  const [reasoningEffort, setReasoningEffort] = useStoredString("ctw.reasoningEffort", "") as [
    CodexReasoningEffort,
    (value: CodexReasoningEffort) => void,
  ];
  const [serviceTier, setServiceTier] = useStoredString("ctw.codexServiceTier", "") as [
    CodexServiceTier,
    (value: CodexServiceTier) => void,
  ];
  const [goalMode, setGoalMode] = useState(false);
  const [reminderCallbacksEnabled, setReminderCallbacksEnabled] = useState(false);
  const [inputImages, setInputImages] = useState<CreateRunRequest["input_images"]>([]);
  const [taskMemory, setTaskMemory] = useState<TaskMemoryDraft>(emptyTaskMemoryDraft());
  const [memoryDetailsOpen, setMemoryDetailsOpen] = useState(false);
  const [memoryDraftRunId, setMemoryDraftRunId] = useState<string | null>(null);
  const taskPageKey = props.task?.id ? `ctw.taskPage.${props.task.id}` : null;
  const [storedWorkbenchTab, setStoredWorkbenchTab] = useStoredString(taskPageKey ? `${taskPageKey}.workspace` : null, "output");
  const [storedSideTab, setStoredSideTab] = useStoredString(taskPageKey ? `${taskPageKey}.side` : null, "context");
  const [mobileLayout, setMobileLayout] = useState(() => isMobileWorkbenchLayout());
  const activeWorkbenchTab = validWorkbenchTab(storedWorkbenchTab);
  const visibleWorkspaceTab = mobileLayout && activeWorkbenchTab === "terminal" ? "output" : activeWorkbenchTab;
  const activeSideTab = validSidePanelTab(storedSideTab);
  const [visitedWorkbenchTabs, setVisitedWorkbenchTabs] = useState<WorkbenchTab[]>(() => [activeWorkbenchTab]);
  const projectId = props.project?.id ?? null;
  const [projectFileOpenRequest, setProjectFileOpenRequest] = useState<ProjectFileOpenRequest | null>(null);
  const projectFileOpenRequestIdRef = useRef(0);
  const currentProjectIdRef = useRef(projectId);
  const [sidePanelCollapsed, setSidePanelCollapsed] = useState(() => isMobileWorkbenchLayout());
  const [composerCollapsed, setComposerCollapsed] = useState(false);
  const [sidePanelDrag, setSidePanelDrag] = useState<{ pointerId: number; startY: number; currentY: number } | null>(null);
  const [sidePanelWidth, setSidePanelWidth] = useStoredPanelSize(
    "ctw.sidePanelWidth",
    sidePanelDefaultWidth,
    sidePanelMinWidth,
    sidePanelMaxWidth,
  );
  const [composerHeight, setComposerHeight] = useStoredPanelSize(
    "ctw.composerHeight",
    composerDefaultHeight,
    composerMinHeight,
    composerMaxHeight,
  );
  const messageDraftKey = props.task?.id ? `ctw.runDraft.${props.task.id}` : null;

  const setMessage = (value: string) => {
    setMessageState(value);
    if (!messageDraftKey) {
      return;
    }
    try {
      if (value) {
        window.localStorage.setItem(messageDraftKey, value);
      } else {
        window.localStorage.removeItem(messageDraftKey);
      }
    } catch {
      // Local storage can be unavailable in private or restricted browser modes.
    }
  };

  const setActiveWorkbenchTab = (tab: WorkbenchTab) => {
    setStoredWorkbenchTab(tab);
  };

  const setActiveSideTab = (tab: SidePanelTab) => {
    setStoredSideTab(tab);
  };

  const clearMessageDraft = () => {
    setMessageState("");
    if (!messageDraftKey) {
      return;
    }
    try {
      window.localStorage.removeItem(messageDraftKey);
    } catch {
      // Local storage can be unavailable in private or restricted browser modes.
    }
  };

  useEffect(() => {
    const query = window.matchMedia("(max-width: 920px)");
    const update = () => setMobileLayout(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);

  useEffect(() => {
    if (!mobileLayout) {
      return;
    }
    setSidePanelCollapsed(true);
    setComposerCollapsed(false);
  }, [mobileLayout, props.task?.id]);

  useEffect(() => {
    setSelectedContextIds([]);
    setMode("auto");
    setGoalMode(false);
    setReminderCallbacksEnabled(false);
    setInputImages([]);
    setTaskMemory(emptyTaskMemoryDraft());
    setMemoryDetailsOpen(false);
    setMemoryDraftRunId(null);
  }, [props.task?.id]);

  useEffect(() => {
    if (!messageDraftKey) {
      setMessageState("");
      return;
    }
    try {
      setMessageState(window.localStorage.getItem(messageDraftKey) ?? "");
    } catch {
      setMessageState("");
    }
  }, [messageDraftKey]);

  useEffect(() => {
    setVisitedWorkbenchTabs([activeWorkbenchTab]);
    setProjectFileOpenRequest(null);
  }, [props.task?.id]);

  useEffect(() => {
    currentProjectIdRef.current = projectId;
  }, [projectId]);

  useEffect(() => {
    setVisitedWorkbenchTabs((current) =>
      current.includes(activeWorkbenchTab) ? current : [...current, activeWorkbenchTab],
    );
  }, [activeWorkbenchTab]);

  const taskHasActiveRun = Boolean(props.activeRun) || props.task?.status === "running";
  const canSend = Boolean(props.task) && !taskHasActiveRun && props.task?.status !== "done" && props.task?.status !== "archived";
  const canInterrupt =
    Boolean(props.task && props.activeRun && isActiveRunStatus(props.activeRun.status)) &&
    props.task?.status !== "done" &&
    props.task?.status !== "archived";
  const canMarkDone =
    Boolean(props.task) &&
    !taskHasActiveRun &&
    (props.task?.status === "open" || props.task?.status === "waiting_user");
  const canDraftMemory = canMarkDone;
  const canSaveMemoryContext = Boolean(props.project && props.task && taskMemoryHasContent(taskMemory));
  const hasObservedSession = Boolean(props.task?.codex_session_id || props.runs.some((run) => run.codex_session_id));
  const canCompact = canSend && hasObservedSession;
  const canChangeGoal = canSend && hasObservedSession;
  const canUseCodexReminders = props.server?.runner_connected === true && props.server.runner_capabilities?.codex_reminders === true;
  const canUseImageInput = props.server?.runner_connected === true && props.server.runner_capabilities?.codex_image_input === true;
  const reminderCallbacksOn = canUseCodexReminders && reminderCallbacksEnabled;
  const selectedContextItems = selectedContextIds
    .map((id) => props.contextItems.find((item) => item.id === id))
    .filter((item): item is ContextItem => Boolean(item));
  const draftPrompt = props.task
    ? buildDraftPromptPreview(props.task, mode, hasObservedSession, message, selectedContextItems, reminderCallbacksOn, inputImages?.length ?? 0)
    : "";
  const draftPromptActive = Boolean(props.task && (message.trim() || selectedContextIds.length > 0 || reminderCallbacksOn || (inputImages?.length ?? 0) > 0));
  const activeWorkspaceTab = visibleWorkspaceTab;
  const mountedWorkspaceTabs = Array.from(new Set<WorkbenchTab>(["output", activeWorkspaceTab, ...visitedWorkbenchTabs]));
  const sidePanelDragOffset = sidePanelDrag ? Math.max(0, sidePanelDrag.currentY - sidePanelDrag.startY) : 0;

  const selectWorkbenchTab = (tab: WorkbenchTab) => {
    setActiveWorkbenchTab(tab);
  };

  const collapseSidePanel = () => {
    setSidePanelDrag(null);
    setSidePanelCollapsed(true);
  };

  const startSidePanelDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (!mobileLayout || event.button !== 0) {
      return;
    }
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    setSidePanelDrag({ pointerId: event.pointerId, startY: event.clientY, currentY: event.clientY });
  };

  const moveSidePanelDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (!sidePanelDrag || sidePanelDrag.pointerId !== event.pointerId) {
      return;
    }
    event.preventDefault();
    setSidePanelDrag({ ...sidePanelDrag, currentY: event.clientY });
  };

  const finishSidePanelDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (!sidePanelDrag || sidePanelDrag.pointerId !== event.pointerId) {
      return;
    }
    const totalDrag = event.clientY - sidePanelDrag.startY;
    const dragDistance = Math.max(0, totalDrag);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    setSidePanelDrag(null);
    if (dragDistance > 64 || Math.abs(totalDrag) < 6) {
      setSidePanelCollapsed(true);
    }
  };

  const cancelSidePanelDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    setSidePanelDrag(null);
  };

  const handleProjectFileLinkClick = (href: string) => {
    const targetPath = projectFilePathFromHref(href, props.project?.workdir ?? "");
    if (!targetPath || !props.project) {
      return false;
    }
    void openExistingProjectFile(props.project.id, targetPath);
    return true;
  };

  const openExistingProjectFile = async (clickedProjectId: string, targetPath: string) => {
    try {
      const listing = await api.listProjectFiles(clickedProjectId, parentProjectPath(targetPath));
      const entry = listing.entries.find((item) => normalizeProjectPath(item.path) === targetPath);
      if (!entry || currentProjectIdRef.current !== clickedProjectId) {
        return;
      }
      projectFileOpenRequestIdRef.current += 1;
      setProjectFileOpenRequest({
        id: projectFileOpenRequestIdRef.current,
        path: entry.path,
        isDir: entry.is_dir,
      });
      selectWorkbenchTab("files");
    } catch {
      // Missing files, disconnected runners, and unsupported file browsing are intentionally silent for link clicks.
    }
  };

  const submitRun = (event: FormEvent) => {
    event.preventDefault();
    const trimmedMessage = message.trim();
    if (!trimmedMessage) {
      return;
    }
    if (trimmedMessage === "/fast") {
      setServiceTier("fast");
      clearMessageDraft();
      props.onNotice({ tone: "info", message: t("session.fastEnabled") });
      return;
    }
    if (trimmedMessage === "/compact") {
      if (!canCompact) {
        props.onNotice({ tone: "danger", message: t("session.compactRequiresSession") });
        return;
      }
      submitCompact();
      return;
    }
    if (isGoalCommand(trimmedMessage)) {
      submitGoalCommand(trimmedMessage);
      return;
    }
    if (!canSend) {
      return;
    }
    if ((inputImages?.length ?? 0) > 0 && !canUseImageInput) {
      props.onNotice({ tone: "danger", message: imageInputBlockedReason(props.server, t) });
      return;
    }
    props.onCreateRun({
      message: trimmedMessage,
      mode,
      codex_model: codexModel.trim(),
      codex_reasoning_effort: reasoningEffort,
      codex_service_tier: serviceTier,
      reminder_callback_enabled: reminderCallbacksOn,
      context_item_ids: selectedContextIds,
      input_images: inputImages,
    });
    clearMessageDraft();
    setReminderCallbacksEnabled(false);
    setSelectedContextIds([]);
    setInputImages([]);
  };

  const submitCompact = () => {
    if (!canCompact) {
      return;
    }
    props.onCreateRun({
      message: "/compact",
      mode: "resume",
      codex_model: codexModel.trim(),
      codex_reasoning_effort: reasoningEffort,
      codex_service_tier: serviceTier,
      raw_command: true,
      context_item_ids: [],
    });
    clearMessageDraft();
    setSelectedContextIds([]);
    setInputImages([]);
  };

  const submitInterrupt = () => {
    const trimmedMessage = message.trim();
    if (!trimmedMessage) {
      return;
    }
    if (trimmedMessage === "/fast") {
      setServiceTier("fast");
      clearMessageDraft();
      props.onNotice({ tone: "info", message: t("session.fastEnabled") });
      return;
    }
    if (trimmedMessage === "/compact") {
      props.onNotice({ tone: "danger", message: t("session.compactAfterActive") });
      return;
    }
    if (isGoalCommand(trimmedMessage)) {
      props.onNotice({ tone: "danger", message: t("session.goalAfterActive") });
      return;
    }
    if (!canInterrupt) {
      return;
    }
    if ((inputImages?.length ?? 0) > 0 && !canUseImageInput) {
      props.onNotice({ tone: "danger", message: imageInputBlockedReason(props.server, t) });
      return;
    }
    props.onInterruptRun({
      message: trimmedMessage,
      mode,
      codex_model: codexModel.trim(),
      codex_reasoning_effort: reasoningEffort,
      codex_service_tier: serviceTier,
      reminder_callback_enabled: reminderCallbacksOn,
      context_item_ids: selectedContextIds,
      input_images: inputImages,
    });
    clearMessageDraft();
    setReminderCallbacksEnabled(false);
    setSelectedContextIds([]);
    setInputImages([]);
  };

  const submitGoalCommand = (command: string) => {
    if (!canChangeGoal) {
      props.onNotice({ tone: "danger", message: t("session.goalRequiresSession") });
      return;
    }
    props.onCreateRun({
      message: command,
      mode: "resume",
      codex_model: codexModel.trim(),
      codex_reasoning_effort: reasoningEffort,
      codex_service_tier: serviceTier,
      raw_command: true,
      context_item_ids: [],
    });
    setGoalMode(command !== "/goal clear");
    clearMessageDraft();
    setSelectedContextIds([]);
    setInputImages([]);
  };

  const submitGoalToggle = () => {
    if (!canChangeGoal || !props.task) {
      props.onNotice({ tone: "danger", message: t("session.goalRequiresSession") });
      return;
    }
    const nextGoalMode = !goalMode;
    props.onCreateRun({
      message: nextGoalMode ? buildGoalCommand(props.task.title, props.task.description) : "/goal clear",
      mode: "resume",
      codex_model: codexModel.trim(),
      codex_reasoning_effort: reasoningEffort,
      codex_service_tier: serviceTier,
      raw_command: true,
      context_item_ids: [],
    });
    setGoalMode(nextGoalMode);
    clearMessageDraft();
    setSelectedContextIds([]);
    setInputImages([]);
  };

  const markTaskDone = () => {
    if (!canMarkDone) {
      return;
    }
    const memory = trimTaskMemoryDraft(taskMemory);
    props.onMarkDone({
      summary: memory.problem,
      memory,
    });
  };

  const saveMemoryContext = (event: FormEvent) => {
    event.preventDefault();
    if (!props.project || !props.task) {
      return;
    }
    const memory = trimTaskMemoryDraft(taskMemory);
    const content = formatTaskMemoryContext(memory, {
      problem: t("complete.memoryNote"),
      changes: t("complete.changes"),
      verification: t("complete.verification"),
      files: t("complete.files"),
      stale_conditions: t("complete.risks"),
    });
    if (!content) {
      return;
    }
    props.onCreateContext({
      server_id: props.project.server_id,
      project_id: props.project.id,
      scope: "task",
      task_id: props.task.id,
      type: "task_summary",
      title: t("complete.contextTitle", { title: props.task.title }),
      content,
      tags: ["task-summary"],
    });
  };

  const updateMemoryField = (field: keyof TaskMemoryDraft, value: string) => {
    setTaskMemory((current) => ({ ...current, [field]: value }));
  };

  const draftMemory = async () => {
    if (!canDraftMemory) {
      return;
    }
    try {
      const response = await props.onDraftMemory({
        message: buildTaskMemoryDraftPrompt(language),
        mode: "auto",
        codex_model: codexModel.trim(),
        codex_reasoning_effort: reasoningEffort,
        codex_service_tier: serviceTier,
        context_item_ids: [],
      });
      setMemoryDraftRunId(response.run.id);
      setActiveSideTab("context");
      selectWorkbenchTab("output");
      props.onNotice({ tone: "info", message: t("session.memoryDraftQueued") });
    } catch {
      // The shared run mutation reports the API error.
    }
  };

  useEffect(() => {
    if (!memoryDraftRunId) {
      return;
    }
    const draftRun = props.runs.find((run) => run.id === memoryDraftRunId);
    if (!draftRun || draftRun.status === "queued" || draftRun.status === "running") {
      return;
    }
    if (draftRun.status !== "succeeded") {
      setMemoryDraftRunId(null);
      props.onNotice({ tone: "danger", message: t("session.memoryDraftFailed") });
      return;
    }
    const parsed = parseTaskMemoryDraft(draftRun.final_message ?? "");
    setMemoryDraftRunId(null);
    if (!parsed) {
      props.onNotice({ tone: "danger", message: t("session.memoryDraftParseFailed") });
      return;
    }
    setTaskMemory(parsed);
    setMemoryDetailsOpen(taskMemoryDraftHasDetails(parsed));
    setActiveSideTab("context");
    props.onNotice({ tone: "success", message: t("session.memoryDraftFilled") });
  }, [memoryDraftRunId, props.runs, props.onNotice, t]);

  const summaryPanel = (
    <form className="memorySummaryBox contextSummaryBox" onSubmit={saveMemoryContext}>
      <div className="boxHeader">
        <div>
          <h3>{t("complete.title")}</h3>
        </div>
      </div>
      <div className="memoryActionRow">
        <button
          className="ghostButton compact"
          type="button"
          onClick={draftMemory}
          disabled={!canDraftMemory || props.creatingRun || Boolean(memoryDraftRunId)}
          title={t("complete.draftTitle")}
        >
          {memoryDraftRunId ? <Loader2 className="spin" size={15} /> : <FileText size={15} />}
          {t("complete.draft")}
        </button>
        <button className="ghostButton compact" type="button" onClick={() => setMemoryDetailsOpen((value) => !value)}>
          {memoryDetailsOpen ? <ChevronUp size={15} /> : <ChevronDown size={15} />}
          {t("complete.sections")}
        </button>
        <button
          className="primaryButton compact"
          type="submit"
          disabled={!canSaveMemoryContext || props.creatingContext}
          title={t("complete.saveContextTitle")}
        >
          {props.creatingContext ? <Loader2 className="spin" size={15} /> : <Save size={15} />}
          {t("complete.saveContext")}
        </button>
      </div>
      <label htmlFor="memory-problem">{t("complete.memoryNote")}</label>
      <textarea
        id="memory-problem"
        rows={5}
        value={taskMemory.problem}
        onChange={(event) => updateMemoryField("problem", event.target.value)}
        disabled={props.creatingContext}
        placeholder={t("complete.memoryPlaceholder")}
      />
      {memoryDetailsOpen ? (
        <div className="memorySectionGrid">
          <label>
            {t("complete.changes")}
            <textarea
              rows={3}
              value={taskMemory.changes}
              onChange={(event) => updateMemoryField("changes", event.target.value)}
              disabled={props.creatingContext}
              placeholder={t("complete.changesPlaceholder")}
            />
          </label>
          <label>
            {t("complete.verification")}
            <textarea
              rows={3}
              value={taskMemory.verification}
              onChange={(event) => updateMemoryField("verification", event.target.value)}
              disabled={props.creatingContext}
              placeholder={t("complete.verificationPlaceholder")}
            />
          </label>
          <label>
            {t("complete.files")}
            <textarea
              rows={3}
              value={taskMemory.files}
              onChange={(event) => updateMemoryField("files", event.target.value)}
              disabled={props.creatingContext}
              placeholder={t("complete.filesPlaceholder")}
            />
          </label>
          <label>
            {t("complete.risks")}
            <textarea
              rows={3}
              value={taskMemory.stale_conditions}
              onChange={(event) => updateMemoryField("stale_conditions", event.target.value)}
              disabled={props.creatingContext}
              placeholder={t("complete.risksPlaceholder")}
            />
          </label>
        </div>
      ) : null}
    </form>
  );

  if (!props.project) {
    return (
      <section className="sessionPanel emptySession" aria-label="Task session">
        <EmptyState
          icon={<FolderKanban size={24} />}
          title={t("session.selectProject")}
          body={t("session.selectProjectBody")}
        />
      </section>
    );
  }

  if (props.taskLoading && !props.task) {
    return (
      <section className="sessionPanel emptySession" aria-label="Task session">
        <LoadingState label={t("session.loading")} />
      </section>
    );
  }

  if (!props.task) {
    return (
      <section className="sessionPanel emptySession" aria-label="Task session">
        <EmptyState
          icon={<ClipboardList size={24} />}
          title={t("session.selectTask")}
          body={t("session.selectTaskBody")}
        />
      </section>
    );
  }

  return (
    <section className="sessionPanel" aria-label="Task session">
      <div className="sessionHeader">
        <div className="sessionTitleBlock">
          <div className="breadcrumb">
            <span>{serverDisplayName(props.server) || t("session.unknownServer")}</span>
            <span>/</span>
            <span>{props.project.name}</span>
          </div>
          <div className="titleLine">
            <h2>{props.task.title}</h2>
            <StatusBadge status={props.task.status} />
          </div>
          {props.task.description ? <p title={props.task.description}>{props.task.description}</p> : null}
        </div>
        <div className="sessionFacts" aria-label={t("session.facts")}>
          <Fact label={t("session.workdir")} value={props.project.workdir} mono />
          <Fact label={t("session.rules")} value={props.project.rules_path || t("session.notConfigured")} />
          <Fact label={t("session.session")} value={shortId(props.task.codex_session_id, 18)} mono />
          <Fact label={t("session.activeRun")} value={shortId(props.task.active_run_id, 12)} mono />
        </div>
      </div>

      <div
        className={`workbenchBody ${sidePanelCollapsed ? "sidePanelCollapsed" : ""} ${composerCollapsed ? "composerCollapsedMobile" : ""}`}
        style={
          {
            "--side-panel-width": `${sidePanelWidth}px`,
            "--composer-height": `${composerHeight}px`,
          } as CSSProperties
        }
      >
        <section className={`primaryWorkspace ${composerCollapsed ? "composerCollapsed" : ""}`} aria-label={t("session.workbench")}>
          <div className="workspaceTabs" role="tablist" aria-label={t("session.tools")}>
            <label className="mobileWorkspaceToolSelect" htmlFor="mobile-workspace-tool">
              <span>{t("session.tools")}</span>
              <select
                id="mobile-workspace-tool"
                value={activeWorkspaceTab}
                onChange={(event) => selectWorkbenchTab(event.target.value as WorkbenchTab)}
                aria-label={t("session.tools")}
              >
                <option value="output">{t("session.output")}</option>
                <option value="files">{t("session.files")}</option>
              </select>
            </label>
            <div className="workspaceTabGroup">
              <button
                type="button"
                role="tab"
                aria-selected={activeWorkspaceTab === "output"}
                className={activeWorkspaceTab === "output" ? "isSelected" : ""}
                onClick={() => selectWorkbenchTab("output")}
                title={t("session.output")}
              >
                <TerminalSquare size={15} />
                {t("session.output")}
              </button>
              <button
                className={`desktopOnlyTab ${activeWorkbenchTab === "terminal" ? "isSelected" : ""}`}
                type="button"
                role="tab"
                aria-selected={activeWorkbenchTab === "terminal"}
                onClick={() => setStoredWorkbenchTab("terminal")}
                title={t("session.terminal")}
              >
                <Terminal size={15} />
                {t("session.terminal")}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={activeWorkspaceTab === "files"}
                className={activeWorkspaceTab === "files" ? "isSelected" : ""}
                onClick={() => selectWorkbenchTab("files")}
                title={t("session.files")}
              >
                <FolderOpen size={15} />
                {t("session.files")}
              </button>
            </div>
            <button
              className="miniCollapseButton"
              type="button"
              onClick={() => setComposerCollapsed((value) => !value)}
              aria-label={composerCollapsed ? t("session.expandComposer") : t("session.collapseComposer")}
              aria-expanded={!composerCollapsed}
              title={composerCollapsed ? t("session.expandComposer") : t("session.collapseComposer")}
            >
              {composerCollapsed ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            </button>
          </div>

          <div className="workspaceContent">
            {mountedWorkspaceTabs.includes("output") ? (
              <div className="workspacePane" hidden={activeWorkspaceTab !== "output"}>
                <RunOutputWorkspace
                  run={props.selectedRun}
                  active={Boolean(props.selectedRun && isActiveRunStatus(props.selectedRun.status))}
                  runs={props.runs}
                  runsState={props.runsState}
                  selectedRunId={props.selectedRunId}
                  onSelectRun={props.onSelectRun}
                  activeRun={props.activeRun}
                  onCancelRun={props.onCancelRun}
                  cancelingRun={props.cancelingRun}
                  onProjectFileLinkClick={handleProjectFileLinkClick}
                />
              </div>
            ) : null}

            {mountedWorkspaceTabs.includes("terminal") ? (
              <div className="workspacePane desktopTerminalPane" hidden={activeWorkspaceTab !== "terminal"}>
                <TerminalPanel server={props.server} project={props.project} visible={activeWorkspaceTab === "terminal"} />
              </div>
            ) : null}

            {mountedWorkspaceTabs.includes("files") ? (
              <div className="workspacePane" hidden={activeWorkspaceTab !== "files"}>
                <ProjectFilesPanel server={props.server} project={props.project} openRequest={projectFileOpenRequest} />
              </div>
            ) : null}
          </div>

          <ResizeHandle
            label={t("session.resizeComposer")}
            hidden={composerCollapsed}
            orientation="horizontal"
            direction="next"
            min={composerMinHeight}
            max={composerMaxHeight}
            value={composerHeight}
            onChange={setComposerHeight}
            onTap={mobileLayout ? () => setComposerCollapsed(true) : undefined}
          />

          {composerCollapsed ? (
            <button
              className="composerCollapsedBar"
              type="button"
              onClick={() => setComposerCollapsed(false)}
              aria-label={t("session.expandComposer")}
              title={t("session.expandComposer")}
            >
              <ChevronUp size={14} />
              <span>{message.trim() || t("session.instruction")}</span>
              {selectedContextIds.length > 0 ? <strong>{selectedContextIds.length}</strong> : null}
            </button>
          ) : (
            <RunComposer
              task={props.task}
              runs={props.runs}
              message={message}
              onMessageChange={setMessage}
              mode={mode}
              onModeChange={setMode}
              codexModel={codexModel}
              onCodexModelChange={setCodexModel}
              reasoningEffort={reasoningEffort}
              onReasoningEffortChange={setReasoningEffort}
              serviceTier={serviceTier}
              onServiceTierChange={setServiceTier}
              goalMode={goalMode}
              onGoalModeChange={submitGoalToggle}
              reminderCallbacksEnabled={reminderCallbacksOn}
              onReminderCallbacksChange={() => setReminderCallbacksEnabled((value) => !value)}
              canUseReminderCallbacks={canUseCodexReminders}
              reminderCallbacksBlockedReason={
                canUseCodexReminders
                  ? undefined
                  : returnNoticeBlockedReason(props.server, t)
              }
              contextCount={selectedContextIds.length}
              inputImages={inputImages ?? []}
              onInputImagesChange={setInputImages}
              canUseImageInput={canUseImageInput}
              imageInputBlockedReason={canUseImageInput ? undefined : imageInputBlockedReason(props.server, t)}
              disabled={!canSend || props.creatingRun}
              canInterrupt={canInterrupt}
              canCompact={canCompact}
              canChangeGoal={canChangeGoal}
              canMarkDone={canMarkDone}
              markingDone={props.markingDone}
              onMarkDone={markTaskDone}
              blockedReason={
                taskHasActiveRun
                  ? t("session.runActive")
                  : props.task.status === "done" || props.task.status === "archived"
                    ? t("session.taskClosed")
                    : undefined
              }
              submitting={props.creatingRun}
              onSubmit={submitRun}
              onCompact={submitCompact}
              onInterrupt={submitInterrupt}
              interrupting={props.interruptingRun}
            />
          )}
        </section>

        <ResizeHandle
          label={t("session.resizeTools")}
          hidden={sidePanelCollapsed}
          orientation="vertical"
          direction="next"
          min={sidePanelMinWidth}
          max={sidePanelMaxWidth}
          value={sidePanelWidth}
          onChange={setSidePanelWidth}
        />

        {sidePanelCollapsed ? (
          <aside className="sideRail" aria-label={t("session.collapsedTools")}>
            <button
              className="miniCollapseButton"
              type="button"
              onClick={() => setSidePanelCollapsed(false)}
              aria-label={t("session.expandTools")}
              aria-expanded={false}
              title={t("session.expandTools")}
            >
              <PanelLeftOpen size={14} />
            </button>
            <button type="button" onClick={() => setSidePanelCollapsed(false)} title={t("session.context")} aria-label={t("session.context")}>
              <ClipboardList size={15} />
            </button>
            <button type="button" onClick={() => setSidePanelCollapsed(false)} title={t("session.turns")} aria-label={t("session.turns")}>
              <History size={15} />
            </button>
          </aside>
        ) : (
          <aside
            className={`sidePanel ${sidePanelDrag ? "isDragging" : ""}`}
            aria-label="Task tools and history"
            style={{ "--sheet-drag-y": `${sidePanelDragOffset}px` } as CSSProperties}
          >
            <button
              className="mobileSheetHandleButton"
              type="button"
              onClick={collapseSidePanel}
              onPointerDown={startSidePanelDrag}
              onPointerMove={moveSidePanelDrag}
              onPointerUp={finishSidePanelDrag}
              onPointerCancel={cancelSidePanelDrag}
              aria-label={t("session.collapseTools")}
              title={t("session.collapseTools")}
            >
              <span className="mobileSheetHandle" aria-hidden="true" />
            </button>
            <div className="segmentedTabs" role="tablist" aria-label={t("session.sideViews")}>
              <button
                className="miniCollapseButton"
                type="button"
                onClick={() => setSidePanelCollapsed(true)}
                aria-label={t("session.collapseTools")}
                aria-expanded={!sidePanelCollapsed}
                title={t("session.collapseTools")}
              >
                <PanelRightClose size={14} />
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={activeSideTab === "context"}
                className={activeSideTab === "context" ? "isSelected" : ""}
                onClick={() => setActiveSideTab("context")}
                title={t("session.context")}
              >
                {t("session.context")}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={activeSideTab === "agents"}
                className={activeSideTab === "agents" ? "isSelected" : ""}
                onClick={() => setActiveSideTab("agents")}
                title={t("session.agents")}
              >
                {t("session.agents")}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={activeSideTab === "runs"}
                className={activeSideTab === "runs" ? "isSelected" : ""}
                onClick={() => setActiveSideTab("runs")}
                title={t("session.turns")}
              >
                {t("session.turns")}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={activeSideTab === "prompt"}
                className={activeSideTab === "prompt" ? "isSelected" : ""}
                onClick={() => setActiveSideTab("prompt")}
                title={t("session.prompt")}
              >
                {t("session.prompt")}
              </button>
            </div>

            <div className="sidePanelContent">
              {activeSideTab === "context" ? (
                <ContextPanel
                  server={props.server}
                  project={props.project}
                  task={props.task}
                  contextItems={props.contextItems}
                  state={props.contextState}
                  selectedIds={selectedContextIds}
                  onSelectionChange={setSelectedContextIds}
                  onCreateContext={props.onCreateContext}
                  onUpdateContext={props.onUpdateContext}
                  onDeleteContext={props.onDeleteContext}
                  creating={props.creatingContext}
                  updating={props.updatingContext}
                  deleting={props.deletingContext}
                  summaryPanel={summaryPanel}
                />
              ) : null}

              {activeSideTab === "agents" ? <AgentsFilePanel server={props.server} project={props.project} /> : null}

              {activeSideTab === "runs" ? (
                <RunHistory
                  runs={props.runs}
                  selectedRunId={props.selectedRunId}
                  onSelectRun={(runId) => {
                    props.onSelectRun(runId);
                    setActiveWorkbenchTab("output");
                  }}
                  state={props.runsState}
                  activeRun={props.activeRun}
                  onCancelRun={props.onCancelRun}
                  cancelingRun={props.cancelingRun}
                />
              ) : null}

              {activeSideTab === "prompt" ? (
                <PromptPanel run={props.selectedRun} draftPrompt={draftPrompt} draftActive={draftPromptActive} />
              ) : null}
            </div>
          </aside>
        )}
      </div>
    </section>
  );
}

function emptyTaskMemoryDraft(): TaskMemoryDraft {
  return {
    problem: "",
    changes: "",
    verification: "",
    files: "",
    stale_conditions: "",
  };
}

function trimTaskMemoryDraft(memory: TaskMemoryDraft): TaskMemoryDraft {
  return {
    problem: memory.problem.trim(),
    root_cause: memory.root_cause?.trim() || undefined,
    changes: memory.changes.trim(),
    files: memory.files.trim(),
    decisions: memory.decisions?.trim() || undefined,
    verification: memory.verification.trim(),
    related_tasks: memory.related_tasks?.trim() || undefined,
    source_commit: memory.source_commit?.trim() || undefined,
    stale_conditions: memory.stale_conditions.trim(),
  };
}

function validWorkbenchTab(value: string): WorkbenchTab {
  return value === "terminal" || value === "files" ? value : "output";
}

function isMobileWorkbenchLayout() {
  return typeof window !== "undefined" && window.matchMedia("(max-width: 920px)").matches;
}

function validSidePanelTab(value: string): SidePanelTab {
  return value === "agents" || value === "runs" || value === "prompt" ? value : "context";
}

function buildGoalCommand(title: string, description: string) {
  const parts = [normalizeGoalText(title), normalizeGoalText(description)].filter(Boolean);
  return `/goal ${parts.join("\n\n") || "Complete the current task."}`;
}

function normalizeGoalText(value: string) {
  return value.replace(/\s+/g, " ").trim();
}

function isGoalCommand(message: string) {
  return message === "/goal" || message.startsWith("/goal ");
}

function returnNoticeBlockedReason(server: Server | null, t: (key: string) => string) {
  if (!server) {
    return t("composer.sendBackNoServer");
  }
  if (!server.runner_connected) {
    return t("composer.sendBackOffline");
  }
  return t("composer.sendBackNeedsUpdate");
}

function buildDraftPromptPreview(
  task: Task,
  mode: CreateRunMode,
  hasObservedSession: boolean,
  message: string,
  items: ContextItem[],
  returnNoticeEnabled: boolean,
  inputImageCount: number,
) {
  const command = message.trim();
  if (command && (command === "/compact" || isGoalCommand(command))) {
    return command;
  }
  const resolvedMode = mode === "auto" ? (hasObservedSession ? "resume" : "new") : mode;
  const lines: string[] = [];
  lines.push("Current task:", task.title, "", "Description:");
  lines.push(task.description.trim() || "(no description provided)", "");
  if (resolvedMode === "resume") {
    lines.push("Continue the current Codex session for this task.", "");
  }
  lines.push("User instruction for this turn:", message, "");
  lines.push(resolvedMode === "resume" ? "Additional context selected for this turn:" : "Selected context:");
  if (items.length === 0) {
    lines.push("(none)", "");
  } else {
    items.forEach((item, index) => {
      lines.push(`Context item ${index + 1} [${item.type}]: ${item.title}`, item.content, "");
    });
  }
  if (inputImageCount > 0) {
    lines.push(`Image inputs attached: ${inputImageCount}`, "");
  }
  lines.push("Instructions:");
  if (resolvedMode === "resume") {
    lines.push(
      "- Continue from the existing Codex session.",
      "- Do not repeat already completed work unless needed.",
      "- Current repository code is authoritative.",
      "- Use the current task title and description above as the scope for this turn.",
    );
  } else {
    lines.push(
      "- First inspect the current repository before deciding.",
      "- Historical context is background only.",
      "- Current repository code is authoritative if it conflicts with context.",
      "- Complete this turn and explain changes, verification, and next steps.",
    );
  }
  if (returnNoticeEnabled) {
    lines.push(
      '- Optional: if a long wait needs the user\'s attention before the run finishes, append `send-back --title "..." --message "..."` after the long command; normal completion notices still happen.',
    );
  }
  return lines.join("\n");
}

function imageInputBlockedReason(server: Server | null, t: (key: string) => string) {
  if (!server) {
    return t("composer.imageNoServer");
  }
  if (!server.runner_connected) {
    return t("composer.imageOffline");
  }
  return t("composer.imageNeedsUpdate");
}

function projectFilePathFromHref(href: string, projectWorkdir: string): string | null {
  let value = decodeLinkPath(href);
  if (!value) {
    return null;
  }
  value = stripLinkPathDecorators(value);
  if (!value || !looksLikeFilePathLink(value)) {
    return null;
  }

  if (isAbsolutePath(value)) {
    return absoluteProjectFilePathToRelative(value, projectWorkdir);
  }
  return normalizeProjectPath(value);
}

function decodeLinkPath(href: string): string | null {
  let value = href.trim();
  if (!value || value.startsWith("#")) {
    return null;
  }
  const windowsPath = /^[A-Za-z]:[\\/]/.test(value);
  const protocol = value.match(/^([A-Za-z][A-Za-z0-9+.-]*):/);
  if (protocol && !windowsPath) {
    if (protocol[1].toLowerCase() !== "file") {
      return null;
    }
    try {
      value = new URL(value).pathname;
    } catch {
      return null;
    }
  }
  try {
    return decodeURI(value);
  } catch {
    return value;
  }
}

function stripLinkPathDecorators(path: string): string {
  let value = path.trim().replace(/\\/g, "/");
  const queryIndex = value.indexOf("?");
  const hashIndex = value.indexOf("#");
  const cutIndex = [queryIndex, hashIndex].filter((index) => index >= 0).sort((left, right) => left - right)[0];
  if (cutIndex !== undefined) {
    value = value.slice(0, cutIndex);
  }
  value = value.replace(/^\/([A-Za-z]:\/)/, "$1");
  value = value.replace(/:(\d+)(?::\d+)?$/, "");
  return value.trim();
}

function looksLikeFilePathLink(path: string): boolean {
  const normalized = path.replace(/\\/g, "/");
  if (isAbsolutePath(normalized) || normalized.startsWith(".") || normalized.includes("/")) {
    return true;
  }
  const name = normalized.split("/").pop() ?? "";
  return /^[A-Za-z0-9_.-]+\.[A-Za-z0-9][A-Za-z0-9_.-]*$/.test(name);
}

function isAbsolutePath(path: string): boolean {
  return /^[A-Za-z]:\//.test(path) || path.startsWith("/");
}

function absoluteProjectFilePathToRelative(path: string, projectWorkdir: string): string | null {
  const root = normalizeAbsolutePath(projectWorkdir);
  const target = normalizeAbsolutePath(path);
  if (!root || !target) {
    return null;
  }
  const caseInsensitive = /^[A-Za-z]:\//.test(root);
  const rootCompare = caseInsensitive ? root.toLowerCase() : root;
  const targetCompare = caseInsensitive ? target.toLowerCase() : target;
  if (targetCompare === rootCompare) {
    return null;
  }
  const rootPrefix = rootCompare.endsWith("/") ? rootCompare : `${rootCompare}/`;
  if (!targetCompare.startsWith(rootPrefix)) {
    return null;
  }
  return normalizeProjectPath(target.slice(rootPrefix.length));
}

function normalizeAbsolutePath(path: string): string {
  return stripLinkPathDecorators(path).replace(/\/+$/, "");
}

function normalizeProjectPath(path: string): string {
  const parts: string[] = [];
  for (const part of path.replace(/\\/g, "/").split("/")) {
    if (!part || part === ".") {
      continue;
    }
    if (part === "..") {
      return "";
    }
    parts.push(part);
  }
  return parts.join("/");
}

function parentProjectPath(path: string): string {
  const index = path.lastIndexOf("/");
  if (index <= 0) {
    return "";
  }
  return path.slice(0, index);
}

function taskMemoryDraftHasDetails(memory: TaskMemoryDraft): boolean {
  return Boolean(memory.changes.trim() || memory.verification.trim() || memory.files.trim() || memory.stale_conditions.trim());
}

function taskMemoryHasContent(memory: TaskMemoryDraft): boolean {
  return Boolean(
    memory.problem.trim() ||
      memory.changes.trim() ||
      memory.verification.trim() ||
      memory.files.trim() ||
      memory.stale_conditions.trim(),
  );
}

type TaskMemoryContextField = "problem" | "changes" | "verification" | "files" | "stale_conditions";

function formatTaskMemoryContext(memory: TaskMemoryDraft, labels: Record<TaskMemoryContextField, string>): string {
  const sections: Array<[TaskMemoryContextField, string]> = [
    ["problem", memory.problem],
    ["changes", memory.changes],
    ["verification", memory.verification],
    ["files", memory.files],
    ["stale_conditions", memory.stale_conditions],
  ];
  return sections
    .map(([key, value]) => {
      const text = value.trim();
      return text ? `${labels[key]}:\n${text}` : "";
    })
    .filter(Boolean)
    .join("\n\n");
}

function buildTaskMemoryDraftPrompt(language: "en" | "zh"): string {
  if (language === "zh") {
    return `请为当前任务起草一份简洁的完成记忆。

规则：
- 不要修改文件。
- 不要把任务标记完成。
- 不要编造细节。
- 优先简短、可复用。
- 只写有可见依据且确实有用的字段。
- 不相关或不确定的字段留空字符串。
- JSON key 必须保持英文，不要翻译 key。
- JSON 字段值必须使用中文，除非文件路径、命令、代码符号或专有名词本身是英文。
- 只返回下面这个带标签的 JSON 块，不要输出任何额外说明。

返回这个准确形状：
<task_memory_json>
{
  "problem": "",
  "changes": "",
  "verification": "",
  "files": "",
  "stale_conditions": ""
}
</task_memory_json>`;
  }
  return `Draft a concise completion memory for this task.

Rules:
- Do not modify files.
- Do not mark the task done.
- Do not invent details.
- Prefer brevity.
- Only include fields that are useful and supported by visible evidence.
- Leave irrelevant or uncertain fields empty.
- Keep JSON keys in English exactly as shown.
- Write JSON field values in English, except for file paths, commands, code symbols, or proper nouns.
- Return only the tagged JSON block and no prose.

Return this exact shape:
<task_memory_json>
{
  "problem": "",
  "changes": "",
  "verification": "",
  "files": "",
  "stale_conditions": ""
}
</task_memory_json>`;
}

function parseTaskMemoryDraft(output: string): TaskMemoryDraft | null {
  const match = output.match(/<task_memory_json>\s*([\s\S]*?)\s*<\/task_memory_json>/i);
  if (!match) {
    return null;
  }
  try {
    const parsed: unknown = JSON.parse(match[1]);
    if (!parsed || typeof parsed !== "object") {
      return null;
    }
    const record = parsed as Record<string, unknown>;
    return {
      problem: stringField(record.problem),
      root_cause: optionalStringField(record.root_cause),
      changes: stringField(record.changes),
      files: stringField(record.files),
      decisions: optionalStringField(record.decisions),
      verification: stringField(record.verification),
      related_tasks: optionalStringField(record.related_tasks),
      source_commit: optionalStringField(record.source_commit),
      stale_conditions: stringField(record.stale_conditions),
    };
  } catch {
    return null;
  }
}

function stringField(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function optionalStringField(value: unknown): string | undefined {
  const text = stringField(value).trim();
  return text ? text : undefined;
}
