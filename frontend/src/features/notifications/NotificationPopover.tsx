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
import { useEffect } from "react";
import type { WorkbenchNotification } from "../../types";
import { formatDateTime } from "../../utils";
import type { LoadState } from "../../shared/loadState";
import { LoadBoundary } from "../../shared/ui";

export function NotificationPopover(props: {
  notifications: WorkbenchNotification[];
  state: LoadState;
  acknowledging: boolean;
  browserPermission: NotificationPermission;
  onOpenNotification: (notification: WorkbenchNotification) => void;
  onAcknowledge: (notificationId: string) => void;
  onEnableBrowserNotifications: () => void;
  onClose: () => void;
}) {
  const browserSupported = typeof window !== "undefined" && "Notification" in window;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        props.onClose();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [props]);

  return (
    <div
      id="notification-popover"
      className="notificationPopover"
      role="dialog"
      aria-label="Pending run completion notifications"
    >
      <div className="popoverHeader">
        <div className="panelTitle">
          <span className="panelIcon" aria-hidden="true">
            <Bell size={16} />
          </span>
          <div>
            <h2>Pending notices</h2>
            <p>{props.notifications.length} awaiting confirmation</p>
          </div>
        </div>
        <button className="iconButton" type="button" onClick={props.onClose} aria-label="Close notifications">
          <X size={16} />
        </button>
      </div>

      {browserSupported && props.browserPermission !== "granted" ? (
        <button
          className="ghostButton compact notificationPermissionButton"
          type="button"
          onClick={props.onEnableBrowserNotifications}
          disabled={props.browserPermission === "denied"}
        >
          <Bell size={14} />
          {props.browserPermission === "denied" ? "Browser notifications blocked" : "Enable browser notifications"}
        </button>
      ) : null}

      <LoadBoundary
        state={props.state}
        empty={props.notifications.length === 0}
        emptyTitle="No pending notices"
        emptyBody="Finished turns you have not opened or dismissed will stay here."
      >
        <div className="notificationList">
          {props.notifications.map((notification) => (
            <article key={notification.id} className="notificationItem">
              <button
                className="notificationOpenButton"
                type="button"
                onClick={() => props.onOpenNotification(notification)}
              >
                <span className="notificationTitle">{notification.title}</span>
                <span className="notificationMessage">{notificationMessage(notification)}</span>
                <span className="metaLine">{formatDateTime(notification.created_at)}</span>
              </button>
              <button
                className="iconButton compact"
                type="button"
                onClick={() => props.onAcknowledge(notification.id)}
                disabled={props.acknowledging}
                aria-label={`Dismiss notification for ${notification.task_title}`}
                title="Dismiss"
              >
                {props.acknowledging ? <Loader2 className="spin" size={14} /> : <CheckCircle2 size={14} />}
              </button>
            </article>
          ))}
        </div>
      </LoadBoundary>
    </div>
  );
}


export function notificationMessage(notification: WorkbenchNotification) {
  const status = notification.run_status ? ` / ${notification.run_status}` : "";
  return `${notification.server_name} / ${notification.project_name}${status}`;
}
