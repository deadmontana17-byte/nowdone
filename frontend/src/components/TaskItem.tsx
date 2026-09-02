import { useState } from 'react';
import { Box, Checkbox, IconButton, ListItem, ListItemText, Chip, Typography } from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import AlarmIcon from '@mui/icons-material/Alarm';
import RepeatIcon from '@mui/icons-material/Repeat';
import ChecklistRtlIcon from '@mui/icons-material/ChecklistRtl';
import { AnimatePresence, motion } from 'framer-motion';

import type { Task, TaskType } from '@/types';
import { CheckboxBurst } from '@/components/CheckboxBurst';
import { randomBurstVariant, type BurstVariant } from '@/utils/burst';
import { useUpdateTask, useDeleteTask } from '@/hooks/useTasks';
import { useAuthStore } from '@/store/authStore';
import { isoToLocalTime } from '@/utils/datetime';
import { normalizeDescription, descriptionChecklistItems } from '@/utils/description';

interface TaskItemProps {
  task: Task;
  taskType?: TaskType;
  onOpenDetail: (task: Task) => void;
}

const RECURRENCE_LABELS: Record<NonNullable<Task['recurrence_rule']>['frequency'], string> = {
  daily: 'Каждый день',
  weekdays: 'По будням',
  weekly: 'Каждую неделю',
  monthly: 'Каждый месяц',
  yearly: 'Ежегодно',
};

function recurrenceLabel(task: Task): string {
  const freq = task.recurrence_rule?.frequency;
  return (freq && RECURRENCE_LABELS[freq]) || 'Повтор';
}

export function TaskItem({ task, taskType, onOpenDetail }: TaskItemProps) {
  const updateTask = useUpdateTask();
  const deleteTask = useDeleteTask();
  const timezone = useAuthStore((s) => s.user?.timezone) ?? 'UTC';
  const [burst, setBurst] = useState<{ key: number; variant: BurstVariant } | null>(null);

  const checklist = descriptionChecklistItems(normalizeDescription(task.description));
  const checklistDone = checklist.filter((i) => i.done).length;

  function handleToggle() {
    const nextDone = !task.is_done;
    updateTask.mutate({ id: task.id, input: { is_done: nextDone } });
    if (nextDone) setBurst({ key: Date.now(), variant: randomBurstVariant() });
  }

  function handleDelete() {
    // Optimistic removal lives in useDeleteTask; AnimatePresence plays the exit.
    deleteTask.mutate(task.id);
  }

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, x: -24, transition: { duration: 0.18 } }}
      style={{ position: 'relative' }}
    >
      <ListItem
        onClick={() => onOpenDetail(task)}
        sx={{
          borderRadius: 2,
          mb: 1,
          bgcolor: 'background.paper',
          cursor: 'pointer',
          opacity: task.is_done ? 0.7 : 1,
        }}
        secondaryAction={
          <IconButton edge="end" onClick={(e) => { e.stopPropagation(); handleDelete(); }} aria-label="Удалить задачу">
            <DeleteOutlineIcon fontSize="small" />
          </IconButton>
        }
      >
        <Box sx={{ position: 'relative', display: 'inline-flex', mr: 1 }}>
          <motion.div
            key={burst?.key ?? 'idle'}
            initial={false}
            animate={{ scale: burst ? [1, 1.3, 0.9, 1] : 1 }}
            transition={{ duration: 0.45, ease: 'easeOut' }}
          >
            <Checkbox
              checked={task.is_done}
              onClick={(e) => e.stopPropagation()}
              onChange={handleToggle}
              sx={{ p: 0 }}
            />
          </motion.div>
          <AnimatePresence>
            {burst && (
              <CheckboxBurst key={burst.key} variant={burst.variant} onDone={() => setBurst(null)} />
            )}
          </AnimatePresence>
        </Box>

        <ListItemText
          sx={{ minWidth: 0 }} // allow the title to shrink/ellipsize instead of pushing the delete button off-screen
          primary={
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, minWidth: 0 }}>
              {taskType && <Box component="span" sx={{ flexShrink: 0 }}>{taskType.emoji}</Box>}
              {/* Completed = muted row (opacity) + dimmed title. No extra icon.
                  noWrap → one line with textOverflow: ellipsis. */}
              <Typography noWrap sx={{ color: task.is_done ? 'text.disabled' : 'text.primary', minWidth: 0 }}>
                {task.title}
              </Typography>
            </Box>
          }
          secondary={
            <Box sx={{ display: 'flex', gap: 1, mt: 0.5, flexWrap: 'wrap' }}>
              {task.reminder_time && (
                <Chip size="small" icon={<AlarmIcon />} label={isoToLocalTime(task.reminder_time, timezone)} />
              )}
              {checklist.length > 0 && (
                <Chip size="small" icon={<ChecklistRtlIcon />} label={`${checklistDone}/${checklist.length}`} />
              )}
              {task.is_recurring && <Chip size="small" icon={<RepeatIcon />} label={recurrenceLabel(task)} />}
            </Box>
          }
        />
      </ListItem>
    </motion.div>
  );
}
