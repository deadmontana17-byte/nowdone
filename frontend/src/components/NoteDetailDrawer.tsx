import { useState } from 'react';
import { Box, Button, Drawer, Typography, Divider, Stack, IconButton } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import EditOutlinedIcon from '@mui/icons-material/EditOutlined';

import type { Note } from '@/types';
import { RichText } from '@/utils/richText';
import { FileLink } from '@/utils/fileIcons';
import { ImageLightbox } from '@/components/ImageLightbox';

interface NoteDetailDrawerProps {
  note: Note | null;
  onClose: () => void;
  onEdit: (note: Note) => void;
}

/** Pull the plain-text body out of the note's JSONB `content`. Legacy notes and
 * new ones both store `{ text: string }`. */
function contentToText(content: Record<string, unknown> | undefined): string {
  return (content as { text?: string } | undefined)?.text ?? '';
}

/**
 * Read-only note card in a right-hand Drawer — the notes counterpart of
 * TaskDetailDrawer.
 *
 * Improvements ported from the task card:
 *  - Opening the card triggers NO backend request: every field is read from the
 *    `note` object already held in the React Query cache.
 *  - Description links (http/https) are clickable via <RichText> (spec 1d).
 *  - Images use loading="lazy" / decoding="async" and open full-screen in the
 *    lightbox; documents/video render as a downloadable icon + filename row,
 *    reusing the shared <FileLink> (spec 1a/1b).
 *  - Editing is reachable only through the big "Редактировать" button pinned to
 *    the bottom of the card — identical to TaskDetailDrawer. A click on an
 *    unlocked note in the list opens this read-only card, not the editor.
 */
export function NoteDetailDrawer({ note, onClose, onEdit }: NoteDetailDrawerProps) {
  const [lightbox, setLightbox] = useState<{ url: string; name: string } | null>(null);

  const text = note ? contentToText(note.content) : '';
  const attachments = note?.attachments ?? [];
  const hasText = text.trim() !== '';

  return (
    <>
      <Drawer
        anchor="right"
        open={Boolean(note)}
        onClose={onClose}
        PaperProps={{ sx: { width: { xs: '100vw', sm: 440 }, p: 2 } }}
      >
        {note && (
          <Stack spacing={2} sx={{ height: '100%' }}>
            <Stack direction="row" alignItems="flex-start" justifyContent="space-between" spacing={1}>
              <Typography variant="h6" sx={{ wordBreak: 'break-word', flex: 1 }}>
                {note.title}
              </Typography>
              <IconButton onClick={onClose} aria-label="Закрыть" size="small">
                <CloseIcon />
              </IconButton>
            </Stack>

            <Box sx={{ flex: 1, overflowY: 'auto' }}>
              <Stack spacing={2}>
                {hasText && (
                  <Box>
                    <Typography variant="caption" color="text.secondary">Содержание</Typography>
                    <Box sx={{ mt: 0.5 }}>
                      {/* Same renderer as tasks: clickable links, bullets, emphasis. */}
                      <RichText text={text} />
                    </Box>
                  </Box>
                )}

                {attachments.length > 0 && (
                  <>
                    <Divider />
                    <Box>
                      <Typography variant="caption" color="text.secondary">Вложения</Typography>
                      <Stack spacing={1} sx={{ mt: 0.5 }}>
                        {attachments.map((a, i) =>
                          a.type === 'image' ? (
                            <Box
                              key={i}
                              component="img"
                              src={a.url}
                              alt={a.name}
                              loading="lazy"
                              decoding="async"
                              onClick={() => setLightbox({ url: a.url, name: a.name })}
                              sx={{ width: '100%', maxHeight: 320, objectFit: 'cover', borderRadius: 2, cursor: 'zoom-in' }}
                            />
                          ) : (
                            // Documents, archives and video: icon + filename, click to download.
                            <FileLink key={i} attachment={a} />
                          ),
                        )}
                      </Stack>
                    </Box>
                  </>
                )}

                {!hasText && attachments.length === 0 && (
                  <Typography variant="body2" color="text.secondary">Пустая заметка</Typography>
                )}
              </Stack>
            </Box>

            {/* Same as TaskDetailDrawer: a full-width edit button pinned to the
                bottom of the card, always visible. */}
            <Button
              fullWidth
              variant="contained"
              startIcon={<EditOutlinedIcon />}
              onClick={() => onEdit(note)}
              sx={{ flexShrink: 0 }}
            >
              Редактировать
            </Button>
          </Stack>
        )}
      </Drawer>

      <ImageLightbox
        src={lightbox?.url ?? null}
        alt={lightbox?.name}
        onClose={() => setLightbox(null)}
      />
    </>
  );
}
