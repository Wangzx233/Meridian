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
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../api";
import type { Project, Server } from "../../types";
import { runnerCapabilityBlockedReason, runnerCapabilityPillLabel } from "../../shared/runnerCapabilities";
import { CapabilityPill, InlineNotice, LoadingState } from "../../shared/ui";
import { errorNotice } from "../../shared/notices";

export function AgentsFilePanel(props: { server: Server | null; project: Project }) {
  const queryClient = useQueryClient();
  const [content, setContent] = useState("");
  const [dirty, setDirty] = useState(false);
  const [saveError, setSaveError] = useState<unknown>(null);
  const canRead = Boolean(props.server?.runner_connected && props.server.runner_capabilities?.project_file_io === true);
  const blockedReason = runnerCapabilityBlockedReason(props.server, "project_file_io", "AGENTS.md editing");
  const agentsPath = "AGENTS.md";
  const fileQuery = useQuery({
    queryKey: ["project-file-content", props.project.id, agentsPath],
    queryFn: () => api.readProjectFile(props.project.id, agentsPath),
    enabled: canRead,
    retry: false,
  });
  const saveMutation = useMutation({
    mutationFn: (body: { path: string; content: string; create_dirs?: boolean }) => api.writeProjectFile(props.project.id, body),
    onSuccess: () => {
      setDirty(false);
      setSaveError(null);
      void queryClient.invalidateQueries({ queryKey: ["project-file-content", props.project.id, agentsPath] });
      void queryClient.invalidateQueries({ queryKey: ["project-files", props.project.id] });
    },
    onError: (error) => setSaveError(error),
  });

  useEffect(() => {
    setContent("");
    setDirty(false);
    setSaveError(null);
  }, [props.project.id]);

  useEffect(() => {
    if (fileQuery.data) {
      setContent(fileQuery.data.content);
      setDirty(false);
    }
  }, [fileQuery.data?.path, fileQuery.data?.content]);

  const saveAgents = () => {
    if (!canRead || !dirty) {
      return;
    }
    saveMutation.mutate({ path: agentsPath, content, create_dirs: true });
  };

  return (
    <section className="agentsPanel" aria-label="Project AGENTS file">
      <div className="toolHeader">
        <div>
          <h3>AGENTS.md</h3>
          <p className="mono">{props.project.workdir}</p>
        </div>
        <div className="toolActions">
          <CapabilityPill available={canRead} label={canRead ? "Editable" : runnerCapabilityPillLabel(props.server)} />
          <button className="ghostButton compact" type="button" onClick={() => fileQuery.refetch()} disabled={!canRead || fileQuery.isFetching}>
            <RefreshCw size={14} />
            Refresh
          </button>
          <button className="primaryButton compact" type="button" onClick={saveAgents} disabled={!canRead || !dirty || saveMutation.isPending}>
            {saveMutation.isPending ? <Loader2 className="spin" size={14} /> : <Save size={14} />}
            Save
          </button>
        </div>
      </div>

      {!canRead ? <InlineNotice tone="danger">{blockedReason}</InlineNotice> : null}
      {fileQuery.isLoading ? (
        <LoadingState label="Loading AGENTS.md" />
      ) : fileQuery.isError ? (
        <InlineNotice tone="info">
          {errorNotice(fileQuery.error, "AGENTS.md was not found. Saving will create it in the project root.").message}
        </InlineNotice>
      ) : null}
      {saveError ? <InlineNotice tone="danger">{errorNotice(saveError, "Unable to save AGENTS.md.").message}</InlineNotice> : null}

      <textarea
        className="agentsEditor mono"
        value={content}
        onChange={(event) => {
          setContent(event.target.value);
          setDirty(true);
        }}
        disabled={!canRead || saveMutation.isPending}
        placeholder="# AGENTS.md"
        spellCheck={false}
      />
    </section>
  );
}
