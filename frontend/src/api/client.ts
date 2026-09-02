const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string;

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE';
  body?: unknown;
}

/**
 * Thin fetch wrapper that always sends the http-only session cookie and
 * surfaces backend error messages (already localized in Russian) for the UI
 * to show via Snackbar.
 */
export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: options.method ?? 'GET',
    credentials: 'include',
    headers: options.body ? { 'Content-Type': 'application/json' } : undefined,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  const isJson = res.headers.get('content-type')?.includes('application/json');
  const data = isJson ? await res.json() : null;

  if (!res.ok) {
    const message = (data && (data as { error?: string }).error) || 'Произошла ошибка';
    throw new ApiError(res.status, message);
  }

  return data as T;
}

export interface PresignResponse {
  uploadUrl: string; // PUT the file here
  fileUrl: string; // browser-facing URL to store once the PUT succeeds
  key: string; // S3 object key, kept so unsaved uploads can be cleaned up
  contentType: string; // send this verbatim as the PUT Content-Type header
  expiresIn: number; // seconds the uploadUrl stays valid
}

/**
 * Ask the backend for a short-lived presigned S3 URL so the browser can upload a
 * file straight to object storage. This bypasses the API/proxy body-size limits,
 * so file size is bounded only by S3 itself.
 */
export async function apiPresignUpload(filename: string, contentType: string): Promise<PresignResponse> {
  return apiRequest<PresignResponse>('/uploads/presign', {
    method: 'POST',
    body: { filename, contentType },
  });
}

/**
 * Delete objects that were uploaded to S3 (via presign) but never saved onto a
 * task — e.g. the "new task" dialog was cancelled. Best-effort: failures are the
 * caller's to swallow.
 */
export async function apiDeleteUploads(keys: string[]): Promise<void> {
  if (keys.length === 0) return;
  const qs = encodeURIComponent(keys.join(','));
  await apiRequest<{ deleted: number }>(`/uploads?keys=${qs}`, { method: 'DELETE' });
}

export interface S3UploadOptions {
  onProgress?: (percent: number) => void;
  signal?: AbortSignal;
}

/** Sentinel message for a user/unmount-cancelled upload (checked by callers). */
export const UPLOAD_CANCELLED = 'Загрузка отменена';

/**
 * PUT a file directly to a presigned S3 URL. Uses XMLHttpRequest rather than
 * fetch so we get real upload-progress events. `contentType` MUST equal the
 * value the presign call was made with or S3 rejects the signature.
 */
export function uploadFileToS3(
  uploadUrl: string,
  file: File,
  contentType: string,
  { onProgress, signal }: S3UploadOptions = {},
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('PUT', uploadUrl, true);
    xhr.setRequestHeader('Content-Type', contentType);
    // No cookies to S3: keeps this a "simple" CORS PUT and avoids needing
    // Access-Control-Allow-Credentials on the bucket.
    xhr.withCredentials = false;
    xhr.timeout = 10 * 60 * 1000; // 10 min ceiling for very large files

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress?.(Math.round((e.loaded / e.total) * 100));
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new ApiError(xhr.status, `Хранилище отклонило файл (код ${xhr.status})`));
    };
    xhr.onerror = () => reject(new ApiError(0, 'Сетевая ошибка при загрузке в хранилище'));
    xhr.ontimeout = () => reject(new ApiError(0, 'Время загрузки истекло, попробуйте ещё раз'));
    xhr.onabort = () => reject(new ApiError(0, UPLOAD_CANCELLED));

    if (signal) {
      if (signal.aborted) {
        xhr.abort();
        return;
      }
      signal.addEventListener('abort', () => xhr.abort(), { once: true });
    }

    xhr.send(file);
  });
}

export async function apiUpload(file: File): Promise<{ url: string; name: string }> {
  const form = new FormData();
  form.append('file', file);

  const res = await fetch(`${API_BASE_URL}/uploads`, {
    method: 'POST',
    credentials: 'include',
    body: form,
  });

  // A proxy (nginx) rejecting an oversized upload replies with an HTML body, not
  // JSON, so parse defensively and map the common failure to a clear message.
  const isJson = res.headers.get('content-type')?.includes('application/json');
  const data = isJson ? await res.json() : null;

  if (!res.ok) {
    const message =
      (data && (data as { error?: string }).error) ||
      (res.status === 413 ? 'Файл слишком большой (максимум 25 МБ)' : 'Не удалось загрузить файл');
    throw new ApiError(res.status, message);
  }
  return data as { url: string; name: string };
}
