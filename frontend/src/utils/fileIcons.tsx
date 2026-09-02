import { Link as MuiLink, Typography } from '@mui/material';
import InsertDriveFileOutlinedIcon from '@mui/icons-material/InsertDriveFileOutlined';
import PictureAsPdfOutlinedIcon from '@mui/icons-material/PictureAsPdfOutlined';
import MovieOutlinedIcon from '@mui/icons-material/MovieOutlined';
import AudiotrackOutlinedIcon from '@mui/icons-material/AudiotrackOutlined';
import FolderZipOutlinedIcon from '@mui/icons-material/FolderZipOutlined';
import DescriptionOutlinedIcon from '@mui/icons-material/DescriptionOutlined';
import TableChartOutlinedIcon from '@mui/icons-material/TableChartOutlined';

import type { Attachment } from '@/types';

/**
 * Shared attachment rendering helpers.
 *
 * Extracted from TaskDetailDrawer so the note card (NoteDetailDrawer) shows
 * documents exactly the same way — an extension-based icon plus a click-to-
 * download link — instead of duplicating the mapping.
 */

/** Icon for a non-image attachment, chosen from its type / file extension. */
export function FileIcon({ name, type }: { name: string; type: Attachment['type'] }) {
  if (type === 'video') return <MovieOutlinedIcon color="action" />;
  const ext = name.split('.').pop()?.toLowerCase() ?? '';
  if (ext === 'pdf') return <PictureAsPdfOutlinedIcon color="action" />;
  if (['doc', 'docx', 'rtf', 'odt', 'txt'].includes(ext)) return <DescriptionOutlinedIcon color="action" />;
  if (['xls', 'xlsx', 'csv', 'ods'].includes(ext)) return <TableChartOutlinedIcon color="action" />;
  if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext)) return <FolderZipOutlinedIcon color="action" />;
  if (['mp4', 'mov', 'avi', 'mkv', 'webm'].includes(ext)) return <MovieOutlinedIcon color="action" />;
  if (['mp3', 'wav', 'ogg', 'm4a', 'flac'].includes(ext)) return <AudiotrackOutlinedIcon color="action" />;
  return <InsertDriveFileOutlinedIcon color="action" />;
}

/** A downloadable document/video row: icon + filename, click to download. */
export function FileLink({ attachment }: { attachment: Attachment }) {
  return (
    <MuiLink
      href={attachment.url}
      download={attachment.name}
      target="_blank"
      rel="noopener noreferrer"
      underline="hover"
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        p: 1,
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        color: 'text.primary',
      }}
    >
      <FileIcon name={attachment.name} type={attachment.type} />
      <Typography variant="body2" sx={{ wordBreak: 'break-all' }}>
        {attachment.name}
      </Typography>
    </MuiLink>
  );
}
