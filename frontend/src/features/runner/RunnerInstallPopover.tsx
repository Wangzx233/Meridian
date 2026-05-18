import { TerminalSquare, X } from "lucide-react";
import { useEffect, useState } from "react";
import type { Server } from "../../types";
import { defaultControlUrl } from "../../shared/constants";
import { useI18n } from "../../shared/i18n";
import type { Notice } from "../../shared/notices";
import { CopyCommand, InlineNotice } from "../../shared/ui";

export function RunnerInstallPopover(props: {
  server: Server | null;
  runnerToken: string;
  onClose: () => void;
  onNotice: (notice: Notice) => void;
}) {
  const { t } = useI18n();
  const [controlUrl, setControlUrl] = useState(defaultControlUrl);
  const installBaseUrl = controlUrl.replace(/\/$/, "");
  const authQuery = () => {
    const query = new URLSearchParams();
    if (props.runnerToken) {
      query.set("runner_token", props.runnerToken);
    }
    const value = query.toString();
    return value ? `?${value}` : "";
  };
  const installQuery = authQuery();
  const windowsInstallCommand = `powershell -ExecutionPolicy Bypass -NoProfile -Command "iex ((iwr -UseBasicParsing -Uri '${installBaseUrl}/api/v1/runner/install.ps1${installQuery}').Content)"`;
  const shellInstallCommand = `curl -fsSL '${installBaseUrl}/api/v1/runner/install.sh${installQuery}' | sh`;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        props.onClose();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [props]);

  const copied = (platform: string) => {
    props.onNotice({ tone: "success", message: t("runner.copied", { platform }) });
  };

  return (
    <div id="runner-install-popover" className="runnerInstallPopover" role="dialog" aria-label={t("runner.aria")}>
      <div className="popoverHeader">
        <div className="panelTitle">
          <span className="panelIcon" aria-hidden="true">
            <TerminalSquare size={16} />
          </span>
          <div>
            <h2>{t("runner.title")}</h2>
            <p>{t("runner.subtitle")}</p>
          </div>
        </div>
        <button className="iconButton" type="button" onClick={props.onClose} aria-label={t("runner.close")} title={t("runner.close")}>
          <X size={16} />
        </button>
      </div>

      <div className="installPanel">
        <label htmlFor="runner-control-url">
          {t("runner.controlUrl")}
          <input
            id="runner-control-url"
            value={controlUrl}
            onChange={(event) => setControlUrl(event.target.value)}
            placeholder="http://127.0.0.1:18080"
          />
        </label>
        <div className="installCommandList">
          <CopyCommand
            label="Linux"
            command={shellInstallCommand}
            copyLabel={t("runner.copy", { platform: "Linux" })}
            onCopied={() => copied("Linux")}
          />
          <CopyCommand
            label="macOS"
            command={shellInstallCommand}
            copyLabel={t("runner.copy", { platform: "macOS" })}
            onCopied={() => copied("macOS")}
          />
          <CopyCommand
            label="Windows"
            command={windowsInstallCommand}
            copyLabel={t("runner.copy", { platform: "Windows" })}
            onCopied={() => copied("Windows")}
          />
        </div>
        <InlineNotice tone="info">{t("runner.remoteNotice")}</InlineNotice>
      </div>
    </div>
  );
}
