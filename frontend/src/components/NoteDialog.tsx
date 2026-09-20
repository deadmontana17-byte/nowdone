import { useEffect, useRef, useState } from 'react';
import { Dialog, DialogTitle, DialogContent, DialogActions, TextField, Button, Stack, FormControlLabel, Switch, Box, Typography } from '@mui/material';

import type { Attachment, ChecklistItem, Note } from '@/types';
import { AttachmentUploader } from '@/components/AttachmentUploader';
import { ChecklistEditor } from '@/components/ChecklistEditor';
import { useCreateNote, useUpdateNote } from '@/hooks/useNotes';
import { apiDeleteUploads } from '@/api/client';
import { attachmentKeyFromUrl } from '@/utils/attachments';
import {
  normalizeDescription, descriptionParagraphText, descriptionChecklistItems, buildDescription,
} from '@/utils/description';

interface NoteDialogProps {
  open: boolean;
  onClose: () => void;
  note: Note | null;
}

export function NoteDialog({ open, onClose, note }: NoteDialogProps) {
  const createNote = useCreateNote();
  const updateNote = useUpdateNote();

  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [checklist, setChecklist] = useState<ChecklistItem[]>([]);
  const [isHidden, setIsHidden] = useState(false);
  const [attachments, setAttachments] = useState<Attachment[]>([]);

  // S3 keys uploaded while this dialog was open. Restored from Changes.txt item 3:
  // this cleanup set had been removed, and without it files uploaded into a *new*
  // note that is then cancelled stay orphaned in the bucket. If the user closes
  // the dialog without saving a new note, every key collected here is deleted.
  const sessionUploadKeys = useRef<Set<string>>(new Set());

  useEffect(() => {
    sessionUploadKeys.current.clear();
    if (note) {
      const desc = normalizeDescription(note.content);
      setTitle(note.title);
      setContent(descriptionParagraphText(desc));
      setChecklist(descriptionChecklistItems(desc));
      setIsHidden(note.is_hidden);
      setAttachments(note.attachments ?? []);
    } else {
      setTitle('');
      setContent('');
      setChecklist([]);
      setIsHidden(false);
      setAttachments([]);
    }
  }, [note, open]);

  // Track newly uploaded keys, and when a file is removed from the list via its
  // "x" delete it from S3 immediately — before the note is saved. This covers
  // both a brand-new note and editing an existing one (remove-before-save).
  function handleAttachmentsChange(next: Attachment[]) {
    next.forEach((a) => {
      const key = attachmentKeyFromUrl(a.url);
      if (key) sessionUploadKeys.current.add(key);
    });

    const nextUrls = new Set(next.map((a) => a.url));
    const removedKeys = attachments
      .filter((a) => !nextUrls.has(a.url))
      .map((a) => attachmentKeyFromUrl(a.url))
      .filter((k): k is string => Boolean(k));
    if (removedKeys.length > 0) {
      apiDeleteUploads(removedKeys).catch(() => {});
      removedKeys.forEach((k) => sessionUploadKeys.current.delete(k));
    }

    setAttachments(next);
  }

  // Closing without saving. For a new note, purge any files already uploaded to
  // S3 (best-effort). Editing an existing note never deletes its stored files
  // here — only files explicitly removed from the list (handled above) go.
  function handleClose() {
    const keys = [...sessionUploadKeys.current];
    sessionUploadKeys.current.clear();
    if (!note && keys.length > 0) {
      apiDeleteUploads(keys).catch(() => {});
    }
    onClose();
  }

  function handleSave() {
    if (!title.trim()) return;
    const payload = { title: title.trim(), content: buildDescription(content, checklist), attachments, is_hidden: isHidden };

    // Saved successfully: the files are now referenced by the note, so drop them
    // from the cleanup set before closing.
    const onSaved = () => {
      sessionUploadKeys.current.clear();
      onClose();
    };

    if (note) {
      updateNote.mutate({ id: note.id, input: payload }, { onSuccess: onSaved });
    } else {
      createNote.mutate(payload, { onSuccess: onSaved });
    }
  }

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      fullWidth
      maxWidth="sm"
      // See TaskFormDialog: recompute the outlined TextField's border notch
      // after the open transition settles, so it never lands on a stale
      // mid-transition measurement (which shows as the label "crossed out").
      TransitionProps={{ onEntered: () => window.dispatchEvent(new Event('resize')) }}
    >
      <DialogTitle>{note ? 'Редактировать заметку' : 'Новая заметка'}</DialogTitle>
      <DialogContent
        // See TaskFormDialog: recompute the notch after every field focus
        // (delegated — React's synthetic onFocus bubbles), not just once on
        // the dialog's own opening transition.
        onFocus={() => window.setTimeout(() => window.dispatchEvent(new Event('resize')), 150)}
      >
        <Stack spacing={2.5} sx={{ mt: 1 }}>
          <TextField label="Заголовок" value={title} onChange={(e) => setTitle(e.target.value)} fullWidth />
          <TextField label="Содержание" value={content} onChange={(e) => setContent(e.target.value)} multiline minRows={5} fullWidth />
          <ChecklistEditor items={checklist} onChange={setChecklist} />
          <Box>
            <Typography variant="caption" color="text.secondary">Вложения</Typography>
            {/* Uploads go straight to S3 (shared AttachmentUploader). The dialog
                tracks the uploaded keys so it can clean up orphans on cancel and
                on per-file removal — see handleAttachmentsChange / handleClose. */}
            <AttachmentUploader attachments={attachments} onChange={handleAttachmentsChange} />
          </Box>
          <FormControlLabel
            control={<Switch checked={isHidden} onChange={(e) => setIsHidden(e.target.checked)} />}
            label="Скрытая заметка (открывается по PIN)"
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>Отмена</Button>
        <Button variant="contained" onClick={handleSave} disabled={!title.trim()}>Сохранить</Button>
      </DialogActions>
    </Dialog>
  );
}
