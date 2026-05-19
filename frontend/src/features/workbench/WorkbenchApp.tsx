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
  Languages,
  X,
  Zap,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../api";
import type { AuthSession, CodexReasoningEffort, CodexServiceTier, ContextScope, ContextType, CreateRunMode, CreateServerRequest, EmailNotificationConfigRequest, ListResponse, MarkDoneRequest, Project, Run, Task, WorkbenchNotification } from "../../types";
import { isActiveRunStatus } from "../../utils";
import {
  activeTaskStatuses,
  navPanelDefaultWidth,
  navPanelMaxWidth,
  navPanelMinWidth,
  productName,
  taskPanelDefaultWidth,
  taskPanelMaxWidth,
  taskPanelMinWidth,
} from "../../shared/constants";
import { MeridianIcon } from "../../shared/MeridianIcon";
import { queryState } from "../../shared/loadState";
import { errorNotice, runnerUpdateNotice } from "../../shared/notices";
import type { Notice } from "../../shared/notices";
import { useI18n } from "../../shared/i18n";
import { useStoredPanelSize } from "../../shared/storage";
import { ResizeHandle, Toast } from "../../shared/ui";
import { notificationMessage, NotificationPopover } from "../notifications/NotificationPopover";
import { NavPanel } from "../navigation/NavPanel";
import { RunnerInstallPopover } from "../runner/RunnerInstallPopover";
import { SettingsDialog } from "../settings/SettingsDialog";
import { TaskSessionPanel } from "../session/TaskSessionPanel";
import { TaskListPanel } from "../tasks/TaskListPanel";

type StoredWorkbenchLocation = {
  serverId: string | null;
  projectId: string | null;
  taskId: string | null;
  runId: string | null;
};

const workbenchLocationStorageKey = "ctw.lastLocation";
const emptyWorkbenchLocation: StoredWorkbenchLocation = {
  serverId: null,
  projectId: null,
  taskId: null,
  runId: null,
};

export function WorkbenchApp(props: { session: AuthSession; onLogout: () => void; loggingOut: boolean }) {
  const queryClient = useQueryClient();
  const { toggleLanguage, t } = useI18n();
  const [initialLocation] = useState(readStoredWorkbenchLocation);
  const [selectedServerId, setSelectedServerId] = useState<string | null>(() => initialLocation.serverId);
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(() => initialLocation.projectId);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(() => initialLocation.taskId);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(() => initialLocation.runId);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [navCollapsed, setNavCollapsed] = useState(false);
  const [taskCollapsed, setTaskCollapsed] = useState(false);
  const [navPanelWidth, setNavPanelWidth] = useStoredPanelSize(
    "ctw.navPanelWidth",
    navPanelDefaultWidth,
    navPanelMinWidth,
    navPanelMaxWidth,
  );
  const [taskPanelWidth, setTaskPanelWidth] = useStoredPanelSize(
    "ctw.taskPanelWidth",
    taskPanelDefaultWidth,
    taskPanelMinWidth,
    taskPanelMaxWidth,
  );
  const [installerOpen, setInstallerOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [browserNotificationPermission, setBrowserNotificationPermission] = useState<NotificationPermission>(
    getBrowserNotificationPermission(),
  );

  const serversQuery = useQuery({
    queryKey: ["servers"],
    queryFn: api.listServers,
    refetchInterval: 5_000,
  });

  const servers = serversQuery.data?.items ?? [];

  useEffect(() => {
    if (servers.length === 0) {
      if (!serversQuery.isLoading && !serversQuery.isFetching) {
        setSelectedServerId(null);
        setSelectedProjectId(null);
        setSelectedTaskId(null);
        setSelectedRunId(null);
      }
      return;
    }

    if (!selectedServerId || !servers.some((server) => server.id === selectedServerId)) {
      setSelectedServerId(servers[0].id);
      setSelectedProjectId(null);
      setSelectedTaskId(null);
      setSelectedRunId(null);
    }
  }, [selectedServerId, servers, serversQuery.isFetching, serversQuery.isLoading]);

  const projectsQuery = useQuery({
    queryKey: ["projects", selectedServerId],
    queryFn: () => api.listProjects(selectedServerId ?? undefined),
    enabled: Boolean(selectedServerId),
  });

  const projects = selectedServerId ? (projectsQuery.data?.items ?? []) : [];

  useEffect(() => {
    if (projects.length === 0) {
      if (!projectsQuery.isLoading && !projectsQuery.isFetching) {
        setSelectedProjectId(null);
        setSelectedTaskId(null);
        setSelectedRunId(null);
      }
      return;
    }

    if (!selectedProjectId || !projects.some((project) => project.id === selectedProjectId)) {
      setSelectedProjectId(projects[0].id);
      setSelectedTaskId(null);
      setSelectedRunId(null);
    }
  }, [projects, projectsQuery.isFetching, projectsQuery.isLoading, selectedProjectId]);

  const selectedServer = servers.find((server) => server.id === selectedServerId) ?? null;
  const selectedProject = projects.find((project) => project.id === selectedProjectId) ?? null;

  const tasksQuery = useQuery({
    queryKey: ["tasks", selectedProjectId],
    queryFn: () => api.listTasks(selectedProjectId!, activeTaskStatuses),
    enabled: Boolean(selectedProjectId),
  });

  const tasks = tasksQuery.data?.items ?? [];

  useEffect(() => {
    if (!selectedTaskId && tasks.length > 0) {
      setSelectedTaskId(tasks[0].id);
      setSelectedRunId(null);
    }
  }, [selectedTaskId, tasks]);

  const taskQuery = useQuery({
    queryKey: ["task", selectedTaskId],
    queryFn: () => api.getTask(selectedTaskId!),
    enabled: Boolean(selectedTaskId),
  });

  const taskFromList = tasks.find((task) => task.id === selectedTaskId) ?? null;
  const selectedTask = taskQuery.data ?? taskFromList;
  const projectServer = selectedProject ? (servers.find((server) => server.id === selectedProject.server_id) ?? null) : selectedServer;

  const runsQuery = useQuery({
    queryKey: ["runs", selectedTaskId],
    queryFn: () => api.listRuns(selectedTaskId!),
    enabled: Boolean(selectedTaskId),
  });

  const runs = runsQuery.data?.items ?? [];
  const latestRun = getLatestRun(runs);
  const activeRun = useMemo(() => {
    if (!selectedTask) {
      return null;
    }

    if (selectedTask.active_run_id) {
      const run = runs.find((run) => run.id === selectedTask.active_run_id) ?? null;
      return run && isActiveRunStatus(run.status) ? run : null;
    }

    return runs.find((run) => isActiveRunStatus(run.status)) ?? null;
  }, [runs, selectedTask]);

  useEffect(() => {
    if (runs.length === 0) {
      setSelectedRunId(null);
      return;
    }

    if (selectedRunId && runs.some((run) => run.id === selectedRunId)) {
      return;
    }

    setSelectedRunId(activeRun?.id ?? latestRun?.id ?? null);
  }, [activeRun, latestRun, runs, selectedRunId]);

  const selectedRun = runs.find((run) => run.id === selectedRunId) ?? activeRun ?? latestRun;

  useEffect(() => {
    writeStoredWorkbenchLocation({
      serverId: selectedServerId,
      projectId: selectedProjectId,
      taskId: selectedTaskId,
      runId: selectedRunId,
    });
  }, [selectedProjectId, selectedRunId, selectedServerId, selectedTaskId]);

  useEffect(() => {
    if (!selectedTaskId || (!activeRun && selectedTask?.status !== "running")) {
      return;
    }
    const timer = window.setInterval(() => {
      void queryClient.invalidateQueries({ queryKey: ["runs", selectedTaskId] });
      void queryClient.invalidateQueries({ queryKey: ["task", selectedTaskId] });
      if (selectedProjectId) {
        void queryClient.invalidateQueries({ queryKey: ["tasks", selectedProjectId] });
      }
    }, 2_500);
    return () => window.clearInterval(timer);
  }, [activeRun, queryClient, selectedProjectId, selectedTask?.status, selectedTaskId]);

  const contextQuery = useQuery({
    queryKey: ["context-items", selectedProjectId, selectedTaskId],
    queryFn: () => api.listContextItems(selectedProjectId!, selectedTaskId ?? undefined),
    enabled: Boolean(selectedProjectId),
  });

  const contextItems = contextQuery.data?.items ?? [];

  const workbenchNotificationsQuery = useQuery({
    queryKey: ["workbench-notifications", "pending"],
    queryFn: () => api.listWorkbenchNotifications(true),
    refetchInterval: notificationsOpen ? 5_000 : 10_000,
  });

  const workbenchNotifications = workbenchNotificationsQuery.data?.items ?? [];

  const emailNotificationsQuery = useQuery({
    queryKey: ["settings", "email-notifications"],
    queryFn: api.listEmailNotificationConfigs,
    enabled: settingsOpen,
  });

  const emailNotificationConfigs = emailNotificationsQuery.data?.items ?? [];

  const createServerMutation = useMutation({
    mutationFn: (body: CreateServerRequest) => api.createServer(body),
    onSuccess: (server) => {
      setSelectedServerId(server.id);
      setSelectedProjectId(null);
      setSelectedTaskId(null);
      setSelectedRunId(null);
      setNotice({ tone: "success", message: t("app.serverCreated") });
      void queryClient.invalidateQueries({ queryKey: ["servers"] });
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.serverCreateFailed"))),
  });

  const updateServerAliasMutation = useMutation({
    mutationFn: (input: { serverId: string; alias: string }) =>
      api.patchServer(input.serverId, { alias: input.alias }),
    onSuccess: () => {
      setNotice({ tone: "success", message: t("app.serverUpdated") });
      void queryClient.invalidateQueries({ queryKey: ["servers"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.serverUpdateFailed"))),
  });

  const deleteServerMutation = useMutation({
    mutationFn: (serverId: string) => api.deleteServer(serverId),
    onSuccess: (_, serverId) => {
      if (selectedServerId === serverId) {
        const nextServer = servers.find((server) => server.id !== serverId) ?? null;
        setSelectedServerId(nextServer?.id ?? null);
        setSelectedProjectId(null);
        setSelectedTaskId(null);
        setSelectedRunId(null);
      }
      setNotice({ tone: "info", message: t("app.serverDeleted") });
      void queryClient.invalidateQueries({ queryKey: ["servers"] });
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
      void queryClient.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.serverDeleteFailed"))),
  });

  const updateAllRunnersMutation = useMutation({
    mutationFn: api.updateAllRunners,
    onSuccess: (response) => {
      setNotice(runnerUpdateNotice(response));
      void queryClient.invalidateQueries({ queryKey: ["servers"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.runnersUpdateFailed"))),
  });

  const createProjectMutation = useMutation({
    mutationFn: (body: {
      server_id: string;
      name: string;
      workdir: string;
      default_branch: string;
      rules_path: string;
    }) => api.createProject(body),
    onSuccess: (project) => {
      setSelectedServerId(project.server_id);
      setSelectedProjectId(project.id);
      setSelectedTaskId(null);
      setSelectedRunId(null);
      setNotice({ tone: "success", message: t("app.projectCreated") });
      void queryClient.invalidateQueries({ queryKey: ["projects", project.server_id] });
      void queryClient.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.projectCreateFailed"))),
  });

  const deleteProjectMutation = useMutation({
    mutationFn: (projectId: string) => api.deleteProject(projectId),
    onSuccess: (_, projectId) => {
      queryClient.setQueriesData<ListResponse<Project>>({ queryKey: ["projects"] }, (current) => {
        if (!current) {
          return current;
        }
        return {
          ...current,
          items: current.items.filter((project) => project.id !== projectId),
        };
      });
      if (selectedProjectId === projectId) {
        const nextProject = projects.find((project) => project.id !== projectId) ?? null;
        setSelectedProjectId(nextProject?.id ?? null);
        setSelectedTaskId(null);
        setSelectedRunId(null);
      }
      setNotice({ tone: "info", message: t("app.projectDeleted") });
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
      void queryClient.invalidateQueries({ queryKey: ["tasks"] });
      void queryClient.invalidateQueries({ queryKey: ["task"] });
      void queryClient.invalidateQueries({ queryKey: ["runs"] });
      void queryClient.invalidateQueries({ queryKey: ["context-items"] });
      void queryClient.invalidateQueries({ queryKey: ["workbench-notifications", "pending"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.projectDeleteFailed"))),
  });

  const createTaskMutation = useMutation({
    mutationFn: (body: { title: string; description: string }) => api.createTask(selectedProjectId!, body),
    onSuccess: (task) => {
      setSelectedTaskId(task.id);
      setSelectedRunId(null);
      setNotice({ tone: "success", message: t("app.taskCreated") });
      void queryClient.invalidateQueries({ queryKey: ["tasks", selectedProjectId] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.taskCreateFailed"))),
  });

  const createContextMutation = useMutation({
    mutationFn: (body: {
      server_id?: string | null;
      project_id?: string | null;
      scope: ContextScope;
      task_id: string | null;
      type: ContextType;
      title: string;
      content: string;
      tags: string[];
    }) => api.createContextItem(selectedProjectId!, body),
    onSuccess: () => {
      setNotice({ tone: "success", message: t("app.contextSaved") });
      void queryClient.invalidateQueries({ queryKey: ["context-items", selectedProjectId, selectedTaskId] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.contextSaveFailed"))),
  });

  const updateContextMutation = useMutation({
    mutationFn: (input: {
      id: string;
      body: {
        server_id?: string | null;
        project_id?: string | null;
        scope: ContextScope;
        task_id: string | null;
        type: ContextType;
        title: string;
        content: string;
        tags: string[];
      };
    }) => api.patchContextItem(input.id, input.body),
    onSuccess: () => {
      setNotice({ tone: "success", message: t("app.contextUpdated") });
      void queryClient.invalidateQueries({ queryKey: ["context-items", selectedProjectId, selectedTaskId] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.contextUpdateFailed"))),
  });

  const deleteContextMutation = useMutation({
    mutationFn: (contextItemId: string) => api.deleteContextItem(contextItemId),
    onSuccess: () => {
      setNotice({ tone: "info", message: t("app.contextDeleted") });
      void queryClient.invalidateQueries({ queryKey: ["context-items", selectedProjectId, selectedTaskId] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.contextDeleteFailed"))),
  });

  const createEmailNotificationMutation = useMutation({
    mutationFn: (body: EmailNotificationConfigRequest) => api.createEmailNotificationConfig(body),
    onSuccess: () => {
      setNotice({ tone: "success", message: t("app.emailSaved") });
      void queryClient.invalidateQueries({ queryKey: ["settings", "email-notifications"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.emailSaveFailed"))),
  });

  const updateEmailNotificationMutation = useMutation({
    mutationFn: (input: { id: string; body: Partial<EmailNotificationConfigRequest> }) =>
      api.patchEmailNotificationConfig(input.id, input.body),
    onSuccess: () => {
      setNotice({ tone: "success", message: t("app.emailUpdated") });
      void queryClient.invalidateQueries({ queryKey: ["settings", "email-notifications"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.emailUpdateFailed"))),
  });

  const deleteEmailNotificationMutation = useMutation({
    mutationFn: (id: string) => api.deleteEmailNotificationConfig(id),
    onSuccess: () => {
      setNotice({ tone: "info", message: t("app.emailDeleted") });
      void queryClient.invalidateQueries({ queryKey: ["settings", "email-notifications"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.emailDeleteFailed"))),
  });

  const selectCreatedRun = (run: Run, task: Task) => {
    queryClient.setQueryData<ListResponse<Run>>(["runs", run.task_id], (current) => {
      const previousItems = current?.items ?? [];
      const items = previousItems.some((item) => item.id === run.id)
        ? previousItems.map((item) => (item.id === run.id ? run : item))
        : [...previousItems, run];

      return {
        items: sortRunsByCreatedAt(items),
        next_cursor: current?.next_cursor ?? null,
      };
    });
    queryClient.setQueryData(["task", task.id], task);
    if (selectedProjectId) {
      queryClient.setQueryData<ListResponse<Task>>(["tasks", selectedProjectId], (current) => {
        if (!current) {
          return current;
        }
        return {
          ...current,
          items: current.items.map((item) => (item.id === task.id ? task : item)),
        };
      });
    }
    setSelectedRunId(run.id);
  };

  const createRunMutation = useMutation({
    mutationFn: (body: {
      message: string;
      mode: CreateRunMode;
      codex_model: string;
      codex_reasoning_effort: CodexReasoningEffort;
      codex_service_tier: CodexServiceTier;
      raw_command?: boolean;
      context_item_ids: string[];
    }) =>
      api.createRun(selectedTaskId!, body, crypto.randomUUID()),
    onSuccess: (response) => {
      selectCreatedRun(response.run, response.task);
      setNotice({ tone: "success", message: t("app.runQueued") });
      void queryClient.invalidateQueries({ queryKey: ["runs", selectedTaskId] });
      void queryClient.invalidateQueries({ queryKey: ["tasks", selectedProjectId] });
      void queryClient.invalidateQueries({ queryKey: ["task", selectedTaskId] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.runCreateFailed"))),
  });

  const interruptRunMutation = useMutation({
    mutationFn: (body: {
      message: string;
      mode: CreateRunMode;
      codex_model: string;
      codex_reasoning_effort: CodexReasoningEffort;
      codex_service_tier: CodexServiceTier;
      raw_command?: boolean;
      context_item_ids: string[];
    }) => api.interruptRun(selectedTaskId!, body, crypto.randomUUID()),
    onSuccess: (response) => {
      selectCreatedRun(response.run, response.task);
      setNotice({ tone: "info", message: t("app.runInterrupted") });
      void queryClient.invalidateQueries({ queryKey: ["runs", selectedTaskId] });
      void queryClient.invalidateQueries({ queryKey: ["tasks", selectedProjectId] });
      void queryClient.invalidateQueries({ queryKey: ["task", selectedTaskId] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.runInterruptFailed"))),
  });

  const cancelRunMutation = useMutation({
    mutationFn: (runId: string) => api.cancelRun(runId, "User canceled from task page."),
    onSuccess: () => {
      setNotice({ tone: "info", message: t("app.cancelRequested") });
      void queryClient.invalidateQueries({ queryKey: ["runs", selectedTaskId] });
      void queryClient.invalidateQueries({ queryKey: ["task", selectedTaskId] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.cancelFailed"))),
  });

  const markDoneMutation = useMutation({
    mutationFn: (body: MarkDoneRequest) => api.markTaskDone(selectedTaskId!, body),
    onSuccess: () => {
      setNotice({ tone: "success", message: t("app.taskMarkedDone") });
      void queryClient.invalidateQueries({ queryKey: ["tasks", selectedProjectId] });
      void queryClient.invalidateQueries({ queryKey: ["task", selectedTaskId] });
      void queryClient.invalidateQueries({ queryKey: ["workbench-notifications", "pending"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.markDoneFailed"))),
  });

  const acknowledgeNotificationMutation = useMutation({
    mutationFn: (id: string) => api.acknowledgeWorkbenchNotification(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["workbench-notifications", "pending"] });
    },
    onError: (error) => setNotice(errorNotice(error, t("app.notificationUpdateFailed"))),
  });

  const refreshAll = () => {
    void queryClient.invalidateQueries();
  };

  const acknowledgeNotification = (notificationId: string) => {
    acknowledgeNotificationMutation.mutate(notificationId);
  };

  const openNotificationTarget = (notification: WorkbenchNotification) => {
    setSelectedServerId(notification.server_id);
    setSelectedProjectId(notification.project_id);
    setSelectedTaskId(notification.task_id);
    setSelectedRunId(notification.run_id ?? null);
    setNotificationsOpen(false);
    acknowledgeNotification(notification.id);
  };

  const enableBrowserNotifications = async () => {
    if (!("Notification" in window)) {
      setNotice({ tone: "info", message: t("app.browserNotificationsUnsupported") });
      return;
    }
    const permission = await window.Notification.requestPermission();
    setBrowserNotificationPermission(permission);
    if (permission === "granted") {
      setNotice({ tone: "success", message: t("app.browserNotificationsEnabled") });
    }
  };

  useEffect(() => {
    if (browserNotificationPermission !== "granted" || !("Notification" in window)) {
      return;
    }
    for (const notification of workbenchNotifications) {
      const key = `ctw.browserNotified.${notification.id}`;
      try {
        if (window.localStorage.getItem(key)) {
          continue;
        }
        const browserNotification = new window.Notification(notification.title, {
          body: notificationMessage(notification),
          tag: notification.id,
        });
        browserNotification.onclick = () => {
          window.focus();
          openNotificationTarget(notification);
        };
        window.localStorage.setItem(key, "1");
      } catch {
        // Browser notification or local storage may be unavailable under restrictive policies.
      }
    }
  }, [browserNotificationPermission, workbenchNotifications]);

  return (
    <div className="appShell">
      <a className="skipLink" href="#main-workbench">
        {t("app.skip")}
      </a>
      <header className="topBar">
        <div className="brandBlock">
          <div className="brandMark" aria-hidden="true">
            <MeridianIcon size={24} />
          </div>
          <div>
            <h1>{productName}</h1>
            <p>{t("app.tagline")}</p>
          </div>
        </div>
        <div className="topBarMeta">
          <div className="notificationMenu">
            <button
              className={`iconButton notificationButton ${notificationsOpen ? "isActive" : ""}`}
              type="button"
              onClick={() => setNotificationsOpen((value) => !value)}
              aria-label={
                workbenchNotifications.length
                  ? t("app.notificationsPending", { count: workbenchNotifications.length })
                  : t("app.notifications")
              }
              title={
                workbenchNotifications.length
                  ? t("app.notificationsPending", { count: workbenchNotifications.length })
                  : t("app.notifications")
              }
              aria-haspopup="dialog"
              aria-expanded={notificationsOpen}
              aria-controls="notification-popover"
            >
              <Bell size={16} />
              {workbenchNotifications.length > 0 ? (
                <span className="notificationCount" aria-hidden="true">
                  {workbenchNotifications.length > 99 ? "99+" : workbenchNotifications.length}
                </span>
              ) : null}
            </button>
            {notificationsOpen ? (
              <NotificationPopover
                notifications={workbenchNotifications}
                state={queryState(workbenchNotificationsQuery)}
                acknowledging={acknowledgeNotificationMutation.isPending}
                browserPermission={browserNotificationPermission}
                onOpenNotification={openNotificationTarget}
                onAcknowledge={acknowledgeNotification}
                onEnableBrowserNotifications={enableBrowserNotifications}
                onClose={() => setNotificationsOpen(false)}
              />
            ) : null}
          </div>
          <button
            className={`iconButton ${settingsOpen ? "isActive" : ""}`}
            type="button"
            onClick={() => setSettingsOpen(true)}
            aria-label={t("app.settings")}
            title={t("app.settings")}
          >
            <SettingsIcon size={16} />
          </button>
          <div className="runnerInstallMenu">
            <button
              className={`iconButton ${installerOpen ? "isActive" : ""}`}
              type="button"
              onClick={() => setInstallerOpen((value) => !value)}
              aria-label={t("app.runnerInstall")}
              title={t("app.runnerInstall")}
              aria-haspopup="dialog"
              aria-expanded={installerOpen}
              aria-controls="runner-install-popover"
            >
              <TerminalSquare size={16} />
            </button>
            {installerOpen ? (
              <RunnerInstallPopover
                server={selectedServer}
                runnerToken={props.session.runner_token ?? ""}
                onClose={() => setInstallerOpen(false)}
                onNotice={setNotice}
              />
            ) : null}
          </div>
          <button
            className="languageToggle"
            type="button"
            onClick={toggleLanguage}
            aria-label={t("app.languageSwitch")}
            title={t("app.languageSwitch")}
          >
            <Languages size={15} />
            <span>{t("app.languageValue")}</span>
          </button>
          <button className="iconButton" type="button" onClick={refreshAll} aria-label={t("app.refreshAria")} title={t("app.refresh")}>
            <RefreshCw size={16} />
          </button>
          <button
            className="iconButton"
            type="button"
            onClick={props.onLogout}
            disabled={props.loggingOut}
            aria-label={props.session.username ? `${t("app.signOut")} ${props.session.username}` : t("app.signOut")}
            title={props.session.username ? t("app.signedInAs", { username: props.session.username }) : t("app.signOut")}
          >
            {props.loggingOut ? <Loader2 className="spin" size={16} /> : <LogOut size={16} />}
          </button>
        </div>
      </header>

      {settingsOpen ? (
        <SettingsDialog
          emailConfigs={emailNotificationConfigs}
          state={queryState(emailNotificationsQuery)}
          creatingEmail={createEmailNotificationMutation.isPending}
          updatingEmail={updateEmailNotificationMutation.isPending}
          deletingEmail={deleteEmailNotificationMutation.isPending}
          onCreateEmail={(input) => createEmailNotificationMutation.mutate(input)}
          onUpdateEmail={(id, input) => updateEmailNotificationMutation.mutate({ id, body: input })}
          onDeleteEmail={(id) => deleteEmailNotificationMutation.mutate(id)}
          onClose={() => setSettingsOpen(false)}
        />
      ) : null}

      {notice ? (
        <Toast tone={notice.tone} onDismiss={() => setNotice(null)}>
          {notice.message}
        </Toast>
      ) : null}

      <main
        id="main-workbench"
        className={`workbenchGrid ${navCollapsed ? "navCollapsed" : ""} ${taskCollapsed ? "taskCollapsed" : ""}`}
        style={
          {
            "--nav-panel-width": `${navPanelWidth}px`,
            "--task-panel-width": `${taskPanelWidth}px`,
          } as CSSProperties
        }
      >
        <NavPanel
          servers={servers}
          projects={projects}
          selectedServerId={selectedServerId}
          selectedProjectId={selectedProjectId}
          collapsed={navCollapsed}
          serversState={queryState(serversQuery)}
          projectsState={queryState(projectsQuery)}
          onToggleCollapsed={() => setNavCollapsed((value) => !value)}
          onSelectServer={(serverId) => {
            setSelectedServerId(serverId);
            setSelectedProjectId(null);
            setSelectedTaskId(null);
            setSelectedRunId(null);
          }}
          onSelectProject={(projectId) => {
            setSelectedProjectId(projectId);
            setSelectedTaskId(null);
            setSelectedRunId(null);
          }}
          onCreateServer={(input) => createServerMutation.mutate(input)}
          creatingServer={createServerMutation.isPending}
          onUpdateServerAlias={(serverId, alias) => updateServerAliasMutation.mutate({ serverId, alias })}
          updatingServerAlias={updateServerAliasMutation.isPending}
          onDeleteServer={(serverId) => deleteServerMutation.mutate(serverId)}
          deletingServer={deleteServerMutation.isPending}
          onUpdateAllRunners={() => updateAllRunnersMutation.mutate()}
          updatingAllRunners={updateAllRunnersMutation.isPending}
          onCreateProject={(input) => createProjectMutation.mutate(input)}
          creatingProject={createProjectMutation.isPending}
          onDeleteProject={(projectId) => deleteProjectMutation.mutate(projectId)}
          deletingProject={deleteProjectMutation.isPending}
        />

        <ResizeHandle
          label="Resize navigation sidebar"
          hidden={navCollapsed}
          min={navPanelMinWidth}
          max={navPanelMaxWidth}
          value={navPanelWidth}
          onChange={setNavPanelWidth}
        />

        <TaskListPanel
          project={selectedProject}
          tasks={tasks}
          selectedTaskId={selectedTaskId}
          collapsed={taskCollapsed}
          state={queryState(tasksQuery)}
          onToggleCollapsed={() => setTaskCollapsed((value) => !value)}
          onSelectTask={(taskId) => {
            setSelectedTaskId(taskId);
            setSelectedRunId(null);
          }}
          onCreateTask={(title, description) => createTaskMutation.mutate({ title, description })}
          creating={createTaskMutation.isPending}
        />

        <ResizeHandle
          label="Resize tasks panel"
          hidden={taskCollapsed}
          min={taskPanelMinWidth}
          max={taskPanelMaxWidth}
          value={taskPanelWidth}
          onChange={setTaskPanelWidth}
        />

        <TaskSessionPanel
          server={projectServer}
          project={selectedProject}
          task={selectedTask ?? null}
          taskLoading={taskQuery.isLoading}
          runs={runs}
          runsState={queryState(runsQuery)}
          activeRun={activeRun}
          selectedRun={selectedRun}
          selectedRunId={selectedRunId}
          onSelectRun={setSelectedRunId}
          contextItems={contextItems}
          contextState={queryState(contextQuery)}
          onCreateContext={(input) => createContextMutation.mutate(input)}
          onUpdateContext={(id, body) => updateContextMutation.mutate({ id, body })}
          onDeleteContext={(id) => deleteContextMutation.mutate(id)}
          creatingContext={createContextMutation.isPending}
          updatingContext={updateContextMutation.isPending}
          deletingContext={deleteContextMutation.isPending}
          onCreateRun={(input) => createRunMutation.mutate(input)}
          creatingRun={createRunMutation.isPending || interruptRunMutation.isPending}
          onInterruptRun={(input) => interruptRunMutation.mutate(input)}
          interruptingRun={interruptRunMutation.isPending}
          onNotice={setNotice}
          onCancelRun={(runId) => cancelRunMutation.mutate(runId)}
          cancelingRun={cancelRunMutation.isPending}
          onDraftMemory={(input) => createRunMutation.mutateAsync(input)}
          onMarkDone={(input) => markDoneMutation.mutate(input)}
          markingDone={markDoneMutation.isPending}
        />
      </main>
    </div>
  );
}


function getBrowserNotificationPermission(): NotificationPermission {
  if (typeof window === "undefined" || !("Notification" in window)) {
    return "denied";
  }
  return window.Notification.permission;
}

function readStoredWorkbenchLocation(): StoredWorkbenchLocation {
  try {
    const raw = window.localStorage.getItem(workbenchLocationStorageKey);
    if (!raw) {
      return emptyWorkbenchLocation;
    }
    const parsed = JSON.parse(raw) as Partial<StoredWorkbenchLocation>;
    return {
      serverId: typeof parsed.serverId === "string" ? parsed.serverId : null,
      projectId: typeof parsed.projectId === "string" ? parsed.projectId : null,
      taskId: typeof parsed.taskId === "string" ? parsed.taskId : null,
      runId: typeof parsed.runId === "string" ? parsed.runId : null,
    };
  } catch {
    return emptyWorkbenchLocation;
  }
}

function writeStoredWorkbenchLocation(location: StoredWorkbenchLocation) {
  try {
    if (!location.serverId && !location.projectId && !location.taskId && !location.runId) {
      window.localStorage.removeItem(workbenchLocationStorageKey);
      return;
    }
    window.localStorage.setItem(workbenchLocationStorageKey, JSON.stringify(location));
  } catch {
    // Local storage can be unavailable in private or restricted browser modes.
  }
}

function getLatestRun(runs: Run[]) {
  if (runs.length === 0) {
    return null;
  }

  return runs.reduce((latest, run) => {
    const latestTime = Date.parse(latest.created_at);
    const runTime = Date.parse(run.created_at);
    if (Number.isNaN(latestTime) || Number.isNaN(runTime)) {
      return run;
    }
    return runTime >= latestTime ? run : latest;
  });
}

function sortRunsByCreatedAt(runs: Run[]) {
  return [...runs].sort((left, right) => {
    const leftTime = Date.parse(left.created_at);
    const rightTime = Date.parse(right.created_at);
    if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) {
      return 0;
    }
    return leftTime - rightTime;
  });
}
