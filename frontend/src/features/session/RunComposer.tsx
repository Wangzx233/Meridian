import {
  CheckCircle2,
  ClipboardList,
  FileText,
  ImagePlus,
  Loader2,
  Mic,
  Play,
  Reply,
  Square,
  X,
  Zap,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import type { CodexReasoningEffort, CodexServiceTier, CreateRunInputImage, CreateRunMode, Run, Task } from "../../types";
import { modelOptions, reasoningEffortOptions } from "../../shared/constants";
import { useI18n } from "../../shared/i18n";
import { InlineNotice } from "../../shared/ui";

export function RunComposer(props: {
  task: Task;
  runs: Run[];
  message: string;
  onMessageChange: (value: string) => void;
  mode: CreateRunMode;
  onModeChange: (value: CreateRunMode) => void;
  codexModel: string;
  onCodexModelChange: (value: string) => void;
  reasoningEffort: CodexReasoningEffort;
  onReasoningEffortChange: (value: CodexReasoningEffort) => void;
  serviceTier: CodexServiceTier;
  onServiceTierChange: (value: CodexServiceTier) => void;
  goalMode: boolean;
  onGoalModeChange: () => void;
  reminderCallbacksEnabled: boolean;
  onReminderCallbacksChange: () => void;
  canUseReminderCallbacks: boolean;
  reminderCallbacksBlockedReason?: string;
  contextCount: number;
  inputImages: CreateRunInputImage[];
  onInputImagesChange: (value: CreateRunInputImage[]) => void;
  canUseImageInput: boolean;
  imageInputBlockedReason?: string;
  onImageInputBlocked: () => void;
  disabled: boolean;
  canInterrupt: boolean;
  canCompact: boolean;
  canChangeGoal: boolean;
  canMarkDone: boolean;
  markingDone: boolean;
  onMarkDone: () => void;
  blockedReason?: string;
  submitting: boolean;
  onSubmit: (event: FormEvent) => void;
  onCompact: () => void;
  onInterrupt: () => void;
  interrupting: boolean;
}) {
  const { t } = useI18n();
  const hasObservedSession = Boolean(props.task.codex_session_id || props.runs.some((run) => run.codex_session_id));
  const showMissingSessionHint = props.mode === "resume" && !hasObservedSession;
  const messageReady = Boolean(props.message.trim()) && !showMissingSessionHint;
  const inputRef = useRef<HTMLTextAreaElement | null>(null);
  const imageInputRef = useRef<HTMLInputElement | null>(null);
  const messageRef = useRef(props.message);
  const recognitionRef = useRef<SpeechRecognition | null>(null);
  const [listening, setListening] = useState(false);
  const speechSupported = typeof window !== "undefined" && Boolean(window.SpeechRecognition || window.webkitSpeechRecognition);
  const composerLocked = (props.disabled && !props.canInterrupt) || props.submitting || props.interrupting;
  const imageInputDisabled = composerLocked || !props.canUseImageInput;

  useEffect(() => {
    messageRef.current = props.message;
  }, [props.message]);

  useEffect(() => {
    return () => {
      recognitionRef.current?.stop();
      recognitionRef.current = null;
    };
  }, []);

  const focusInput = () => {
    inputRef.current?.focus();
  };

  const toggleSpeech = () => {
    if (!speechSupported) {
      focusInput();
      return;
    }
    if (listening) {
      recognitionRef.current?.stop();
      setListening(false);
      return;
    }
    const Recognition = window.SpeechRecognition ?? window.webkitSpeechRecognition;
    if (!Recognition) {
      focusInput();
      return;
    }
    const recognition = new Recognition();
    recognition.lang = navigator.language || "en-US";
    recognition.continuous = true;
    recognition.interimResults = false;
    recognition.onresult = (event) => {
      let transcript = "";
      for (let index = event.resultIndex; index < event.results.length; index += 1) {
        transcript += event.results[index][0]?.transcript ?? "";
      }
      const text = transcript.trim();
      if (!text) {
        return;
      }
      const nextMessage = appendSpeechText(messageRef.current, text);
      messageRef.current = nextMessage;
      props.onMessageChange(nextMessage);
    };
    recognition.onerror = () => setListening(false);
    recognition.onend = () => setListening(false);
    recognitionRef.current = recognition;
    recognition.start();
    setListening(true);
  };

  const chooseImages = () => {
    if (composerLocked) {
      focusInput();
      return;
    }
    if (!props.canUseImageInput) {
      props.onImageInputBlocked();
      return;
    }
    imageInputRef.current?.click();
  };

  const addImages = async (files: FileList | null) => {
    if (!files || files.length === 0) {
      return;
    }
    const existing = props.inputImages;
    const remaining = Math.max(0, maxInputImages - existing.length);
    const selected = Array.from(files).slice(0, remaining);
    if (selected.length === 0) {
      return;
    }
    const nextImages = [...existing];
    let totalBytes = existing.reduce((total, image) => total + base64ByteSize(image.content_base64), 0);
    for (const file of selected) {
      if (!allowedImageTypes.has(file.type) || file.size <= 0 || file.size > maxInputImageBytes) {
        continue;
      }
      if (totalBytes + file.size > maxInputImageTotalBytes) {
        continue;
      }
      try {
        nextImages.push({
          filename: file.name || "image",
          mime_type: file.type,
          content_base64: await fileToBase64(file),
        });
        totalBytes += file.size;
      } catch {
        // Ignore unreadable files; browser file reads can fail if the source is removed.
      }
    }
    props.onInputImagesChange(nextImages);
  };

  const removeImage = (index: number) => {
    props.onInputImagesChange(props.inputImages.filter((_, itemIndex) => itemIndex !== index));
  };

  return (
    <form className="composer" onSubmit={props.onSubmit}>
      <div className="composerToolbar">
        <div>
          <label htmlFor="run-message">{t("session.instruction")}</label>
          <p>{t("composer.contextSelected", { count: props.contextCount })}</p>
        </div>
        <label className="selectLabel" htmlFor="run-mode">
          {t("composer.mode")}
          <select
            id="run-mode"
            value={props.mode}
            onChange={(event) => props.onModeChange(event.target.value as CreateRunMode)}
            disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
          >
            <option value="auto">auto</option>
            <option value="resume">resume</option>
            <option value="new">new</option>
          </select>
        </label>
        <label className="selectLabel" htmlFor="run-model">
          {t("composer.model")}
          <input
            id="run-model"
            list="codex-model-options"
            value={props.codexModel}
            onChange={(event) => props.onCodexModelChange(event.target.value)}
            disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
            placeholder={t("composer.modelDefault")}
          />
          <datalist id="codex-model-options">
            {modelOptions.filter(Boolean).map((model) => (
              <option key={model} value={model} />
            ))}
          </datalist>
        </label>
        <label className="selectLabel" htmlFor="run-reasoning-effort">
          {t("composer.reasoning")}
          <select
            id="run-reasoning-effort"
            value={props.reasoningEffort}
            onChange={(event) => props.onReasoningEffortChange(event.target.value as CodexReasoningEffort)}
            disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
          >
            {reasoningEffortOptions.map((effort) => (
              <option key={effort || "default"} value={effort}>
                {effort || t("composer.modelDefault")}
              </option>
            ))}
          </select>
        </label>
        <div className="composerSwitchGroup" aria-label="Codex controls">
          <button
            className={`optionToggle ${props.serviceTier === "fast" ? "isSelected" : ""}`}
            type="button"
            onClick={() => props.onServiceTierChange(props.serviceTier === "fast" ? "" : "fast")}
            disabled={props.submitting || props.interrupting}
            title={t("composer.fastTitle")}
          >
            <Zap size={14} />
            {t("composer.fast")}
          </button>
          <button
            className={`optionToggle ${props.goalMode ? "isSelected" : ""}`}
            type="button"
            onClick={props.onGoalModeChange}
            disabled={!props.canChangeGoal || props.submitting || props.interrupting}
            title={t("composer.goalTitle")}
          >
            <ClipboardList size={14} />
            {t("composer.goal")}
          </button>
          <button
            className={`optionToggle ${props.reminderCallbacksEnabled ? "isSelected" : ""}`}
            type="button"
            onClick={props.onReminderCallbacksChange}
            disabled={!props.canUseReminderCallbacks || props.submitting || props.interrupting}
            title={props.canUseReminderCallbacks ? t("composer.remindersTitle") : props.reminderCallbacksBlockedReason}
          >
            <Reply size={14} />
            {t("composer.reminders")}
          </button>
          <button
            className="ghostButton compact"
            type="button"
            onClick={props.onCompact}
            disabled={!props.canCompact || props.submitting || props.interrupting}
            title={t("composer.compactTitle")}
          >
            <FileText size={14} />
            {t("composer.compact")}
          </button>
        </div>
      </div>
      <textarea
        ref={inputRef}
        id="run-message"
        className="instructionInput"
        value={props.message}
        onChange={(event) => props.onMessageChange(event.target.value)}
        disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
        placeholder={t("composer.placeholder")}
        rows={6}
      />
      <input
        ref={imageInputRef}
        className="srOnly"
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        multiple
        onChange={(event) => {
          void addImages(event.target.files);
          event.target.value = "";
        }}
        disabled={imageInputDisabled}
      />
      {props.inputImages.length > 0 ? (
        <div className="imageAttachmentList" aria-label={t("composer.imagesAttached")}>
          {props.inputImages.map((image, index) => (
            <div className="imageAttachment" key={`${image.filename}-${index}`}>
              <img src={`data:${image.mime_type};base64,${image.content_base64}`} alt={image.filename} />
              <div>
                <strong>{image.filename}</strong>
                <span>{formatBytes(base64ByteSize(image.content_base64))}</span>
              </div>
              <button
                className="iconButton compact"
                type="button"
                onClick={() => removeImage(index)}
                aria-label={t("composer.removeImage", { name: image.filename })}
                title={t("composer.removeImage", { name: image.filename })}
                disabled={props.submitting || props.interrupting}
              >
                <X size={14} />
              </button>
            </div>
          ))}
        </div>
      ) : null}
      {props.blockedReason ? <InlineNotice tone="info">{props.blockedReason}</InlineNotice> : null}
      {showMissingSessionHint ? <InlineNotice tone="danger">{t("composer.noSession")}</InlineNotice> : null}
      <div className="composerActions">
        <span className="targetHint">{t("composer.targetHint")}</span>
        <button
          className="voiceButton"
          type="button"
          onClick={chooseImages}
          disabled={composerLocked || props.inputImages.length >= maxInputImages}
          aria-disabled={!props.canUseImageInput}
          title={props.canUseImageInput ? t("composer.imageTitle") : props.imageInputBlockedReason}
        >
          <ImagePlus size={16} />
          {props.inputImages.length > 0
            ? t("composer.imageCount", { count: props.inputImages.length })
            : t("composer.image")}
        </button>
        <button
          className={`voiceButton ${listening ? "isListening" : ""}`}
          type="button"
          onClick={toggleSpeech}
          disabled={(props.disabled && !props.canInterrupt) || props.submitting || props.interrupting}
          aria-pressed={listening}
          title={speechSupported ? t("composer.voiceTitle") : t("composer.voiceFallbackTitle")}
        >
          <Mic size={16} />
          {listening ? t("composer.voiceListening") : t("composer.voice")}
        </button>
        <button
          className="primaryButton"
          type="submit"
          disabled={props.disabled || props.submitting || props.interrupting || !messageReady}
        >
          {props.submitting ? <Loader2 className="spin" size={16} /> : <Play size={16} />}
          {t("composer.send")}
        </button>
        {props.canInterrupt ? (
          <button
            className="interruptButton"
            type="button"
            onClick={props.onInterrupt}
            disabled={props.submitting || props.interrupting || !messageReady}
          >
            {props.interrupting ? <Loader2 className="spin" size={16} /> : <Square size={16} />}
            {t("composer.interrupt")}
          </button>
        ) : null}
        <button
          className="doneButton"
          type="button"
          onClick={props.onMarkDone}
          disabled={!props.canMarkDone || props.submitting || props.interrupting || props.markingDone}
          title={t("complete.markDoneTitle")}
        >
          {props.markingDone ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
          {t("complete.markDone")}
        </button>
      </div>
    </form>
  );
}

function appendSpeechText(current: string, transcript: string) {
  const trimmedCurrent = current.trimEnd();
  const separator = trimmedCurrent ? " " : "";
  return `${trimmedCurrent}${separator}${transcript}`;
}

const maxInputImages = 4;
const maxInputImageBytes = 8 * 1024 * 1024;
const maxInputImageTotalBytes = 24 * 1024 * 1024;
const allowedImageTypes = new Set(["image/png", "image/jpeg", "image/gif", "image/webp"]);

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("Unable to read image."));
    reader.onload = () => {
      const value = typeof reader.result === "string" ? reader.result : "";
      const comma = value.indexOf(",");
      resolve(comma >= 0 ? value.slice(comma + 1) : value);
    };
    reader.readAsDataURL(file);
  });
}

function base64ByteSize(value: string) {
  const clean = value.replace(/\s/g, "");
  if (!clean) {
    return 0;
  }
  const padding = clean.endsWith("==") ? 2 : clean.endsWith("=") ? 1 : 0;
  return Math.max(0, Math.floor((clean.length * 3) / 4) - padding);
}

function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
