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
import { useMemo, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import type { ContextItem, ContextScope, ContextType, Project, Server, Task } from "../../types";
import { contextTypeLabel } from "../../utils";
import { contextTypes } from "../../shared/constants";
import { useI18n } from "../../shared/i18n";
import type { LoadState } from "../../shared/loadState";
import { LoadBoundary } from "../../shared/ui";

export function ContextPanel(props: {
  server: Server | null;
  project: Project;
  task: Task;
  contextItems: ContextItem[];
  state: LoadState;
  selectedIds: string[];
  onSelectionChange: (ids: string[]) => void;
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
  creating: boolean;
  updating: boolean;
  deleting: boolean;
  summaryPanel?: ReactNode;
}) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [activeView, setActiveView] = useState<"pick" | "summary">("pick");
  const [scopeFilter, setScopeFilter] = useState<ContextScope | "all">("all");
  const [isAdding, setIsAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [scope, setScope] = useState<ContextScope>("task");
  const [type, setType] = useState<ContextType>("note");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [tags, setTags] = useState("");

  const visibleItems = props.contextItems.filter((item) => {
    if (scopeFilter !== "all" && item.scope !== scopeFilter) {
      return false;
    }
    const needle = query.trim().toLowerCase();
    if (!needle) {
      return true;
    }
    return `${item.title} ${item.content} ${item.type} ${item.scope} ${item.tags.join(" ")}`
      .toLowerCase()
      .includes(needle);
  });

  const toggleItem = (id: string) => {
    props.onSelectionChange(
      props.selectedIds.includes(id)
        ? props.selectedIds.filter((itemId) => itemId !== id)
        : [...props.selectedIds, id],
    );
  };

  const scopeCounts = useMemo(() => {
    const counts: Record<ContextScope | "all", number> = {
      all: props.contextItems.length,
      global: 0,
      server: 0,
      project: 0,
      task: 0,
    };
    for (const item of props.contextItems) {
      counts[item.scope] += 1;
    }
    return counts;
  }, [props.contextItems]);

  const resetForm = () => {
    setEditingId(null);
    setTitle("");
    setContent("");
    setTags("");
    setScope("task");
    setType("note");
  };

  const startEdit = (item: ContextItem) => {
    setEditingId(item.id);
    setIsAdding(true);
    setScope(item.scope);
    setType(item.type);
    setTitle(item.title);
    setContent(item.content);
    setTags(item.tags.join(", "));
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!title.trim() || !content.trim()) {
      return;
    }
    const input = {
      server_id: scope === "server" ? props.project.server_id : scope === "project" || scope === "task" ? props.project.server_id : null,
      project_id: scope === "project" || scope === "task" ? props.project.id : null,
      scope,
      task_id: scope === "task" ? props.task.id : null,
      type,
      title: title.trim(),
      content: content.trim(),
      tags: tags
        .split(",")
        .map((tag) => tag.trim())
        .filter(Boolean),
    };

    if (editingId) {
      props.onUpdateContext(editingId, input);
    } else {
      props.onCreateContext(input);
    }
    resetForm();
    setIsAdding(false);
  };

  return (
    <section className="contextPanel" aria-label={t("context.aria")}>
      <div className="contextModeTabs" role="tablist" aria-label={t("context.aria")}>
        <button
          type="button"
          role="tab"
          aria-selected={activeView === "pick"}
          className={activeView === "pick" ? "isSelected" : ""}
          onClick={() => setActiveView("pick")}
        >
          <ClipboardList size={14} />
          <span>{t("context.pickTab")}</span>
          {props.selectedIds.length > 0 ? <strong>{props.selectedIds.length}</strong> : null}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeView === "summary"}
          className={activeView === "summary" ? "isSelected" : ""}
          onClick={() => setActiveView("summary")}
        >
          <CheckCircle2 size={14} />
          <span>{t("context.summaryTab")}</span>
        </button>
      </div>

      {activeView === "summary" ? (
        props.summaryPanel
      ) : (
        <>
          <div className="contextToolbar">
            <label className="searchBox" htmlFor="context-search">
              <Search size={15} />
              <span className="srOnly">{t("context.search")}</span>
              <input
                id="context-search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t("context.filterPlaceholder")}
              />
            </label>
            <label className="contextScopeFilter" htmlFor="context-scope-filter">
              <span className="srOnly">{t("context.filterScope")}</span>
              <select
                id="context-scope-filter"
                value={scopeFilter}
                onChange={(event) => setScopeFilter(event.target.value as ContextScope | "all")}
              >
                <option value="all">all ({scopeCounts.all})</option>
                <option value="global">global ({scopeCounts.global})</option>
                <option value="server">server ({scopeCounts.server})</option>
                <option value="project">project ({scopeCounts.project})</option>
                <option value="task">task ({scopeCounts.task})</option>
              </select>
            </label>
            <button
              type="button"
              className="iconButton"
              onClick={() => {
                resetForm();
                setIsAdding((value) => !value);
              }}
              aria-label={t("context.add")}
              title={t("context.add")}
            >
              {isAdding ? <ChevronDown size={16} /> : <Plus size={16} />}
            </button>
          </div>

          {isAdding ? (
            <form className="compactForm" onSubmit={submit}>
              <div className="formGridTwo">
                <label htmlFor="context-scope">
                  {t("context.scope")}
                  <select
                    id="context-scope"
                    value={scope}
                    onChange={(event) => setScope(event.target.value as ContextScope)}
                    disabled={props.creating}
                  >
                    <option value="global">global</option>
                    <option value="server">server</option>
                    <option value="task">task</option>
                    <option value="project">project</option>
                  </select>
                </label>
                <label htmlFor="context-type">
                  {t("context.type")}
                  <select
                    id="context-type"
                    value={type}
                    onChange={(event) => setType(event.target.value as ContextType)}
                    disabled={props.creating}
                  >
                    {contextTypes.map((itemType) => (
                      <option key={itemType} value={itemType}>
                        {contextTypeLabel(itemType)}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              <label htmlFor="context-title">{t("context.title")}</label>
              <input
                id="context-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                disabled={props.creating}
                required
              />
              <label htmlFor="context-content">{t("context.content")}</label>
              <textarea
                id="context-content"
                value={content}
                onChange={(event) => setContent(event.target.value)}
                disabled={props.creating}
                rows={4}
                required
              />
              <label htmlFor="context-tags">{t("context.tags")}</label>
              <input
                id="context-tags"
                value={tags}
                onChange={(event) => setTags(event.target.value)}
                disabled={props.creating}
                placeholder={t("context.tagsPlaceholder")}
              />
              <button
                className="primaryButton"
                type="submit"
                disabled={props.creating || props.updating || !title.trim() || !content.trim()}
              >
                {props.creating || props.updating ? <Loader2 className="spin" size={16} /> : <Save size={16} />}
                {editingId ? t("context.update") : t("context.save")}
              </button>
            </form>
          ) : null}

          <LoadBoundary
            state={props.state}
            empty={visibleItems.length === 0}
            emptyTitle={t("context.emptyTitle")}
            emptyBody={t("context.emptyBody")}
          >
            <div className="contextList">
              {visibleItems.map((item) => (
                <label key={item.id} className="contextItem">
                  <input
                    type="checkbox"
                    checked={props.selectedIds.includes(item.id)}
                    onChange={() => toggleItem(item.id)}
                  />
                  <span className="contextMain">
                    <span className="rowTop">
                      <span className="contextTitle" title={item.title}>
                        {item.title}
                      </span>
                      <span className="contextType">{contextTypeLabel(item.type)}</span>
                    </span>
                    <span className="contextContent" title={item.content}>
                      {item.content}
                    </span>
                    <span className="rowMeta">
                      <span>{contextScopeLabel(item, props.server, props.project, props.task)}</span>
                      <span>{item.tags.join(", ") || t("context.noTags")}</span>
                    </span>
                    <span className="contextActions">
                      <button
                        type="button"
                        onClick={(event) => {
                          event.preventDefault();
                          startEdit(item);
                        }}
                      >
                        {t("context.edit")}
                      </button>
                      <button
                        type="button"
                        className="dangerText"
                        disabled={props.deleting}
                        onClick={(event) => {
                          event.preventDefault();
                          props.onDeleteContext(item.id);
                        }}
                      >
                        {t("context.delete")}
                      </button>
                    </span>
                  </span>
                </label>
              ))}
            </div>
          </LoadBoundary>
        </>
      )}
    </section>
  );
}


function contextScopeLabel(item: ContextItem, server: Server | null, project: Project, task: Task) {
  if (item.scope === "global") {
    return "global";
  }
  if (item.scope === "server") {
    return `server: ${server?.name ?? item.server_id ?? "unknown"}`;
  }
  if (item.scope === "project") {
    return `project: ${project.name}`;
  }
  if (item.scope === "task") {
    return `task: ${task.title}`;
  }
  return item.scope;
}
