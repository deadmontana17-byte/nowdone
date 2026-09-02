import { useEffect, useMemo, useState } from 'react';
import { Box, Button, Fab, IconButton, Stack, Typography } from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import CategoryIcon from '@mui/icons-material/Category';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';

import { TaskList } from '@/components/TaskList';
import { TaskFormDialog } from '@/components/TaskFormDialog';
import { TaskDetailDrawer } from '@/components/TaskDetailDrawer';
import { TaskTypeDialog } from '@/components/TaskTypeDialog';
import { MonthYearPickerDialog } from '@/components/MonthYearPickerDialog';
import { useTasks } from '@/hooks/useTasks';
import { useTaskTypes } from '@/hooks/useTaskTypes';
import { useUiStore } from '@/store/uiStore';
import { fromISODate, toISODate } from '@/utils/datetime';
import type { Task } from '@/types';

function monthRange(dateStr: string): { from: string; to: string } {
  // Parse/format in local time so the month boundaries don't slip a day for
  // users east/west of UTC.
  const date = fromISODate(dateStr);
  const from = new Date(date.getFullYear(), date.getMonth(), 1);
  const to = new Date(date.getFullYear(), date.getMonth() + 1, 0);
  return { from: toISODate(from), to: toISODate(to) };
}

export function HomePage() {
  const { selectedDate, setSelectedDate } = useUiStore();
  const { from, to } = useMemo(() => monthRange(selectedDate), [selectedDate]);

  const { data: tasks = [], isLoading } = useTasks(from, to);
  const { data: taskTypes = [] } = useTaskTypes();

  const [formOpen, setFormOpen] = useState(false);
  const [typesOpen, setTypesOpen] = useState(false);
  const [monthPickerOpen, setMonthPickerOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);

  // Track the viewed task by id so the detail Drawer always shows live data
  // (checklist toggles, edits) and auto-closes if the task is deleted.
  const [detailId, setDetailId] = useState<string | null>(null);
  const detailTask = tasks.find((t) => t.id === detailId) ?? null;
  useEffect(() => {
    if (detailId && !isLoading && !detailTask) setDetailId(null);
  }, [detailId, detailTask, isLoading]);

  function shiftMonth(delta: number) {
    const date = fromISODate(selectedDate);
    date.setMonth(date.getMonth() + delta);
    setSelectedDate(toISODate(date));
  }

  function openCreate() {
    setEditingTask(null);
    setFormOpen(true);
  }

  function openEdit(task: Task) {
    setEditingTask(task);
    setFormOpen(true);
  }

  const monthLabel = fromISODate(selectedDate).toLocaleDateString('ru-RU', { month: 'long', year: 'numeric' });

  return (
    <Box>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
        <Stack direction="row" alignItems="center" spacing={0.5}>
          <IconButton onClick={() => shiftMonth(-1)}><ChevronLeftIcon /></IconButton>
          {/* Click the month/year label to jump to any month via the picker. */}
          <Typography
            component="button"
            onClick={() => setMonthPickerOpen(true)}
            aria-label="Выбрать месяц и год"
            sx={{
              textTransform: 'capitalize',
              minWidth: 140,
              textAlign: 'center',
              cursor: 'pointer',
              border: 0,
              background: 'none',
              font: 'inherit',
              color: 'inherit',
              borderRadius: 1,
              py: 0.5,
              '&:hover': { bgcolor: 'action.hover' },
            }}
          >
            {monthLabel}
          </Typography>
          <IconButton onClick={() => shiftMonth(1)}><ChevronRightIcon /></IconButton>
        </Stack>
        {/* Purple icon + a white "Типы" label so it's clearly a control that
            opens task-type management. Label weight matches the task titles
            (regular, 400), overriding the theme's bold (600) buttons. */}
        <Button
          onClick={() => setTypesOpen(true)}
          startIcon={<CategoryIcon sx={{ color: 'primary.main', fontSize: '1.6rem' }} />}
          size="small"
          aria-label="Управление типами задач"
          sx={{ color: 'common.white', fontSize: '1rem', fontWeight: 400 }}
        >
          Типы
        </Button>
      </Stack>

      {isLoading ? (
        <Typography color="text.secondary">Загрузка…</Typography>
      ) : (
        <TaskList tasks={tasks} taskTypes={taskTypes} onOpenDetail={(t) => setDetailId(t.id)} />
      )}

      <Fab color="primary" onClick={openCreate} sx={{ position: 'fixed', bottom: 80, right: 24 }} aria-label="Добавить задачу">
        <AddIcon />
      </Fab>

      <TaskDetailDrawer
        task={detailTask}
        taskType={taskTypes.find((t) => t.id === detailTask?.type_id) ?? undefined}
        onClose={() => setDetailId(null)}
        onEdit={(t) => { setDetailId(null); openEdit(t); }}
      />

      <TaskFormDialog
        open={formOpen}
        onClose={() => setFormOpen(false)}
        task={editingTask}
        defaultDate={selectedDate}
        taskTypes={taskTypes}
      />
      <TaskTypeDialog open={typesOpen} onClose={() => setTypesOpen(false)} />

      <MonthYearPickerDialog
        open={monthPickerOpen}
        onClose={() => setMonthPickerOpen(false)}
        value={selectedDate}
        onSelect={setSelectedDate}
      />
    </Box>
  );
}
