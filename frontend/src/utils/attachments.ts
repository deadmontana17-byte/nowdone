/**
 * Shared attachment helpers.
 */

/** `accept` for the file picker: images + video (unchanged) plus common
 * documents — PDF, Word, Excel, plain text, ZIP/RAR archives. */
export const ATTACHMENT_ACCEPT = [
  'image/*',
  'video/*',
  'application/pdf',
  '.pdf',
  '.doc',
  '.docx',
  'application/msword',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  '.xls',
  '.xlsx',
  'application/vnd.ms-excel',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  '.txt',
  'text/plain',
  '.zip',
  'application/zip',
  'application/x-zip-compressed',
  '.rar',
  'application/vnd.rar',
  'application/x-rar-compressed',
].join(',');

/**
 * Pull the S3 object key ("attachments/<uuid>.ext") out of a stored file URL,
 * independent of the storage host. Returns null for URLs that aren't ours.
 * Used to clean up files that were uploaded but never saved onto a task.
 */
export function attachmentKeyFromUrl(url: string): string | null {
  const match = url.match(/(attachments\/[^/?#]+)/);
  return match ? match[1] : null;
}
