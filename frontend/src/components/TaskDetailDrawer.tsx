import { useState } from 'react';
import {
  Box, Button, Drawer, Typography, Divider, Stack, IconButton,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import EditOutlinedIcon from '@mui/icons-material/EditOutlined';

import type { Task, TaskType } from '@/types';
import { useUpdateTask } from '@/hooks/useTasks';
import {
  normalizeDescription, descriptionParagraphText, descriptionChecklistItems, toggleChecklistItem,
} from '@/utils/description';
import { RichText } from '@/utils/richText';
// fileIcon/FileLink now live in a shared util so NoteDetailDrawer reuses them.
import { FileLink } from '@/utils/fileIcons';
import { ChecklistView } from '@/components/ChecklistView';
import { ImageLightbox } from '@/components/ImageLightbox';

interface TaskDetailDrawerProps {
  task: Task | null;
  taskType?: TaskType;
  onClose: () => void;
  onEdit: (task: Task) => void;
}

/** Read-only task card in a right-hand Drawer: title, type, rich description,
 * attachments (inline images + downloadable files) and an interactive
 * checklist. The "Редактировать" button opens the existing edit dialog. */
export function TaskDetailDrawer({ task, taskType, onClose, onEdit }: TaskDetailDrawerProps) {
  const updateTask = useUpdateTask();
  const [lightbox, setLightbox] = useState<{ url: string; name: string } | null>(null);

  // Read description live from the task so checklist toggles reflect instantly.
  const desc = task ? normalizeDescription(task.description) : { blocks: [] };
  const paragraph = descriptionParagraphText(desc);
  const checklist = descriptionChecklistItems(desc);
  const attachments = task?.attachments ?? [];

  function handleToggleChecklist(id: string, done: boolean) {
    if (!task) return;
    updateTask.mutate({ id: task.id, input: { description: toggleChecklistItem(desc, id, done) } });
  }

  return (
    <>
      <Drawer
        anchor="right"
        open={Boolean(task)}
        onClose={onClose}
        PaperProps={{ sx: { width: { xs: '100vw', sm: 440 }, p: 2 } }}
      >
        {task && (
          <Stack spacing={2} sx={{ height: '100%' }}>
            <Stack direction="row" alignItems="flex-start" justifyContent="space-between" spacing={1}>
              <Typography variant="h6" sx={{ wordBreak: 'break-word', flex: 1 }}>
                {task.title}
              </Typography>
              <IconButton onClick={onClose} aria-label="Закрыть" size="small">
                <CloseIcon />
              </IconButton>
            </Stack>

            {taskType && (
              <Typography variant="body2" color="text.secondary">
                {taskType.emoji} {taskType.name}
              </Typography>
            )}

            <Box sx={{ flex: 1, overflowY: 'auto' }}>
              <Stack spacing={2}>
                {paragraph && (
                  <Box>
                    <Typography variant="caption" color="text.secondary">Описание</Typography>
                    <Box sx={{ mt: 0.5 }}>
                      <RichText text={paragraph} />
                    </Box>
                  </Box>
                )}

                {checklist.length > 0 && (
                  <>
                    <Divider />
                    <ChecklistView items={checklist} onToggle={handleToggleChecklist} />
                  </>
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
                            <FileLink key={i} attachment={a} />
                          ),
                        )}
                      </Stack>
                    </Box>
                  </>
                )}

                {!paragraph && checklist.length === 0 && attachments.length === 0 && (
                  <Typography variant="body2" color="text.secondary">Нет описания</Typography>
                )}
              </Stack>
            </Box>

            {/* Edit stays pinned to the bottom of the card, always visible. */}
            <Button
              fullWidth
              variant="contained"
              startIcon={<EditOutlinedIcon />}
              onClick={() => onEdit(task)}
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
