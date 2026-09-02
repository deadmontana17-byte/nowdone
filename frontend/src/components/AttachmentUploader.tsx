import { useEffect, useRef, useState } from 'react';
import { Box, Chip, IconButton, CircularProgress, LinearProgress, Typography } from '@mui/material';
import AttachFileIcon from '@mui/icons-material/AttachFile';
import CloseIcon from '@mui/icons-material/Close';

import type { Attachment } from '@/types';
import { apiPresignUpload, uploadFileToS3, apiUpload, ApiError, UPLOAD_CANCELLED } from '@/api/client';
import { useUiStore } from '@/store/uiStore';
import { ATTACHMENT_ACCEPT } from '@/utils/attachments';

interface AttachmentUploaderProps {
  attachments: Attachment[];
  onChange: (attachments: Attachment[]) => void;
}

function attachmentType(mime: string): Attachment['type'] {
  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('video/')) return 'video';
  return 'file';
}

/**
 * Uploads attachments straight to S3 via a backend-issued presigned URL, showing
 * real upload progress. Falls back to the legacy "through the API" upload if the
 * direct path is unavailable (e.g. bucket CORS not configured yet).
 */
export function AttachmentUploader({ attachments, onChange }: AttachmentUploaderProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  // Files already uploaded in this dialog session, keyed by name+size+mtime, so
  // re-picking the same file reuses its S3 URL instead of uploading again.
  const uploadCacheRef = useRef<Map<string, Attachment>>(new Map());
  // null = idle, otherwise 0..100 for the current upload.
  const [progress, setProgress] = useState<number | null>(null);
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  // Cancel an in-flight upload if the dialog/component unmounts.
  useEffect(() => () => abortRef.current?.abort(), []);

  // Primary path: presign, then PUT the bytes straight to S3.
  async function uploadDirect(file: File): Promise<Attachment> {
    const contentType = file.type || 'application/octet-stream';
    const presign = await apiPresignUpload(file.name, contentType);

    const controller = new AbortController();
    abortRef.current = controller;

    await uploadFileToS3(presign.uploadUrl, file, presign.contentType, {
      onProgress: setProgress,
      signal: controller.signal,
    });

    return { type: attachmentType(contentType), url: presign.fileUrl, name: file.name };
  }

  // Fallback path: stream the file through the API (bounded at 25 MB).
  async function uploadViaApi(file: File): Promise<Attachment> {
    const { url, name } = await apiUpload(file);
    return { type: attachmentType(file.type), url, name };
  }

  async function handleFileSelected(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    // Same file picked twice in one session → reuse, don't re-upload.
    const cacheKey = `${file.name}:${file.size}:${file.lastModified}`;
    const cached = uploadCacheRef.current.get(cacheKey);
    if (cached) {
      onChange([...attachments, cached]);
      if (inputRef.current) inputRef.current.value = '';
      return;
    }

    setProgress(0);
    try {
      let attachment: Attachment;
      try {
        attachment = await uploadDirect(file);
      } catch (directErr) {
        // A deliberate cancel must not trigger the fallback.
        if (directErr instanceof ApiError && directErr.message === UPLOAD_CANCELLED) throw directErr;
        // Direct-to-S3 failed (endpoint missing, bucket CORS, network) — retry
        // through the API so uploads keep working while S3 is being configured.
        console.warn('direct S3 upload failed, falling back to API upload', directErr);
        setProgress(null); // API upload has no progress events
        attachment = await uploadViaApi(file);
      }
      uploadCacheRef.current.set(cacheKey, attachment);
      onChange([...attachments, attachment]);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Не удалось загрузить файл';
      showSnackbar(message, 'error');
    } finally {
      abortRef.current = null;
      setProgress(null);
      if (inputRef.current) inputRef.current.value = '';
    }
  }

  function removeAttachment(index: number) {
    onChange(attachments.filter((_, i) => i !== index));
  }

  const isUploading = progress !== null;

  return (
    <Box>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 1 }}>
        {attachments.map((a, i) => (
          <Chip
            key={i}
            label={a.name}
            onDelete={() => removeAttachment(i)}
            deleteIcon={<CloseIcon />}
            variant="outlined"
          />
        ))}
      </Box>

      <input ref={inputRef} type="file" hidden accept={ATTACHMENT_ACCEPT} onChange={handleFileSelected} />
      <IconButton onClick={() => inputRef.current?.click()} disabled={isUploading} size="small">
        {isUploading ? <CircularProgress size={18} /> : <AttachFileIcon fontSize="small" />}
      </IconButton>

      {isUploading && (
        <Box sx={{ mt: 1 }}>
          <LinearProgress variant="determinate" value={progress ?? 0} />
          <Typography variant="caption" color="text.secondary">
            Загрузка… {progress ?? 0}%
          </Typography>
        </Box>
      )}
    </Box>
  );
}
