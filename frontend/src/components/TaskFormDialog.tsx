import { useEffect, useRef, useState } from 'react';
import {
  Dialog, DialogTitle, DialogContent, DialogActions, TextField, Button, MenuItem,
  FormControlLabel, Switch, Stack, Box, Typography,
} from '@mui/material';

import type { Attachment, ChecklistItem, RecurrenceRule, Task, TaskType } from '@/types';
import { AttachmentUploader } from '@/components/AttachmentUploader';
import { ChecklistEditor } from '@/components/ChecklistEditor';
import { useCreateTask, useUpdateTask } from '@/hooks/useTasks';
import { useAuthStore } from '@/store/authStore';
import { apiDeleteUploads } from '@/api/client';
import { attachmentKeyFromUrl } from '@/utils/attachments';
import { isoToLocalInput } from '@/utils/datetime';
import {
  normalizeDescription, descriptionParagraphText, descriptionChecklistItems, buildDescription,
} from '@/utils/description';

interface TaskFormDialogProps {
  open: boolean;
  onClose: () => void;
  task: Task | null; // null = create mode
  defaultDate: string;
  taskTypes: TaskType[];
}

const FREQUENCIES = [
  { value: 'daily', label: 'Каждый день' },
  { value: 'weekdays', label: 'По будням (Пн–Пт)' },
  { value: 'weekly', label: 'Каждую неделю' },
  { value: 'monthly', label: 'Каждый месяц' },
  { value: 'yearly', label: 'Ежегодно' },
];

/** Create/edit dialog: title, rich-text description + interactive checklist,
 * attachments (images/video via S3 upload), reminder time and recurrence rule. */
export function TaskFormDialog({ open, onClose, task, defaultDate, taskTypes }: TaskFormDialogProps) {
  const createTask = useCreateTask();
  const updateTask = useUpdateTask();
  const timezone = useAuthStore((s) => s.user?.timezone) ?? 'UTC';

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [checklist, setChecklist] = useState<ChecklistItem[]>([]);
  const [typeId, setTypeId] = useState<string>('');
  const [date, setDate] = useState(defaultDate);
  const [reminderTime, setReminderTime] = useState('');
  const [isRecurring, setIsRecurring] = useState(false);
  const [frequency, setFrequency] = useState<RecurrenceRule['frequency']>('daily');
  const [attachments, setAttachments] = useState<Attachment[]>([]);

  // S3 keys uploaded during this dialog session. If the user cancels a *new*
  // task, these files were never saved anywhere, so we delete them from S3.
  const sessionUploadKeys = useRef<Set<string>>(new Set());

  useEffect(() => {
    sessionUploadKeys.current.clear();
    if (task) {
      const desc = normalizeDescription(task.description);
      setTitle(task.title);
      setDescription(descriptionParagraphText(desc));
      setChecklist(descriptionChecklistItems(desc));
      setTypeId(task.type_id ?? '');
      setDate(task.date);
      setReminderTime(task.reminder_time ? isoToLocalInput(task.reminder_time, timezone) : '');
      setIsRecurring(task.is_recurring);
      setFrequency(task.recurrence_rule?.frequency ?? 'daily');
      setAttachments(task.attachments ?? []);
    } else {
      setTitle('');
      setDescription('');
      setChecklist([]);
      setTypeId('');
      setDate(defaultDate);
      setReminderTime('');
      setIsRecurring(false);
      setFrequency('daily');
      setAttachments([]);
    }
  }, [task, defaultDate, open, timezone]);

  // Remember every uploaded file's key so a cancelled "new task" can clean them
  // up. Additionally, when a file is removed from the list via its "x" delete it
  // from S3 straight away — even before the task is saved (mirrors NoteDialog).
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

  // Closing without saving. For a new task, purge any files already uploaded to
  // S3 (best-effort). Editing an existing task never deletes here — that path
  // already reconciles attachments on save.
  function handleClose() {
    const keys = [...sessionUploadKeys.current];
    sessionUploadKeys.current.clear();
    if (!task && keys.length > 0) {
      apiDeleteUploads(keys).catch(() => {});
    }
    onClose();
  }

  function handleSave() {
    if (!title.trim()) return;

    const payload = {
      title: title.trim(),
      description: buildDescription(description, checklist),
      attachments,
      date,
      type_id: typeId || null,
      // Send the raw wall-clock value from <input type="datetime-local">
      // ("2026-09-01T18:30"); the backend interprets it in the user's timezone.
      reminder_time: reminderTime || null,
      is_recurring: isRecurring,
      recurrence_rule: isRecurring ? { frequency } : null,
    };

    // Saved successfully: the files are now referenced by a task, so drop them
    // from the cleanup set before closing.
    const onSaved = () => {
      sessionUploadKeys.current.clear();
      onClose();
    };

    if (task) {
      // clear_reminder makes "remove the time and save" actually clear it.
      updateTask.mutate(
        { id: task.id, input: { ...payload, clear_reminder: !reminderTime } },
        { onSuccess: onSaved },
      );
    } else {
      createTask.mutate(payload, { onSuccess: onSaved });
    }
  }

  return (
    <Dialog open={open} onClose={handleClose} fullWidth maxWidth="sm">
      <DialogTitle>{task ? 'Редактировать задачу' : 'Новая задача'}</DialogTitle>
      <DialogContent>
        <Stack spacing={2.5} sx={{ mt: 1 }}>
          <TextField label="Название" value={title} onChange={(e) => setTitle(e.target.value)} fullWidth autoFocus />

          {/* No placeholder — just the "Описание" label; the field opens empty. */}
          <TextField
            label="Описание"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            multiline
            minRows={3}
            fullWidth
          />

          <ChecklistEditor items={checklist} onChange={setChecklist} />

          <Box>
            <Typography variant="caption" color="text.secondary">Вложения</Typography>
            <AttachmentUploader attachments={attachments} onChange={handleAttachmentsChange} />
          </Box>

          <TextField select label="Тип задачи" value={typeId} onChange={(e) => setTypeId(e.target.value)} fullWidth>
            <MenuItem value="">Без типа</MenuItem>
            {taskTypes.map((t) => (
              <MenuItem key={t.id} value={t.id}>
                {t.emoji} {t.name}
              </MenuItem>
            ))}
          </TextField>

          <TextField
            label="Дата"
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            InputLabelProps={{ shrink: true }}
            fullWidth
          />

          <TextField
            label="Напоминание"
            type="datetime-local"
            value={reminderTime}
            onChange={(e) => setReminderTime(e.target.value)}
            InputLabelProps={{ shrink: true }}
            fullWidth
          />

          <FormControlLabel
            control={<Switch checked={isRecurring} onChange={(e) => setIsRecurring(e.target.checked)} />}
            label="Повторяющаяся задача"
          />

          {isRecurring && (
            <TextField
              select
              label="Периодичность"
              value={frequency}
              onChange={(e) => setFrequency(e.target.value as RecurrenceRule['frequency'])}
              fullWidth
            >
              {FREQUENCIES.map((f) => (
                <MenuItem key={f.value} value={f.value}>{f.label}</MenuItem>
              ))}
            </TextField>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>Отмена</Button>
        <Button variant="contained" onClick={handleSave} disabled={!title.trim()}>
          Сохранить
        </Button>
      </DialogActions>
    </Dialog>
  );
}
