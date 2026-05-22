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
import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../../api";
import type { CreateServerRequest, DirectoryEntry, Project, RunnerUpdateProgress, Server } from "../../types";
import { runnerCapabilitySummary } from "../../shared/runnerCapabilities";
import { serverDisplayName } from "../../shared/serverDisplay";
import { useI18n } from "../../shared/i18n";
import type { LoadState } from "../../shared/loadState";
import { EmptyState, ErrorState, Fact, InlineNotice, LoadBoundary, LoadingState, PanelHeader, StatusBadge } from "../../shared/ui";

export function NavPanel(props: {
  servers: Server[];
  projects: Project[];
  selectedServerId: string | null;
  selectedProjectId: string | null;
  collapsed: boolean;
  serversState: LoadState;
  projectsState: LoadState;
  onToggleCollapsed: () => void;
  onSelectServer: (serverId: string) => void;
  onSelectProject: (projectId: string) => void;
  onCreateServer: (input: CreateServerRequest) => void;
  creatingServer: boolean;
  onUpdateServerAlias: (serverId: string, alias: string) => void;
  updatingServerAlias: boolean;
  onDeleteServer: (serverId: string) => void;
  deletingServer: boolean;
  onUpdateAllRunners: () => void;
  updatingAllRunners: boolean;
  runnerUpdateProgress: RunnerUpdateProgress | null;
  onOpenRunnerUpdateProgress: () => void;
  onCreateProject: (input: {
    server_id: string;
    name: string;
    workdir: string;
    default_branch: string;
    rules_path: string;
  }) => void;
  creatingProject: boolean;
  onDeleteProject: (projectId: string) => void;
  deletingProject: boolean;
}) {
  const { t } = useI18n();
  const [isCreatingServer, setIsCreatingServer] = useState(false);
  const [serverName, setServerName] = useState("");
  const [serverAlias, setServerAlias] = useState("");
  const [runnerId, setRunnerId] = useState("");
  const [isCreatingProject, setIsCreatingProject] = useState(false);
  const [expandedServerId, setExpandedServerId] = useState<string | null>(null);
  const selectedServer = props.servers.find((server) => server.id === props.selectedServerId) ?? null;
  const selectedProject = props.projects.find((project) => project.id === props.selectedProjectId) ?? null;

  const submitServer = (event: FormEvent) => {
    event.preventDefault();
    if (!serverName.trim() || !runnerId.trim()) {
      return;
    }
    props.onCreateServer({
      name: serverName.trim(),
      alias: serverAlias.trim(),
      runner_id: runnerId.trim(),
    });
    setServerName("");
    setServerAlias("");
    setRunnerId("");
    setIsCreatingServer(false);
  };

  const selectedServerName = serverDisplayName(selectedServer);

  return (
    <aside className={`navPanel ${props.collapsed ? "isCollapsed" : ""}`} aria-label={t("nav.aria")}>
      {props.collapsed ? (
        <div className="collapsedPanelControl">
          <button
            className="iconButton"
            type="button"
            onClick={props.onToggleCollapsed}
            aria-label={t("nav.expand")}
            title={t("nav.expand")}
            aria-expanded={!props.collapsed}
          >
            <PanelLeftOpen size={16} />
          </button>
        </div>
      ) : null}
      {props.collapsed ? (
        <div className="collapsedNavSummary" aria-label="Collapsed navigation summary">
          <button
            className={`collapsedNavButton ${selectedServer ? "hasSelection" : ""}`}
            type="button"
            onClick={props.onToggleCollapsed}
            aria-label={selectedServer ? t("nav.selectedServer", { name: selectedServerName }) : t("nav.expand")}
            title={selectedServer ? t("nav.selectedServer", { name: selectedServerName }) : t("nav.serversCollapsed")}
          >
            <ServerIcon size={18} />
          </button>
          <button
            className={`collapsedNavButton ${selectedProject ? "hasSelection" : ""}`}
            type="button"
            onClick={props.onToggleCollapsed}
            aria-label={selectedProject ? t("nav.selectedProject", { name: selectedProject.name }) : t("nav.expand")}
            title={selectedProject ? t("nav.selectedProject", { name: selectedProject.name }) : t("nav.projectsCollapsed")}
          >
            <FolderKanban size={18} />
          </button>
        </div>
      ) : null}
      <div className="navPanelContent">
        <PanelHeader
          icon={<ServerIcon size={16} />}
          title={t("nav.servers")}
          detail={t("nav.configured", { count: props.servers.length })}
          action={
            <div className="panelActionGroup">
              <button
                className="ghostButton compact visibleUpdateButton"
                type="button"
                onClick={() => {
                  if (window.confirm(t("nav.updateConfirm"))) {
                    props.onUpdateAllRunners();
                  }
                }}
                disabled={props.updatingAllRunners || props.servers.length === 0}
                aria-label={t("nav.updateAll")}
                title={t("nav.updateAll")}
              >
                {props.updatingAllRunners ? <Loader2 className="spin" size={16} /> : <Download size={16} />}
                <span>{t("nav.updateAllShort")}</span>
              </button>
              <button
                className={`iconButton ${props.runnerUpdateProgress?.active ? "isActive" : ""}`}
                type="button"
                onClick={props.onOpenRunnerUpdateProgress}
                aria-label={t("nav.updateProgress")}
                title={t("nav.updateProgress")}
              >
                <History size={16} />
              </button>
              <button
                className="iconButton"
                type="button"
                onClick={() => setIsCreatingServer((value) => !value)}
                aria-label={t("nav.createServer")}
                title={t("nav.createServer")}
              >
                {isCreatingServer ? <X size={16} /> : <Plus size={16} />}
              </button>
              <button
                className="iconButton"
                type="button"
                onClick={props.onToggleCollapsed}
                aria-label={t("nav.collapse")}
                title={t("nav.collapse")}
                aria-expanded={!props.collapsed}
              >
                <PanelLeftClose size={16} />
              </button>
            </div>
          }
        />
        {isCreatingServer ? (
          <form className="compactForm" onSubmit={submitServer}>
            <label htmlFor="server-name">{t("nav.serverName")}</label>
            <input
              id="server-name"
              value={serverName}
              onChange={(event) => setServerName(event.target.value)}
              placeholder="desktop"
              disabled={props.creatingServer}
              required
            />
            <label htmlFor="server-alias">{t("nav.serverAlias")}</label>
            <input
              id="server-alias"
              value={serverAlias}
              onChange={(event) => setServerAlias(event.target.value)}
              placeholder={t("nav.serverAliasPlaceholder")}
              disabled={props.creatingServer}
            />
            <label htmlFor="server-runner-id">{t("nav.runnerId")}</label>
            <input
              id="server-runner-id"
              value={runnerId}
              onChange={(event) => setRunnerId(event.target.value)}
              placeholder="runner_desktop"
              disabled={props.creatingServer}
              required
            />
            <button
              className="primaryButton"
              type="submit"
              disabled={props.creatingServer || !serverName.trim() || !runnerId.trim()}
            >
              {props.creatingServer ? <Loader2 className="spin" size={16} /> : <Save size={16} />}
              {t("nav.createServer")}
            </button>
          </form>
        ) : null}
        <LoadBoundary
          state={props.serversState}
          empty={props.servers.length === 0}
          emptyTitle={t("nav.noServers")}
          emptyBody={t("nav.noServersBody")}
        >
          <div className="listStack">
            {props.servers.map((server) => {
              const displayName = serverDisplayName(server);
              return (
              <div
                key={server.id}
                className={`serverNavItem ${props.selectedServerId === server.id ? "isSelected" : ""}`}
              >
                <div className="serverCompactRow">
                  <button
                    className="serverSelectButton"
                    type="button"
                    onClick={() => props.onSelectServer(server.id)}
                  >
                    <span className="itemTitle">{displayName}</span>
                    <StatusBadge status={server.status} />
                  </button>
                  <button
                    className={`iconButton compact ${expandedServerId === server.id ? "isActive" : ""}`}
                    type="button"
                    onClick={() => setExpandedServerId((current) => (current === server.id ? null : server.id))}
                    aria-label={
                      expandedServerId === server.id
                        ? t("nav.collapseServer", { name: displayName })
                        : t("nav.manageServer", { name: displayName })
                    }
                    title={
                      expandedServerId === server.id
                        ? t("nav.collapseServer", { name: displayName })
                        : t("nav.manageServer", { name: displayName })
                    }
                    aria-expanded={expandedServerId === server.id}
                  >
                    <SettingsIcon size={14} />
                  </button>
                </div>
                {expandedServerId === server.id ? (
                  <div className="serverManagePanel">
                    <ServerAliasForm
                      server={server}
                      disabled={props.updatingServerAlias}
                      onUpdate={props.onUpdateServerAlias}
                    />
                    <div className="serverManageFacts">
                      <Fact label={t("nav.registeredName")} value={server.name} />
                      <Fact label={t("nav.runner")} value={server.runner_id} mono />
                      <Fact label={t("nav.connection")} value={server.runner_connected ? t("nav.connected") : t("nav.heartbeat")} />
                      <Fact label={t("nav.capabilities")} value={runnerCapabilitySummary(server)} />
                      <Fact
                        label={t("nav.runnerVersion")}
                        value={
                          server.runner_connected
                            ? `${server.runner_connection?.hostname || t("nav.connectedRunner")} / v${server.runner_connection?.version || "unknown"}`
                            : t("nav.noWebsocket")
                        }
                      />
                    </div>
                    <button
                      className="dangerButton compact"
                      type="button"
                      disabled={props.deletingServer}
                      onClick={() => {
                        if (window.confirm(t("nav.deleteServerConfirm", { name: displayName }))) {
                          props.onDeleteServer(server.id);
                        }
                      }}
                    >
                      {props.deletingServer ? <Loader2 className="spin" size={14} /> : <Trash2 size={14} />}
                      {t("nav.deleteServer")}
                    </button>
                  </div>
                ) : null}
              </div>
              );
            })}
          </div>
        </LoadBoundary>

        <section className="childNavSection" aria-label={t("nav.projects")}>
          <PanelHeader
            icon={<FolderKanban size={16} />}
            title={selectedServer ? t("nav.projectsOn", { name: selectedServerName }) : t("nav.projects")}
            detail={selectedServer ? t("nav.configured", { count: props.projects.length }) : t("nav.selectServer")}
            action={
              <button
                className="iconButton"
                type="button"
                onClick={() => setIsCreatingProject(true)}
                disabled={!selectedServer}
                aria-label={t("nav.createProject")}
                title={t("nav.createProject")}
              >
                <Plus size={16} />
              </button>
            }
          />
          {!selectedServer ? (
            <EmptyState icon={<FolderKanban size={22} />} title={t("nav.selectServerTitle")} body={t("nav.selectServerBody")} />
          ) : (
            <LoadBoundary
              state={props.projectsState}
              empty={props.projects.length === 0}
              emptyTitle={t("nav.noProjects")}
              emptyBody={t("nav.noProjectsBody")}
            >
              <div className="listStack">
                {props.projects.map((project) => (
                  <div
                    key={project.id}
                    className={`projectNavItem ${props.selectedProjectId === project.id ? "isSelected" : ""}`}
                  >
                    <button
                      className="projectSelectButton"
                      type="button"
                      onClick={() => props.onSelectProject(project.id)}
                    >
                      <span className="itemTitle">{project.name}</span>
                      <span className="itemSub mono">{project.workdir}</span>
                      <span className="metaLine">{project.default_branch || t("nav.branchUnknown")}</span>
                    </button>
                    <button
                      className="iconButton compact danger projectDeleteButton"
                      type="button"
                      disabled={props.deletingProject}
                      onClick={() => {
                        if (window.confirm(t("nav.deleteProjectConfirm", { name: project.name }))) {
                          props.onDeleteProject(project.id);
                        }
                      }}
                      aria-label={t("nav.deleteProject")}
                      title={t("nav.deleteProject")}
                    >
                      {props.deletingProject ? <Loader2 className="spin" size={13} /> : <Trash2 size={13} />}
                    </button>
                  </div>
                ))}
              </div>
            </LoadBoundary>
          )}
        </section>
      </div>
      {isCreatingProject && selectedServer ? (
        <ProjectCreateDialog
          server={selectedServer}
          creating={props.creatingProject}
          onCreate={(input) => {
            props.onCreateProject(input);
            setIsCreatingProject(false);
          }}
          onClose={() => setIsCreatingProject(false)}
        />
      ) : null}
    </aside>
  );
}

function ServerAliasForm(props: {
  server: Server;
  disabled: boolean;
  onUpdate: (serverId: string, alias: string) => void;
}) {
  const { t } = useI18n();
  const [aliasDraft, setAliasDraft] = useState(props.server.alias ?? "");

  useEffect(() => {
    setAliasDraft(props.server.alias ?? "");
  }, [props.server.alias, props.server.id]);

  const trimmedAlias = aliasDraft.trim();
  const currentAlias = props.server.alias ?? "";
  const changed = trimmedAlias !== currentAlias;

  const submitAlias = (event: FormEvent) => {
    event.preventDefault();
    if (!changed) {
      return;
    }
    props.onUpdate(props.server.id, trimmedAlias);
  };

  return (
    <form className="serverAliasForm" onSubmit={submitAlias}>
      <label htmlFor={`server-alias-${props.server.id}`}>{t("nav.serverAlias")}</label>
      <div className="serverAliasRow">
        <input
          id={`server-alias-${props.server.id}`}
          value={aliasDraft}
          onChange={(event) => setAliasDraft(event.target.value)}
          placeholder={props.server.name}
          disabled={props.disabled}
        />
        <button
          className="iconButton compact"
          type="submit"
          disabled={props.disabled || !changed}
          aria-label={t("nav.saveAlias")}
          title={t("nav.saveAlias")}
        >
          {props.disabled ? <Loader2 className="spin" size={14} /> : <Save size={14} />}
        </button>
      </div>
    </form>
  );
}


function ProjectCreateDialog(props: {
  server: Server;
  creating: boolean;
  onCreate: (input: {
    server_id: string;
    name: string;
    workdir: string;
    default_branch: string;
    rules_path: string;
  }) => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const displayName = serverDisplayName(props.server);
  const [projectName, setProjectName] = useState("");
  const [workdir, setWorkdir] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("main");
  const [rulesPath, setRulesPath] = useState("AGENTS.md");
  const [browsePath, setBrowsePath] = useState("");
  const [manualBrowsePath, setManualBrowsePath] = useState("");
  const canBrowseDirectories = Boolean(props.server.runner_connected && props.server.runner_capabilities?.fs_list === true);
  const [browseEnabled, setBrowseEnabled] = useState(canBrowseDirectories);

  const directoriesQuery = useQuery({
    queryKey: ["server-directories", props.server.id, browsePath],
    queryFn: () => api.listServerDirectories(props.server.id, browsePath),
    enabled: browseEnabled && canBrowseDirectories,
    retry: false,
  });

  useEffect(() => {
    setBrowseEnabled(canBrowseDirectories);
  }, [canBrowseDirectories]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        props.onClose();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [props]);

  const submitProject = (event: FormEvent) => {
    event.preventDefault();
    if (!projectName.trim() || !workdir.trim()) {
      return;
    }
    props.onCreate({
      server_id: props.server.id,
      name: projectName.trim(),
      workdir: workdir.trim(),
      default_branch: defaultBranch.trim(),
      rules_path: rulesPath.trim(),
    });
  };

  const listing = directoriesQuery.data ?? null;
  const canUseCurrentDirectory = Boolean(listing?.path);

  return (
    <div className="modalScrim" role="presentation" onMouseDown={props.onClose}>
      <section
        className="projectDialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="project-dialog-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="dialogHeader">
          <div className="panelTitle">
            <span className="panelIcon" aria-hidden="true">
              <FolderKanban size={16} />
            </span>
            <div>
              <h2 id="project-dialog-title">{t("nav.addProject")}</h2>
              <p>{t("nav.serverPrefix", { name: displayName })}</p>
            </div>
          </div>
          <button className="iconButton" type="button" onClick={props.onClose} aria-label={t("nav.closeProjectDialog")} title={t("nav.closeProjectDialog")}>
            <X size={16} />
          </button>
        </div>

        <form className="projectDialogBody" onSubmit={submitProject}>
          <div className="projectFormColumn">
            <label htmlFor="project-name">{t("nav.projectName")}</label>
            <input
              id="project-name"
              value={projectName}
              onChange={(event) => setProjectName(event.target.value)}
              placeholder="codex-task-workbench"
              disabled={props.creating}
              required
            />
            <label htmlFor="project-workdir">{t("nav.workdir")}</label>
            <div className="pathInputRow">
              <input
                id="project-workdir"
                className="mono"
                value={workdir}
                onChange={(event) => setWorkdir(event.target.value)}
                placeholder="D:\\go\\workplace"
                disabled={props.creating}
                required
              />
              <button
                className="iconButton"
                type="button"
                onClick={() => {
                  setBrowsePath(workdir.trim());
                  setBrowseEnabled(true);
                }}
                disabled={!canBrowseDirectories}
                aria-label={t("nav.browseWorkdir")}
                title={t("nav.browseWorkdir")}
              >
                <FolderOpen size={16} />
              </button>
            </div>
            <div className="formGridTwo">
              <label htmlFor="project-default-branch">
                {t("nav.defaultBranch")}
                <input
                  id="project-default-branch"
                  value={defaultBranch}
                  onChange={(event) => setDefaultBranch(event.target.value)}
                  placeholder="main"
                  disabled={props.creating}
                />
              </label>
              <label htmlFor="project-rules-path">
                {t("nav.rulesPath")}
                <input
                  id="project-rules-path"
                  value={rulesPath}
                  onChange={(event) => setRulesPath(event.target.value)}
                  placeholder="AGENTS.md"
                  disabled={props.creating}
                />
              </label>
            </div>
            <InlineNotice tone={canBrowseDirectories ? "info" : "danger"}>
              {canBrowseDirectories
                ? t("nav.browseUsesRunner")
                : props.server.runner_connected
                  ? t("nav.updateRunnerToBrowse")
                  : t("nav.noRunnerConnected")}
            </InlineNotice>
          </div>

          <div className="directoryBrowser" aria-label={t("nav.directoryBrowser")}>
            <div className="directoryToolbar">
              <button
                className="iconButton"
                type="button"
                onClick={() => listing?.parent && setBrowsePath(listing.parent)}
                disabled={!listing?.parent || directoriesQuery.isFetching}
                aria-label={t("nav.parentDirectory")}
                title={t("nav.parentDirectory")}
              >
                <ChevronLeft size={16} />
              </button>
              <span className="directoryPath mono">{listing?.path || t("nav.roots")}</span>
              <button
                className="primaryButton"
                type="button"
                onClick={() => listing?.path && setWorkdir(listing.path)}
                disabled={!canUseCurrentDirectory}
              >
                {t("nav.useFolder")}
              </button>
            </div>
            <div className="directoryPathJump">
              <input
                className="mono"
                value={manualBrowsePath}
                onChange={(event) => setManualBrowsePath(event.target.value)}
                placeholder={t("nav.jumpPath")}
                disabled={!canBrowseDirectories}
                aria-label={t("nav.jumpPath")}
              />
              <button
                className="ghostButton compact"
                type="button"
                onClick={() => {
                  setBrowsePath(manualBrowsePath.trim());
                  setBrowseEnabled(true);
                }}
                disabled={!canBrowseDirectories}
              >
                {t("nav.browse")}
              </button>
            </div>
            {directoriesQuery.isLoading || directoriesQuery.isFetching ? (
              <LoadingState label={t("nav.loadingDirectories")} />
            ) : directoriesQuery.isError ? (
              <ErrorState error={directoriesQuery.error} />
            ) : listing ? (
              <div className="directoryList">
                {listing.entries.length === 0 ? (
                  <EmptyState icon={<FolderOpen size={22} />} title={t("nav.noChildDirectories")} body={t("nav.noChildDirectoriesBody")} />
                ) : (
                  listing.entries.map((entry) => (
                  <DirectoryEntryRow key={entry.path} entry={entry} onOpen={() => setBrowsePath(entry.path)} onUse={() => setWorkdir(entry.path)} />
                  ))
                )}
              </div>
            ) : (
              <EmptyState icon={<FolderOpen size={22} />} title={t("nav.directoryBrowser")} body={t("nav.directoryBrowserBody")} />
            )}
          </div>

          <div className="dialogActions">
            <button className="ghostButton" type="button" onClick={props.onClose}>
              {t("nav.cancel")}
            </button>
            <button className="primaryButton" type="submit" disabled={props.creating || !projectName.trim() || !workdir.trim()}>
              {props.creating ? <Loader2 className="spin" size={16} /> : <Save size={16} />}
              {t("nav.createProject")}
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}


function DirectoryEntryRow(props: { entry: DirectoryEntry; onOpen: () => void; onUse: () => void }) {
  const { t } = useI18n();

  return (
    <div className="directoryRow">
      <button className="directoryOpenButton" type="button" onClick={props.onOpen}>
        <FolderOpen size={16} />
        <span>
          <strong>{props.entry.name}</strong>
          <span className="mono">{props.entry.path}</span>
        </span>
      </button>
      <div className="directoryMeta">
        {(props.entry.markers ?? []).slice(0, 3).map((marker) => (
          <span key={marker}>{marker}</span>
        ))}
      </div>
      <button className="ghostButton compact" type="button" onClick={props.onUse}>
        {t("nav.useFolder")}
      </button>
    </div>
  );
}
