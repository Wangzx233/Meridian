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
import { useState } from "react";
import type { FormEvent } from "react";
import type { Project, Task } from "../../types";
import { formatDateTime, shortId } from "../../utils";
import { useI18n } from "../../shared/i18n";
import type { LoadState } from "../../shared/loadState";
import { EmptyState, LoadBoundary, PanelHeader, StatusBadge } from "../../shared/ui";

export function TaskListPanel(props: {
  project: Project | null;
  tasks: Task[];
  selectedTaskId: string | null;
  collapsed: boolean;
  state: LoadState;
  onToggleCollapsed: () => void;
  onSelectTask: (taskId: string) => void;
  onCreateTask: (title: string, description: string) => void;
  creating: boolean;
}) {
  const { t } = useI18n();
  const [isCreating, setIsCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const selectedTask = props.tasks.find((task) => task.id === props.selectedTaskId) ?? null;
  const taskCount = props.tasks.length;

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!title.trim() || !props.project) {
      return;
    }
    props.onCreateTask(title.trim(), description.trim());
    setTitle("");
    setDescription("");
    setIsCreating(false);
  };

  return (
    <aside className={`taskPanel ${props.collapsed ? "isCollapsed" : ""}`} aria-label={t("tasks.aria")}>
      {props.collapsed ? (
        <div className="collapsedPanelControl">
          <button
            className="iconButton"
            type="button"
            onClick={props.onToggleCollapsed}
            aria-label={t("tasks.expand")}
            title={t("tasks.expand")}
            aria-expanded={!props.collapsed}
            aria-controls="task-panel-content"
          >
            <PanelRightOpen size={16} />
          </button>
        </div>
      ) : null}

      {props.collapsed ? (
        <div className="collapsedTaskSummary" aria-label="Collapsed task summary">
          <button
            className={`collapsedTaskButton ${selectedTask ? "hasSelection" : ""}`}
            type="button"
            onClick={props.onToggleCollapsed}
            title={selectedTask ? selectedTask.title : taskCount > 0 ? t("tasks.activeCount", { count: taskCount }) : t("tasks.noActive")}
            aria-label={
              selectedTask
                ? t("tasks.selected", { title: selectedTask.title })
                : t("tasks.activeCount", { count: taskCount })
            }
          >
            <ClipboardList size={17} />
            {taskCount > 0 ? (
              <span className="collapsedCount" aria-hidden="true">
                {taskCount > 99 ? "99+" : taskCount}
              </span>
            ) : null}
          </button>
        </div>
      ) : (
        <div id="task-panel-content" className="taskPanelContent">
          <PanelHeader
            icon={<ClipboardList size={16} />}
            title={t("tasks.title")}
            detail={props.project ? props.project.name : t("tasks.selectProject")}
            action={
              <div className="panelActionGroup">
                <button
                  className="iconButton"
                  type="button"
                  onClick={() => setIsCreating((value) => !value)}
                  disabled={!props.project}
                  aria-label={t("tasks.create")}
                  title={t("tasks.create")}
                >
                  {isCreating ? <X size={16} /> : <Plus size={16} />}
                </button>
                <button
                  className="iconButton"
                  type="button"
                  onClick={props.onToggleCollapsed}
                  aria-label={t("tasks.collapse")}
                  title={t("tasks.collapse")}
                  aria-expanded={!props.collapsed}
                  aria-controls="task-panel-content"
                >
                  <PanelRightClose size={16} />
                </button>
              </div>
            }
          />

          {isCreating ? (
            <form className="compactForm" onSubmit={submit}>
              <label htmlFor="task-title">{t("tasks.taskTitle")}</label>
              <input
                id="task-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="Fix order export timeout"
                disabled={props.creating}
                required
              />
              <label htmlFor="task-description">{t("tasks.description")}</label>
              <textarea
                id="task-description"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder={t("tasks.descriptionPlaceholder")}
                disabled={props.creating}
                rows={4}
              />
              <button className="primaryButton" type="submit" disabled={!title.trim() || props.creating}>
                {props.creating ? <Loader2 className="spin" size={16} /> : <Save size={16} />}
                {t("tasks.create")}
              </button>
            </form>
          ) : null}

          <LoadBoundary
            state={props.state}
            empty={Boolean(props.project) && props.tasks.length === 0}
            emptyTitle={props.project ? t("tasks.noActive") : t("tasks.noProject")}
            emptyBody={props.project ? t("tasks.noActiveBody") : t("tasks.noProjectBody")}
          >
            <div className="taskList">
              {props.tasks.map((task) => (
                <button
                  key={task.id}
                  className={`taskRow ${props.selectedTaskId === task.id ? "isSelected" : ""}`}
                  type="button"
                  onClick={() => props.onSelectTask(task.id)}
                >
                  <span className="rowTop">
                    <span className="taskTitle">{task.title}</span>
                    <StatusBadge status={task.status} />
                  </span>
                  <span className="taskDescription">{task.description || t("tasks.noDescription")}</span>
                  <span className="rowMeta">
                    <span>{t("tasks.session")} {shortId(task.codex_session_id, 8)}</span>
                    <span>{formatDateTime(task.updated_at)}</span>
                  </span>
                </button>
              ))}
            </div>
          </LoadBoundary>
        </div>
      )}
    </aside>
  );
}
