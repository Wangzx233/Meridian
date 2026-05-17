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
import type { EmailNotificationConfig, EmailNotificationConfigRequest, EmailTLSMode } from "../../types";
import { emailTLSModes, productName } from "../../shared/constants";
import { useI18n } from "../../shared/i18n";
import type { LoadState } from "../../shared/loadState";
import { LoadBoundary, StatusBadge } from "../../shared/ui";

type EmailNotificationDraft = {
  name: string;
  enabled: boolean;
  smtp_host: string;
  smtp_port: string;
  smtp_username: string;
  smtp_password: string;
  from_address: string;
  to_addresses: string;
  tls_mode: EmailTLSMode;
  subject_prefix: string;
};

const emptyEmailNotificationDraft: EmailNotificationDraft = {
  name: "",
  enabled: true,
  smtp_host: "",
  smtp_port: "587",
  smtp_username: "",
  smtp_password: "",
  from_address: "",
  to_addresses: "",
  tls_mode: "starttls",
  subject_prefix: `[${productName}]`,
};

export function SettingsDialog(props: {
  emailConfigs: EmailNotificationConfig[];
  state: LoadState;
  creatingEmail: boolean;
  updatingEmail: boolean;
  deletingEmail: boolean;
  onCreateEmail: (input: EmailNotificationConfigRequest) => void;
  onUpdateEmail: (id: string, input: Partial<EmailNotificationConfigRequest>) => void;
  onDeleteEmail: (id: string) => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const [isAdding, setIsAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<EmailNotificationDraft>(emptyEmailNotificationDraft);
  const busy = props.creatingEmail || props.updatingEmail || props.deletingEmail;
  const editingConfig = props.emailConfigs.find((config) => config.id === editingId) ?? null;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        props.onClose();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [props]);

  const reset = () => {
    setDraft(emptyEmailNotificationDraft);
    setEditingId(null);
    setIsAdding(false);
  };

  const startCreate = () => {
    setDraft(emptyEmailNotificationDraft);
    setEditingId(null);
    setIsAdding(true);
  };

  const startEdit = (config: EmailNotificationConfig) => {
    setDraft(emailConfigToDraft(config));
    setEditingId(config.id);
    setIsAdding(true);
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const input = draftToEmailConfigRequest(draft);
    if (!input) {
      return;
    }
    if (editingConfig) {
      const update: Partial<EmailNotificationConfigRequest> = { ...input };
      if (!draft.smtp_password.trim()) {
        delete update.smtp_password;
      }
      props.onUpdateEmail(editingConfig.id, update);
    } else {
      props.onCreateEmail(input);
    }
    reset();
  };

  const updateDraft = <K extends keyof EmailNotificationDraft>(key: K, value: EmailNotificationDraft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const canSubmit = Boolean(
    draft.name.trim() &&
      draft.smtp_host.trim() &&
      Number(draft.smtp_port) >= 1 &&
      Number(draft.smtp_port) <= 65535 &&
      draft.from_address.trim() &&
      draft.to_addresses.trim(),
  );

  return (
    <div className="modalScrim" role="presentation" onMouseDown={props.onClose}>
      <section className="settingsDialog" role="dialog" aria-modal="true" aria-label={t("settings.title")} onMouseDown={(event) => event.stopPropagation()}>
        <div className="dialogHeader">
          <div className="panelTitle">
            <span className="panelIcon" aria-hidden="true">
              <SettingsIcon size={16} />
            </span>
            <div>
              <h2>{t("settings.title")}</h2>
              <p>{t("settings.subtitle")}</p>
            </div>
          </div>
          <button className="iconButton" type="button" onClick={props.onClose} aria-label={t("settings.close")} title={t("settings.close")}>
            <X size={16} />
          </button>
        </div>

        <div className="settingsBody">
          <aside className="settingsNav" aria-label={t("settings.sections")}>
            <button className="settingsNavItem isSelected" type="button">
              <Mail size={15} />
              {t("settings.emailAlerts")}
            </button>
          </aside>

          <section className="settingsContent" aria-label={t("settings.emailAria")}>
            <div className="settingsSectionHeader">
              <div>
                <h3>{t("settings.emailAlerts")}</h3>
                <p>{t("settings.emailHelp")}</p>
              </div>
              <button className="primaryButton compact" type="button" onClick={startCreate} disabled={busy}>
                <Plus size={15} />
                {t("settings.add")}
              </button>
            </div>

            {isAdding ? (
              <form className="emailConfigForm" onSubmit={submit}>
                <div className="formGridTwo">
                  <label htmlFor="email-config-name">
                    {t("settings.name")}
                    <input
                      id="email-config-name"
                      value={draft.name}
                      onChange={(event) => updateDraft("name", event.target.value)}
                      disabled={busy}
                      required
                    />
                  </label>
                  <label className="toggleRow" htmlFor="email-config-enabled">
                    <input
                      id="email-config-enabled"
                      type="checkbox"
                      checked={draft.enabled}
                      onChange={(event) => updateDraft("enabled", event.target.checked)}
                      disabled={busy}
                    />
                    {t("settings.enabled")}
                  </label>
                </div>

                <div className="formGridThree">
                  <label htmlFor="email-smtp-host">
                    {t("settings.smtpHost")}
                    <input
                      id="email-smtp-host"
                      value={draft.smtp_host}
                      onChange={(event) => updateDraft("smtp_host", event.target.value)}
                      disabled={busy}
                      required
                    />
                  </label>
                  <label htmlFor="email-smtp-port">
                    {t("settings.port")}
                    <input
                      id="email-smtp-port"
                      type="number"
                      min={1}
                      max={65535}
                      value={draft.smtp_port}
                      onChange={(event) => updateDraft("smtp_port", event.target.value)}
                      disabled={busy}
                      required
                    />
                  </label>
                  <label htmlFor="email-tls-mode">
                    {t("settings.tls")}
                    <select
                      id="email-tls-mode"
                      value={draft.tls_mode}
                      onChange={(event) => updateDraft("tls_mode", event.target.value as EmailTLSMode)}
                      disabled={busy}
                    >
                      {emailTLSModes.map((mode) => (
                        <option key={mode} value={mode}>
                          {mode}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>

                <div className="formGridTwo">
                  <label htmlFor="email-smtp-username">
                    {t("settings.smtpUsername")}
                    <input
                      id="email-smtp-username"
                      value={draft.smtp_username}
                      onChange={(event) => updateDraft("smtp_username", event.target.value)}
                      disabled={busy}
                    />
                  </label>
                  <label htmlFor="email-smtp-password">
                    {t("settings.smtpPassword")}
                    <input
                      id="email-smtp-password"
                      type="password"
                      value={draft.smtp_password}
                      onChange={(event) => updateDraft("smtp_password", event.target.value)}
                      disabled={busy}
                      placeholder={editingConfig ? t("settings.keepPassword") : ""}
                    />
                  </label>
                </div>

                <div className="formGridTwo">
                  <label htmlFor="email-from-address">
                    {t("settings.from")}
                    <input
                      id="email-from-address"
                      type="email"
                      value={draft.from_address}
                      onChange={(event) => updateDraft("from_address", event.target.value)}
                      disabled={busy}
                      required
                    />
                  </label>
                  <label htmlFor="email-subject-prefix">
                    {t("settings.subjectPrefix")}
                    <input
                      id="email-subject-prefix"
                      value={draft.subject_prefix}
                      onChange={(event) => updateDraft("subject_prefix", event.target.value)}
                      disabled={busy}
                    />
                  </label>
                </div>

                <label htmlFor="email-to-addresses">
                  {t("settings.recipients")}
                  <textarea
                    id="email-to-addresses"
                    value={draft.to_addresses}
                    onChange={(event) => updateDraft("to_addresses", event.target.value)}
                    disabled={busy}
                    rows={3}
                    placeholder="dev@example.com, ops@example.com"
                    required
                  />
                </label>

                <div className="dialogActions">
                  <button className="ghostButton" type="button" onClick={reset} disabled={busy}>
                    {t("settings.cancel")}
                  </button>
                  <button className="primaryButton" type="submit" disabled={busy || !canSubmit}>
                    {props.creatingEmail || props.updatingEmail ? <Loader2 className="spin" size={16} /> : <Save size={16} />}
                    {editingConfig ? t("settings.update") : t("settings.save")}
                  </button>
                </div>
              </form>
            ) : null}

            <LoadBoundary
              state={props.state}
              empty={props.emailConfigs.length === 0}
              emptyTitle={t("settings.noEmailAlerts")}
              emptyBody={t("settings.noEmailAlertsBody")}
            >
              <div className="emailConfigList">
                {props.emailConfigs.map((config) => (
                  <article key={config.id} className="emailConfigRow">
                    <div className="emailConfigMain">
                      <div className="rowTop">
                        <h4>{config.name}</h4>
                        <StatusBadge status={config.enabled ? "online" : "offline"} />
                      </div>
                      <p>
                        {config.smtp_host}:{config.smtp_port} / {config.tls_mode}
                      </p>
                      <p>{config.to_addresses.join(", ")}</p>
                    </div>
                    <div className="emailConfigActions">
                      <button className="ghostButton compact" type="button" onClick={() => startEdit(config)} disabled={busy}>
                        <Edit3 size={14} />
                        {t("settings.edit")}
                      </button>
                      <button className="dangerButton compact" type="button" onClick={() => props.onDeleteEmail(config.id)} disabled={busy}>
                        <Trash2 size={14} />
                        {t("settings.delete")}
                      </button>
                    </div>
                  </article>
                ))}
              </div>
            </LoadBoundary>
          </section>
        </div>
      </section>
    </div>
  );
}

function emailConfigToDraft(config: EmailNotificationConfig): EmailNotificationDraft {
  return {
    name: config.name,
    enabled: config.enabled,
    smtp_host: config.smtp_host,
    smtp_port: String(config.smtp_port),
    smtp_username: config.smtp_username,
    smtp_password: "",
    from_address: config.from_address,
    to_addresses: config.to_addresses.join(", "),
    tls_mode: config.tls_mode,
    subject_prefix: config.subject_prefix,
  };
}

function draftToEmailConfigRequest(draft: EmailNotificationDraft): EmailNotificationConfigRequest | null {
  const port = Number(draft.smtp_port);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return null;
  }
  const toAddresses = draft.to_addresses
    .split(/[\n,;]/)
    .map((address) => address.trim())
    .filter(Boolean);
  if (!draft.name.trim() || !draft.smtp_host.trim() || !draft.from_address.trim() || toAddresses.length === 0) {
    return null;
  }
  return {
    name: draft.name.trim(),
    enabled: draft.enabled,
    smtp_host: draft.smtp_host.trim(),
    smtp_port: port,
    smtp_username: draft.smtp_username.trim(),
    smtp_password: draft.smtp_password,
    from_address: draft.from_address.trim(),
    to_addresses: toAddresses,
    tls_mode: draft.tls_mode,
    subject_prefix: draft.subject_prefix.trim(),
  };
}
