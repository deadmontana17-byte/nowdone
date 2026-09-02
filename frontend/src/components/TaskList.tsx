import { Box, Typography } from '@mui/material';
import { AnimatePresence } from 'framer-motion';

import type { Task, TaskType } from '@/types';
import { TaskItem } from '@/components/TaskItem';
import { toISODate } from '@/utils/datetime';

interface TaskListProps {
  tasks: Task[];
  taskTypes: TaskType[];
  onOpenDetail: (task: Task) => void;
}

function formatDateHeading(dateStr: string): string {
  // Compare as local "YYYY-MM-DD" strings so "Сегодня"/"Завтра" track the
  // user's local midnight, not the browser's UTC date.
  const now = new Date();
  const todayStr = toISODate(now);
  const tomorrow = new Date(now);
  tomorrow.setDate(now.getDate() + 1);
  const tomorrowStr = toISODate(tomorrow);

  // Parse the group's date in local time for the "1 сентября" style label.
  const [y, m, d] = dateStr.split('-').map(Number);
  const localDate = new Date(y, m - 1, d);
  const dayMonth = localDate.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });

  if (dateStr === todayStr) return `Сегодня, ${dayMonth}`;
  if (dateStr === tomorrowStr) return `Завтра, ${dayMonth}`;
  return localDate.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', weekday: 'long' });
}

/** Groups tasks by date for the planner view, per the task-management spec. */
export function TaskList({ tasks, taskTypes, onOpenDetail }: TaskListProps) {
  const grouped = new Map<string, Task[]>();
  for (const task of tasks) {
    const existing = grouped.get(task.date) ?? [];
    existing.push(task);
    grouped.set(task.date, existing);
  }
  const dates = Array.from(grouped.keys()).sort();

  if (dates.length === 0) {
    return (
      <Box sx={{ textAlign: 'center', mt: 6 }}>
        <Typography color="text.secondary">Задач пока нет — добавьте первую!</Typography>
      </Box>
    );
  }

  return (
    <Box>
      {dates.map((date) => (
        <Box key={date} sx={{ mb: 3 }}>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1, textTransform: 'capitalize' }}>
            {formatDateHeading(date)}
          </Typography>
          <AnimatePresence initial={false}>
            {grouped.get(date)!.map((task) => (
              <TaskItem
                key={task.id}
                task={task}
                taskType={taskTypes.find((t) => t.id === task.type_id) ?? undefined}
                onOpenDetail={onOpenDetail}
              />
            ))}
          </AnimatePresence>
        </Box>
      ))}
    </Box>
  );
}
