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
  FileUp,
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
import Editor from "@monaco-editor/react";
import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, api } from "../../api";
import type { Project, ProjectFileContent, ProjectFileEntry, Server } from "../../types";
import { formatBytes } from "../../shared/format";
import { runnerCapabilityBlockedReason, runnerCapabilityPillLabel } from "../../shared/runnerCapabilities";
import { CapabilityPill, EmptyState, ErrorState, InlineNotice, LoadingState } from "../../shared/ui";
import { errorNotice } from "../../shared/notices";

const maxUploadBytes = 5 * 1024 * 1024;

type FileDialogState =
  | { action: "create_file" | "create_dir"; path: string }
  | { action: "rename"; path: string; targetPath: string }
  | { action: "delete"; path: string; isDir: boolean }
  | null;

export type ProjectFileOpenRequest = {
  id: number;
  path: string;
  isDir: boolean;
};

export function ProjectFilesPanel(props: { server: Server | null; project: Project; openRequest?: ProjectFileOpenRequest | null }) {
  const queryClient = useQueryClient();
  const [path, setPath] = useState("");
  const [manualPath, setManualPath] = useState("");
  const [selectedFilePath, setSelectedFilePath] = useState<string | null>(null);
  const [handledOpenRequestId, setHandledOpenRequestId] = useState<number | null>(null);
  const [editorValue, setEditorValue] = useState("");
  const [dirty, setDirty] = useState(false);
  const [dialog, setDialog] = useState<FileDialogState>(null);
  const [actionError, setActionError] = useState<unknown>(null);
  const [saveError, setSaveError] = useState<unknown>(null);
  const [uploadError, setUploadError] = useState<unknown>(null);
  const uploadInputRef = useRef<HTMLInputElement | null>(null);
  const canBrowse = Boolean(props.server?.runner_connected && props.server.runner_capabilities?.project_files === true);
  const canEdit = Boolean(props.server?.runner_connected && props.server.runner_capabilities?.project_file_io === true);
  const canUpload = Boolean(props.server?.runner_connected && props.server.runner_capabilities?.project_file_upload === true);
  const blockedReason = runnerCapabilityBlockedReason(props.server, canBrowse ? "project_file_io" : "project_files", "file browsing");
  const uploadBlockedReason = runnerCapabilityBlockedReason(props.server, "project_file_upload", "file upload");
  const filesQuery = useQuery({
    queryKey: ["project-files", props.project.id, path],
    queryFn: () => api.listProjectFiles(props.project.id, path),
    enabled: canBrowse,
    retry: false,
  });
  const fileQuery = useQuery({
    queryKey: ["project-file-content", props.project.id, selectedFilePath],
    queryFn: () => api.readProjectFile(props.project.id, selectedFilePath!),
    enabled: canEdit && Boolean(selectedFilePath),
    retry: false,
  });
  const saveMutation = useMutation({
    mutationFn: (body: { path: string; content: string; create_dirs?: boolean }) => api.writeProjectFile(props.project.id, body),
    onSuccess: () => {
      setDirty(false);
      setSaveError(null);
      void queryClient.invalidateQueries({ queryKey: ["project-files", props.project.id] });
      void queryClient.invalidateQueries({ queryKey: ["project-file-content", props.project.id, selectedFilePath] });
    },
    onError: (error) => setSaveError(error),
  });
  const uploadMutation = useMutation({
    mutationFn: (body: { path: string; file: File; create_dirs?: boolean }) => api.uploadProjectFile(props.project.id, body),
    onSuccess: (result) => {
      const parent = parentDirectory(result.path);
      setUploadError(null);
      setPath(parent);
      setManualPath(parent);
      void queryClient.invalidateQueries({ queryKey: ["project-files", props.project.id] });
      void queryClient.invalidateQueries({ queryKey: ["project-files", props.project.id, parent] });
      void queryClient.invalidateQueries({ queryKey: ["project-file-content", props.project.id, result.path] });
    },
    onError: (error) => setUploadError(error),
  });
  const actionMutation = useMutation({
    mutationFn: (body: { action: "create" | "rename" | "delete"; path: string; target_path?: string; is_dir?: boolean }) =>
      api.projectFileAction(props.project.id, body),
    onSuccess: (_result, variables) => {
      setDialog(null);
      setActionError(null);
      if (variables.action === "delete" && variables.path === selectedFilePath) {
        setSelectedFilePath(null);
      }
      if (variables.action === "rename" && variables.path === selectedFilePath && variables.target_path) {
        setSelectedFilePath(variables.target_path);
        setPath(parentDirectory(variables.target_path));
      }
      if (variables.action === "create") {
        setPath(parentDirectory(variables.path));
        if (!variables.is_dir) {
          setSelectedFilePath(variables.path);
        }
      }
      void queryClient.invalidateQueries({ queryKey: ["project-files", props.project.id] });
    },
    onError: (error) => setActionError(error),
  });
  const listing = filesQuery.data ?? null;
  const activeContent = fileQuery.data ?? null;

  useEffect(() => {
    setPath("");
    setManualPath("");
    setSelectedFilePath(null);
    setHandledOpenRequestId(null);
    setEditorValue("");
    setDirty(false);
    setActionError(null);
    setSaveError(null);
    setUploadError(null);
  }, [props.project.id]);

  useEffect(() => {
    const request = props.openRequest;
    if (!request || request.id === handledOpenRequestId) {
      return;
    }
    setHandledOpenRequestId(request.id);
    setActionError(null);
    setSaveError(null);
    setUploadError(null);
    if (request.isDir) {
      setPath(request.path);
      setManualPath(request.path);
      setSelectedFilePath(null);
      return;
    }
    const parent = parentDirectory(request.path);
    setPath(parent);
    setManualPath(parent);
    setSelectedFilePath(request.path);
  }, [handledOpenRequestId, props.openRequest]);

  useEffect(() => {
    if (activeContent) {
      setEditorValue(activeContent.content);
      setDirty(false);
    }
  }, [activeContent?.path, activeContent?.content]);

  const refreshFiles = () => {
    void filesQuery.refetch();
    if (selectedFilePath) {
      void fileQuery.refetch();
    }
  };

  const openFile = (entry: ProjectFileEntry) => {
    if (entry.is_dir) {
      setPath(entry.path);
      return;
    }
    setSelectedFilePath(entry.path);
  };

  const saveFile = () => {
    if (!selectedFilePath || !canEdit) {
      return;
    }
    saveMutation.mutate({ path: selectedFilePath, content: editorValue, create_dirs: true });
  };

  const uploadFile = (file: File | undefined) => {
    if (!file || !canUpload || uploadMutation.isPending) {
      return;
    }
    if (file.size > maxUploadBytes) {
      setUploadError(new ApiError(413, `File is too large. Upload files up to ${formatBytes(maxUploadBytes)}.`, "validation_error"));
      return;
    }
    setUploadError(null);
    uploadMutation.mutate({ path, file, create_dirs: true });
  };

  return (
    <section className="filesPanel" aria-label="Project files">
      <div className="toolHeader">
        <div>
          <h3>Files</h3>
          <p className="mono">{listing?.root ?? props.project.workdir}</p>
        </div>
        <div className="toolActions">
          <CapabilityPill
            available={canBrowse && canEdit && canUpload}
            label={canBrowse && canEdit && canUpload ? "Files ready" : runnerCapabilityPillLabel(props.server)}
          />
            <button className="ghostButton compact" type="button" onClick={refreshFiles} disabled={!canBrowse || filesQuery.isFetching}>
              <RefreshCw size={14} />
              Refresh
            </button>
        </div>
      </div>

      {!canBrowse || !canEdit ? <InlineNotice tone="danger">{blockedReason}</InlineNotice> : null}
      {canBrowse && canEdit && !canUpload ? <InlineNotice tone="danger">{uploadBlockedReason}</InlineNotice> : null}

      <div className="fileToolbar">
        <button
          className="iconButton"
          type="button"
          onClick={() => listing?.parent !== null && setPath(listing?.parent ?? "")}
          disabled={!canBrowse || !listing || listing.parent === null || filesQuery.isFetching}
          aria-label="Go to parent directory"
        >
          <ChevronLeft size={16} />
        </button>
        <span className="directoryPath mono">{listing?.path || "."}</span>
        <div className="fileToolbarActions">
          <button className="ghostButton compact" type="button" onClick={() => setDialog({ action: "create_file", path: path ? `${path}/` : "" })} disabled={!canEdit}>
            <FileText size={14} />
            New file
          </button>
          <button className="ghostButton compact" type="button" onClick={() => setDialog({ action: "create_dir", path: path ? `${path}/` : "" })} disabled={!canEdit}>
            <FolderOpen size={14} />
            New folder
          </button>
          <input
            ref={uploadInputRef}
            className="srOnly"
            type="file"
            onChange={(event) => {
              uploadFile(event.target.files?.[0]);
              event.target.value = "";
            }}
            disabled={!canUpload}
            aria-label="Upload file to current directory"
          />
          <button
            className="ghostButton compact"
            type="button"
            onClick={() => uploadInputRef.current?.click()}
            disabled={!canUpload || uploadMutation.isPending}
          >
            {uploadMutation.isPending ? <Loader2 className="spin" size={14} /> : <FileUp size={14} />}
            Upload
          </button>
        </div>
      </div>

      {uploadError ? <InlineNotice tone="danger">{errorNotice(uploadError, "Unable to upload file.").message}</InlineNotice> : null}

      <div className="directoryPathJump">
        <input
          className="mono"
          value={manualPath}
          onChange={(event) => setManualPath(event.target.value)}
          placeholder="Jump within project"
          disabled={!canBrowse}
          aria-label="Project path to browse"
        />
        <button className="ghostButton compact" type="button" onClick={() => setPath(manualPath.trim())} disabled={!canBrowse}>
          Browse
        </button>
      </div>

      <div className="fileWorkspace">
        <div className="projectFileList" role="tree" aria-label="Project file tree">
          {filesQuery.isLoading || filesQuery.isFetching ? (
            <LoadingState label="Loading files" />
          ) : filesQuery.isError ? (
            <ErrorState error={filesQuery.error} />
          ) : listing && listing.entries.length > 0 ? (
            listing.entries.map((entry) => (
              <ProjectFileRow
                key={entry.path}
                entry={entry}
                selected={entry.path === selectedFilePath}
                onOpen={() => openFile(entry)}
                onRename={() => setDialog({ action: "rename", path: entry.path, targetPath: entry.path })}
                onDelete={() => setDialog({ action: "delete", path: entry.path, isDir: entry.is_dir })}
              />
            ))
          ) : listing ? (
            <EmptyState icon={<FolderOpen size={22} />} title="Empty directory" body="No visible entries." />
          ) : (
            <EmptyState icon={<FolderOpen size={22} />} title="Project files" body="Connect a runner to browse files." />
          )}
        </div>

        <div className="fileEditorPane">
          <div className="fileEditorToolbar">
            <div>
              <h4>{selectedFilePath ?? "No file selected"}</h4>
              <p>{activeContent ? `${formatBytes(activeContent.size)} / ${activeContent.encoding}` : "Open a file to inspect or edit it."}</p>
            </div>
            <div className="toolActions">
              <button className="ghostButton compact" type="button" onClick={() => downloadFile(activeContent)} disabled={!activeContent}>
                <Download size={14} />
                Download
              </button>
              <button className="primaryButton compact" type="button" onClick={saveFile} disabled={!canEdit || !selectedFilePath || !dirty || saveMutation.isPending}>
                {saveMutation.isPending ? <Loader2 className="spin" size={14} /> : <Save size={14} />}
                Save
              </button>
            </div>
          </div>

          <div className="fileEditorBody">
            {fileQuery.isLoading ? (
              <LoadingState label="Loading file" />
            ) : fileQuery.isError ? (
              <ErrorState error={fileQuery.error} />
            ) : selectedFilePath ? (
              <Editor
                height="100%"
                language={languageForPath(selectedFilePath)}
                value={editorValue}
                theme="vs"
                options={{
                  minimap: { enabled: false },
                  fontSize: 13,
                  tabSize: 2,
                  wordWrap: "on",
                  scrollBeyondLastLine: false,
                  automaticLayout: true,
                }}
                onChange={(value) => {
                  setEditorValue(value ?? "");
                  setDirty(true);
                }}
              />
            ) : (
              <div className="workspaceEmpty">
                <EmptyState icon={<FileText size={22} />} title="No file selected" body="Choose a file." />
              </div>
            )}
            {saveError ? <InlineNotice tone="danger">{errorNotice(saveError, "Unable to save file.").message}</InlineNotice> : null}
          </div>
        </div>
      </div>

      {dialog ? (
        <FileActionDialog
          dialog={dialog}
          busy={actionMutation.isPending}
          error={actionError}
          onClose={() => {
            setDialog(null);
            setActionError(null);
          }}
          onSubmit={(next) => {
            if (next.action === "create_file" || next.action === "create_dir") {
              actionMutation.mutate({ action: "create", path: next.path, is_dir: next.action === "create_dir" });
              return;
            }
            if (next.action === "rename") {
              actionMutation.mutate({ action: "rename", path: next.path, target_path: next.targetPath });
              return;
            }
            actionMutation.mutate({ action: "delete", path: next.path });
          }}
        />
      ) : null}
    </section>
  );
}

function ProjectFileRow(props: {
  entry: ProjectFileEntry;
  selected: boolean;
  onOpen: () => void;
  onRename: () => void;
  onDelete: () => void;
}) {
  return (
    <div className={`projectFileRow ${props.selected ? "isSelected" : ""}`} role="treeitem" aria-selected={props.selected}>
      <button className="fileOpenButton" type="button" onClick={props.onOpen}>
        <span className="fileKind">{props.entry.is_dir ? <FolderOpen size={15} /> : <FileText size={15} />}</span>
        <span className="fileMain">
          <strong>{props.entry.name}</strong>
          <span className="mono">{props.entry.path}</span>
        </span>
        <span className="fileSize">{props.entry.is_dir ? "folder" : formatBytes(props.entry.size)}</span>
      </button>
      <div className="fileRowActions">
        <button className="iconButton compact" type="button" onClick={props.onRename} aria-label={`Rename ${props.entry.name}`}>
          <Edit3 size={13} />
        </button>
        <button className="iconButton compact danger" type="button" onClick={props.onDelete} aria-label={`Delete ${props.entry.name}`}>
          <Trash2 size={13} />
        </button>
      </div>
    </div>
  );
}

function FileActionDialog(props: {
  dialog: FileDialogState;
  busy: boolean;
  error: unknown;
  onClose: () => void;
  onSubmit: (dialog: Exclude<FileDialogState, null>) => void;
}) {
  const dialog = props.dialog;
  const [pathValue, setPathValue] = useState(dialog && "path" in dialog ? dialog.path : "");
  const [targetValue, setTargetValue] = useState(dialog?.action === "rename" ? dialog.targetPath : "");

  if (!dialog) {
    return null;
  }

  const title =
    dialog.action === "create_file"
      ? "New file"
      : dialog.action === "create_dir"
        ? "New folder"
        : dialog.action === "rename"
          ? "Rename"
          : "Delete";
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (dialog.action === "delete") {
      props.onSubmit(dialog);
      return;
    }
    if (dialog.action === "rename") {
      if (targetValue.trim()) {
        props.onSubmit({ ...dialog, targetPath: targetValue.trim() });
      }
      return;
    }
    if (pathValue.trim()) {
      props.onSubmit({ ...dialog, path: pathValue.trim() });
    }
  };

  return (
    <div className="modalScrim" role="presentation" onMouseDown={props.onClose}>
      <form className="fileDialog" onSubmit={submit} role="dialog" aria-modal="true" aria-label={title} onMouseDown={(event) => event.stopPropagation()}>
        <div className="boxHeader">
          <div>
            <h3>{title}</h3>
            <p className="mono">{dialog.path}</p>
          </div>
          <button className="iconButton" type="button" onClick={props.onClose} aria-label="Close file dialog">
            <X size={16} />
          </button>
        </div>
        {dialog.action === "delete" ? (
          <p className="dialogBodyText">Delete this {dialog.isDir ? "folder" : "file"} from the project directory?</p>
        ) : dialog.action === "rename" ? (
          <>
            <label htmlFor="file-target-path">New path</label>
            <input id="file-target-path" className="mono" value={targetValue} onChange={(event) => setTargetValue(event.target.value)} autoFocus />
          </>
        ) : (
          <>
            <label htmlFor="file-action-path">Path</label>
            <input id="file-action-path" className="mono" value={pathValue} onChange={(event) => setPathValue(event.target.value)} autoFocus />
          </>
        )}
        {props.error ? <InlineNotice tone="danger">{errorNotice(props.error, "File action failed.").message}</InlineNotice> : null}
        <div className="dialogActions">
          <button className="ghostButton" type="button" onClick={props.onClose} disabled={props.busy}>
            Cancel
          </button>
          <button className={dialog.action === "delete" ? "dangerButton" : "primaryButton"} type="submit" disabled={props.busy}>
            {props.busy ? <Loader2 className="spin" size={16} /> : dialog.action === "delete" ? <Trash2 size={16} /> : <Save size={16} />}
            {dialog.action === "delete" ? "Delete" : "Apply"}
          </button>
        </div>
      </form>
    </div>
  );
}

function languageForPath(path: string) {
  const ext = path.split(".").pop()?.toLowerCase();
  switch (ext) {
    case "go":
      return "go";
    case "ts":
    case "tsx":
      return "typescript";
    case "js":
    case "jsx":
      return "javascript";
    case "json":
      return "json";
    case "md":
      return "markdown";
    case "css":
      return "css";
    case "html":
      return "html";
    case "sql":
      return "sql";
    case "yaml":
    case "yml":
      return "yaml";
    case "ps1":
      return "powershell";
    case "sh":
      return "shell";
    case "py":
      return "python";
    default:
      return "plaintext";
  }
}

function downloadFile(file: ProjectFileContent | null) {
  if (!file) {
    return;
  }
  const blob = new Blob([file.content], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = file.name || file.path.split("/").pop() || "download.txt";
  link.click();
  URL.revokeObjectURL(url);
}

function parentDirectory(path: string) {
  const index = path.lastIndexOf("/");
  if (index <= 0) {
    return "";
  }
  return path.slice(0, index);
}
