import type {
  ApiErrorResponse,
  AuthSession,
  ContextItem,
  CreateContextItemRequest,
  CreateProjectRequest,
  CreateRunRequest,
  CreateRunResponse,
  CreateServerRequest,
  CreateTaskRequest,
  DirectoryListing,
  EmailNotificationConfig,
  EmailNotificationConfigRequest,
  ListResponse,
  MarkDoneRequest,
  Project,
  ProjectFileActionResult,
  ProjectFileContent,
  ProjectCommandResult,
  ProjectFileListing,
  Run,
  RunEvent,
  RunnerUpdateAllResponse,
  Server,
  Task,
  TaskStatus,
  WorkbenchNotification,
} from "./types";

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? "/api/v1").replace(/\/$/, "");

export class ApiError extends Error {
  code: string;
  details?: Record<string, unknown>;
  status: number;

  constructor(status: number, message: string, code = "request_failed", details?: Record<string, unknown>) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export const apiBaseUrl = API_BASE;

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...init.headers,
    },
  });

  const contentType = response.headers.get("content-type") ?? "";
  const isJson = contentType.includes("application/json");
  const body = isJson ? ((await response.json()) as unknown) : await response.text();

  if (response.ok && response.status !== 204 && !isJson) {
    throw new ApiError(
      response.status,
      "Control plane API did not return JSON. Check that the frontend is pointed at /api/v1.",
      "invalid_api_response",
    );
  }

  if (!response.ok) {
    if (isJson && typeof body === "object" && body !== null && "error" in body) {
      const errorBody = body as ApiErrorResponse;
      if (response.status === 401 && !path.startsWith("/auth/")) {
        window.dispatchEvent(new CustomEvent("ctw:unauthorized"));
      }
      throw new ApiError(
        response.status,
        errorBody.error.message,
        errorBody.error.code,
        errorBody.error.details,
      );
    }

    throw new ApiError(
      response.status,
      response.status === 404
        ? "Control plane API endpoint was not found."
        : "Control plane request failed.",
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return body as T;
}

function normalizeList<T>(response: ListResponse<T>): ListResponse<T> {
  return {
    items: Array.isArray(response.items) ? response.items : [],
    next_cursor: response.next_cursor ?? null,
  };
}

function queryString(params: Record<string, string | number | null | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      search.set(key, String(value));
    }
  }
  const value = search.toString();
  return value ? `?${value}` : "";
}

export const api = {
  getAuthSession: () => request<AuthSession>("/auth/session"),
  login: (body: { username: string; password: string }) =>
    request<AuthSession>("/auth/login", { method: "POST", body: JSON.stringify(body) }),
  setupAuth: (body: { username: string; password: string }) =>
    request<AuthSession>("/auth/setup", { method: "POST", body: JSON.stringify(body) }),
  logout: () => request<AuthSession>("/auth/logout", { method: "POST" }),

  listServers: async () => normalizeList(await request<ListResponse<Server>>("/servers")),
  createServer: (body: CreateServerRequest) =>
    request<Server>("/servers", { method: "POST", body: JSON.stringify(body) }),
  getServer: (serverId: string) => request<Server>(`/servers/${encodeURIComponent(serverId)}`),
  patchServer: (serverId: string, body: Partial<CreateServerRequest>) =>
    request<Server>(`/servers/${encodeURIComponent(serverId)}`, { method: "PATCH", body: JSON.stringify(body) }),
  deleteServer: (serverId: string) =>
    request<void>(`/servers/${encodeURIComponent(serverId)}`, { method: "DELETE" }),
  listServerDirectories: (serverId: string, path?: string) =>
    request<DirectoryListing>(`/servers/${encodeURIComponent(serverId)}/directories${queryString({ path })}`),
  updateAllRunners: () =>
    request<RunnerUpdateAllResponse>("/runners/update-all", { method: "POST" }),

  listProjects: (serverId?: string) =>
    request<ListResponse<Project>>(`/projects${queryString({ server_id: serverId })}`).then(normalizeList),
  createProject: (body: CreateProjectRequest) =>
    request<Project>("/projects", { method: "POST", body: JSON.stringify(body) }),
  getProject: (projectId: string) => request<Project>(`/projects/${encodeURIComponent(projectId)}`),
  patchProject: (projectId: string, body: Partial<CreateProjectRequest>) =>
    request<Project>(`/projects/${encodeURIComponent(projectId)}`, { method: "PATCH", body: JSON.stringify(body) }),
  listProjectFiles: (projectId: string, path?: string) =>
    request<ProjectFileListing>(`/projects/${encodeURIComponent(projectId)}/files${queryString({ path })}`),
  readProjectFile: (projectId: string, path: string) =>
    request<ProjectFileContent>(
      `/projects/${encodeURIComponent(projectId)}/files/content${queryString({ path })}`,
    ),
  writeProjectFile: (projectId: string, body: { path: string; content: string; create_dirs?: boolean }) =>
    request<ProjectFileActionResult>(`/projects/${encodeURIComponent(projectId)}/files/content`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  projectFileAction: (
    projectId: string,
    body: { action: "create" | "rename" | "delete"; path: string; target_path?: string; is_dir?: boolean },
  ) =>
    request<ProjectFileActionResult>(`/projects/${encodeURIComponent(projectId)}/files/actions`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  runProjectCommand: (projectId: string, body: { command: string; timeout_secs?: number }) =>
    request<ProjectCommandResult>(`/projects/${encodeURIComponent(projectId)}/command`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  projectTerminalWebSocketUrl: (projectId: string) => {
    const base = new URL(API_BASE, window.location.origin);
    const path = `${base.pathname.replace(/\/$/, "")}/projects/${encodeURIComponent(projectId)}/terminal`;
    const url = new URL(path, base.origin);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    return url.toString();
  },

  listTasks: (projectId: string, statuses: TaskStatus[]) =>
    request<ListResponse<Task>>(
      `/projects/${encodeURIComponent(projectId)}/tasks${queryString({ status: statuses.join(",") })}`,
    ).then(normalizeList),
  createTask: (projectId: string, body: CreateTaskRequest) =>
    request<Task>(`/projects/${encodeURIComponent(projectId)}/tasks`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  getTask: (taskId: string) => request<Task>(`/tasks/${encodeURIComponent(taskId)}`),
  patchTask: (taskId: string, body: Partial<CreateTaskRequest>) =>
    request<Task>(`/tasks/${encodeURIComponent(taskId)}`, { method: "PATCH", body: JSON.stringify(body) }),
  markTaskDone: (taskId: string, body: MarkDoneRequest) =>
    request<Task>(`/tasks/${encodeURIComponent(taskId)}/mark-done`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  archiveTask: (taskId: string) =>
    request<Task>(`/tasks/${encodeURIComponent(taskId)}/archive`, { method: "POST" }),

  listContextItems: (projectId: string, taskId?: string) =>
    request<ListResponse<ContextItem>>(
      `/projects/${encodeURIComponent(projectId)}/context-items${queryString({ task_id: taskId })}`,
    ).then(normalizeList),
  createContextItem: (projectId: string, body: CreateContextItemRequest) =>
    request<ContextItem>(`/projects/${encodeURIComponent(projectId)}/context-items`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  getContextItem: (contextItemId: string) =>
    request<ContextItem>(`/context-items/${encodeURIComponent(contextItemId)}`),
  patchContextItem: (contextItemId: string, body: Partial<CreateContextItemRequest>) =>
    request<ContextItem>(`/context-items/${encodeURIComponent(contextItemId)}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  deleteContextItem: (contextItemId: string) =>
    request<void>(`/context-items/${encodeURIComponent(contextItemId)}`, { method: "DELETE" }),

  listEmailNotificationConfigs: () =>
    request<ListResponse<EmailNotificationConfig>>("/settings/email-notifications").then(normalizeList),
  createEmailNotificationConfig: (body: EmailNotificationConfigRequest) =>
    request<EmailNotificationConfig>("/settings/email-notifications", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  patchEmailNotificationConfig: (configId: string, body: Partial<EmailNotificationConfigRequest>) =>
    request<EmailNotificationConfig>(`/settings/email-notifications/${encodeURIComponent(configId)}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  deleteEmailNotificationConfig: (configId: string) =>
    request<void>(`/settings/email-notifications/${encodeURIComponent(configId)}`, { method: "DELETE" }),

  listWorkbenchNotifications: (pending = true) =>
    request<ListResponse<WorkbenchNotification>>(`/notifications${queryString({ pending: pending ? "true" : "false" })}`).then(normalizeList),
  acknowledgeWorkbenchNotification: (notificationId: string) =>
    request<WorkbenchNotification>(`/notifications/${encodeURIComponent(notificationId)}/ack`, { method: "POST" }),

  listRuns: (taskId: string) =>
    request<ListResponse<Run>>(`/tasks/${encodeURIComponent(taskId)}/runs`).then(normalizeList),
  createRun: (taskId: string, body: CreateRunRequest, idempotencyKey: string) =>
    request<CreateRunResponse>(`/tasks/${encodeURIComponent(taskId)}/runs`, {
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body: JSON.stringify(body),
    }),
  interruptRun: (taskId: string, body: CreateRunRequest, idempotencyKey: string) =>
    request<CreateRunResponse>(`/tasks/${encodeURIComponent(taskId)}/runs/interrupt`, {
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body: JSON.stringify(body),
    }),
  getRun: (runId: string) => request<Run>(`/runs/${encodeURIComponent(runId)}`),
  listRunEvents: (runId: string, afterSeq = 0) =>
    request<ListResponse<RunEvent>>(
      `/runs/${encodeURIComponent(runId)}/events${queryString({ after_seq: afterSeq })}`,
    ).then(normalizeList),
  streamRunEventsUrl: (runId: string, afterSeq = 0) =>
    `${API_BASE}/runs/${encodeURIComponent(runId)}/events/stream${queryString({ after_seq: afterSeq })}`,
  cancelRun: (runId: string, reason: string) =>
    request<Run>(`/runs/${encodeURIComponent(runId)}/cancel`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    }),
};
