import { Upload } from "tus-js-client";
import { apiBaseUrl, ApiError } from "../../api";
import type { ProjectFileActionResult } from "../../types";

const uploadChunkBytes = 4 * 1024 * 1024;

export type ProjectFileUploadProgress = {
  id: string;
  projectId: string;
  directoryPath: string;
  filename: string;
  uploadedBytes: number;
  totalBytes: number;
  sentBytes: number;
  complete: boolean;
  resumed: boolean;
  error: unknown | null;
  result: ProjectFileActionResult | null;
};

type UploadProjectFileInput = {
  projectId: string;
  path: string;
  file: File;
  create_dirs?: boolean;
};

type Listener = () => void;

const uploads = new Map<string, ProjectFileUploadProgress>();
const listeners = new Set<Listener>();
let uploadSnapshot: ProjectFileUploadProgress[] = [];

function notify() {
  uploadSnapshot = Array.from(uploads.values());
  for (const listener of listeners) {
    listener();
  }
}

export function subscribeProjectFileUploads(listener: Listener) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function projectFileUploadSnapshot() {
  return uploadSnapshot;
}

export function clearCompletedProjectFileUpload(id: string) {
  const upload = uploads.get(id);
  if (!upload || (!upload.complete && !upload.error)) {
    return;
  }
  uploads.delete(id);
  notify();
}

export function uploadProjectFile(input: UploadProjectFileInput) {
  const filename = input.file.name || "upload.bin";
  const id = uploadIDForFile(input.projectId, input.path, input.file);
  const existing = uploads.get(id);
  if (existing && !existing.complete && !existing.error) {
    return existing;
  }

  const initial: ProjectFileUploadProgress = {
    id,
    projectId: input.projectId,
    directoryPath: input.path,
    filename,
    uploadedBytes: 0,
    sentBytes: 0,
    totalBytes: input.file.size,
    complete: false,
    resumed: false,
    error: null,
    result: null,
  };
  uploads.set(id, initial);
  notify();

  const endpoint = `${apiBaseUrl}/projects/${encodeURIComponent(input.projectId)}/files/upload/tus`;
  const upload = new Upload(input.file, {
    endpoint,
    chunkSize: uploadChunkBytes,
    retryDelays: [0, 1000, 3000, 5000],
    removeFingerprintOnSuccess: true,
    metadata: {
      filename,
      path: input.path,
      upload_id: id,
      create_dirs: String(input.create_dirs ?? true),
    },
    onProgress(uploadedBytes, totalBytes) {
      updateUpload(id, {
        sentBytes: uploadedBytes,
        totalBytes,
      });
    },
    onAfterResponse(_req, res) {
      const offset = parseUploadOffset(res.getHeader("Upload-Offset"));
      if (offset !== null) {
        updateUpload(id, {
          uploadedBytes: Math.min(offset, input.file.size),
          totalBytes: input.file.size,
          resumed: offset > 0,
        });
      }
      const info = res.getHeader("Upload-Info");
      if (!info) {
        return;
      }
      const result = decodeUploadInfo(info);
      if (result) {
        updateUpload(id, { result });
      }
    },
    onError(error) {
      updateUpload(id, { error: normalizeTusError(error) });
    },
    onSuccess() {
      const current = uploads.get(id);
      updateUpload(id, {
        uploadedBytes: input.file.size,
        sentBytes: input.file.size,
        totalBytes: input.file.size,
        complete: true,
        error: null,
        result: current?.result ?? {
          root: "",
          path: joinProjectPath(input.path, filename),
          size: input.file.size,
          uploaded_bytes: input.file.size,
          total_size: input.file.size,
          complete: true,
          resume_offset: input.file.size,
        },
      });
    },
  });
  upload.findPreviousUploads().then((previousUploads) => {
    if (previousUploads.length > 0) {
      upload.resumeFromPreviousUpload(previousUploads[0]);
      updateUpload(id, { resumed: true });
    }
    upload.start();
  }).catch((error) => {
    updateUpload(id, { error: normalizeTusError(error) });
  });
  return initial;
}

function updateUpload(id: string, patch: Partial<ProjectFileUploadProgress>) {
  const current = uploads.get(id);
  if (!current) {
    return;
  }
  uploads.set(id, { ...current, ...patch });
  notify();
}

function uploadIDForFile(projectId: string, path: string, file: File) {
  const name = file.name || "upload.bin";
  const seed = `${projectId}/${path}/${name}:${file.size}:${file.lastModified}`;
  let hash = 2166136261;
  for (let i = 0; i < seed.length; i += 1) {
    hash ^= seed.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  return `up-${file.size}-${file.lastModified || 0}-${(hash >>> 0).toString(16)}`;
}

function decodeUploadInfo(value: string): ProjectFileActionResult | null {
  try {
    return JSON.parse(atob(base64UrlToBase64(value))) as ProjectFileActionResult;
  } catch {
    return null;
  }
}

function parseUploadOffset(value: string | null | undefined) {
  if (!value) {
    return null;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null;
}

function base64UrlToBase64(value: string) {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/");
  return padded + "=".repeat((4 - (padded.length % 4)) % 4);
}

function normalizeTusError(error: unknown) {
  const message = error instanceof Error && error.message ? error.message : "Upload failed.";
  return new ApiError(0, message, "upload_failed");
}

function joinProjectPath(path: string, filename: string) {
  const cleanPath = path.trim().replace(/^\/+|\/+$/g, "");
  return cleanPath ? `${cleanPath}/${filename}` : filename;
}
