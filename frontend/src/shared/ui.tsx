import { AlertTriangle, Archive, Copy, Loader2, X } from "lucide-react";
import { useEffect, useState } from "react";
import type { KeyboardEvent as ReactKeyboardEvent, PointerEvent as ReactPointerEvent, ReactNode } from "react";
import type { Run, ServerStatus, TaskStatus } from "../types";
import { statusTone } from "../utils";
import { clamp } from "./format";
import { useI18n } from "./i18n";
import type { LoadState } from "./loadState";
import { errorNotice } from "./notices";
import type { Notice } from "./notices";

export function CapabilityPill(props: { available: boolean; label: string }) {
  return <span className={`capabilityPill ${props.available ? "isReady" : ""}`}>{props.label}</span>;
}

export function PanelHeader(props: { icon: ReactNode; title: string; detail?: string; action?: ReactNode }) {
  return (
    <div className="panelHeader">
      <div className="panelTitle">
        <span className="panelIcon" aria-hidden="true">
          {props.icon}
        </span>
        <div>
          <h2>{props.title}</h2>
          {props.detail ? <p>{props.detail}</p> : null}
        </div>
      </div>
      {props.action}
    </div>
  );
}


export function ResizeHandle(props: {
  label: string;
  hidden?: boolean;
  orientation?: "vertical" | "horizontal";
  direction?: "previous" | "next";
  min: number;
  max: number;
  value: number;
  onChange: (value: number) => void;
  onTap?: () => void;
}) {
  const orientation = props.orientation ?? "vertical";
  const direction = props.direction ?? "previous";
  const hidden = Boolean(props.hidden);
  const [drag, setDrag] = useState<{ pointerId: number; startPosition: number; startValue: number } | null>(null);

  useEffect(() => {
    const className = orientation === "vertical" ? "isResizingPanelX" : "isResizingPanelY";
    document.body.classList.toggle(className, Boolean(drag));
    return () => document.body.classList.remove(className);
  }, [drag, orientation]);

  const moveTo = (value: number) => {
    props.onChange(clamp(value, props.min, props.max));
  };

  const startDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (hidden || event.button !== 0) {
      return;
    }
    event.currentTarget.setPointerCapture(event.pointerId);
    setDrag({
      pointerId: event.pointerId,
      startPosition: orientation === "vertical" ? event.clientX : event.clientY,
      startValue: props.value,
    });
  };

  const moveDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!drag || event.pointerId !== drag.pointerId) {
      return;
    }
    const position = orientation === "vertical" ? event.clientX : event.clientY;
    const delta = position - drag.startPosition;
    moveTo(drag.startValue + (direction === "previous" ? delta : -delta));
  };

  const stopDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!drag || event.pointerId !== drag.pointerId) {
      return;
    }
    const position = orientation === "vertical" ? event.clientX : event.clientY;
    const totalDelta = position - drag.startPosition;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    setDrag(null);
    if (Math.abs(totalDelta) < 6) {
      props.onTap?.();
    }
  };

  const onKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (hidden) {
      return;
    }
    const step = event.shiftKey ? 40 : 16;
    const decreaseKey = orientation === "vertical" ? "ArrowLeft" : "ArrowUp";
    const increaseKey = orientation === "vertical" ? "ArrowRight" : "ArrowDown";
    const decreaseValue = direction === "previous" ? props.value - step : props.value + step;
    const increaseValue = direction === "previous" ? props.value + step : props.value - step;
    if (event.key === decreaseKey) {
      event.preventDefault();
      moveTo(decreaseValue);
    }
    if (event.key === increaseKey) {
      event.preventDefault();
      moveTo(increaseValue);
    }
    if (event.key === "Home") {
      event.preventDefault();
      moveTo(props.min);
    }
    if (event.key === "End") {
      event.preventDefault();
      moveTo(props.max);
    }
  };

  return (
    <div
      className={`resizeHandle ${orientation === "horizontal" ? "isHorizontal" : "isVertical"} ${drag ? "isDragging" : ""} ${hidden ? "isHidden" : ""}`}
      role="separator"
      aria-label={props.label}
      aria-orientation={orientation}
      aria-valuemin={props.min}
      aria-valuemax={props.max}
      aria-valuenow={props.value}
      aria-hidden={hidden}
      tabIndex={hidden ? -1 : 0}
      onPointerDown={startDrag}
      onPointerMove={moveDrag}
      onPointerUp={stopDrag}
      onPointerCancel={stopDrag}
      onKeyDown={onKeyDown}
    />
  );
}


export function CopyCommand(props: { label: string; command: string; onCopied: () => void; copyLabel?: string }) {
  const copy = () => {
    void navigator.clipboard.writeText(props.command).then(props.onCopied);
  };

  return (
    <div className="copyCommand">
      <span>{props.label}</span>
      <code>{props.command}</code>
      <button type="button" onClick={copy} aria-label={props.copyLabel ?? `Copy ${props.label} install command`} title={props.copyLabel ?? `Copy ${props.label} install command`}>
        <Copy size={14} />
      </button>
    </div>
  );
}


export function StatusBadge(props: { status: TaskStatus | Run["status"] | ServerStatus }) {
  const { t } = useI18n();
  return (
    <span className={`statusBadge tone-${statusTone(props.status)}`}>
      <span aria-hidden="true" />
      {t(`status.${props.status}`)}
    </span>
  );
}


export function Fact(props: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="fact">
      <span>{props.label}</span>
      <strong className={props.mono ? "mono" : ""}>{props.value}</strong>
    </div>
  );
}


export function LoadBoundary(props: {
  state: LoadState;
  empty: boolean;
  emptyTitle: string;
  emptyBody: string;
  children: ReactNode;
}) {
  if (props.state.isLoading) {
    return <LoadingState label="Loading" />;
  }

  if (props.state.isError) {
    return <ErrorState error={props.state.error} />;
  }

  if (props.empty) {
    return <EmptyState icon={<Archive size={22} />} title={props.emptyTitle} body={props.emptyBody} />;
  }

  return props.children;
}


export function LoadingState(props: { label: string }) {
  return (
    <div className="stateBlock">
      <Loader2 className="spin" size={18} />
      <span>{props.label}</span>
    </div>
  );
}


export function ErrorState(props: { error: unknown }) {
  const notice = errorNotice(props.error, "Unable to load data.");
  return (
    <div className="stateBlock error">
      <AlertTriangle size={18} />
      <span>{notice.message}</span>
    </div>
  );
}


export function EmptyState(props: { icon: ReactNode; title: string; body: string }) {
  return (
    <div className="emptyState">
      <span aria-hidden="true">{props.icon}</span>
      <h3>{props.title}</h3>
      <p>{props.body}</p>
    </div>
  );
}


export function InlineNotice(props: { tone: "info" | "danger"; children: ReactNode }) {
  return <div className={`inlineNotice ${props.tone}`}>{props.children}</div>;
}


export function Toast(props: { tone: Notice["tone"]; children: ReactNode; onDismiss: () => void }) {
  return (
    <div className={`toast ${props.tone}`} role="status" aria-live="polite">
      <span>{props.children}</span>
      <button type="button" onClick={props.onDismiss} aria-label="Dismiss notification">
        <X size={15} />
      </button>
    </div>
  );
}
