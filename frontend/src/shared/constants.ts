import type { CodexReasoningEffort, ContextType, EmailTLSMode, TaskStatus } from "../types";

export const activeTaskStatuses: TaskStatus[] = ["open", "waiting_user", "running"];

export const productName = "Meridian";

export const productTagline = "Manage project work across servers.";

export const navPanelDefaultWidth = 226;

export const navPanelMinWidth = 210;

export const navPanelMaxWidth = 330;

export const taskPanelDefaultWidth = 286;

export const taskPanelMinWidth = 238;

export const taskPanelMaxWidth = 390;

export const sidePanelDefaultWidth = 318;

export const sidePanelMinWidth = 276;

export const sidePanelMaxWidth = 460;

export const composerDefaultHeight = 154;

export const composerMinHeight = 112;

export const composerMaxHeight = 300;

export const defaultControlUrl = (import.meta.env.VITE_CONTROL_URL ?? window.location.origin).replace(/\/$/, "");

export const runnerFeatureCapabilities = ["fs_list", "project_files", "project_file_io", "project_terminal"] as const;

export const modelOptions = ["", "gpt-5.5", "gpt-5.4", "gpt-5.3-codex", "gpt-5.4-mini"];

export const reasoningEffortOptions: CodexReasoningEffort[] = ["", "low", "medium", "high", "xhigh"];

export const emailTLSModes: EmailTLSMode[] = ["starttls", "tls", "none"];

export const contextTypes: ContextType[] = [
  "project_rule",
  "task_summary",
  "decision",
  "log_snippet",
  "verify_command",
  "file_hint",
  "note",
];


export type WorkbenchTab = "output" | "terminal" | "files";

export type SidePanelTab = "context" | "agents" | "runs" | "prompt";
