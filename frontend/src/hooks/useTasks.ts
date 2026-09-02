import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createTask, deleteTask, fetchTasks, updateTask, type CreateTaskInput, type UpdateTaskInput } from '@/api/tasks';
import { ApiError } from '@/api/client';
import { useUiStore } from '@/store/uiStore';
import type { Task } from '@/types';

type TasksCache = { tasks: Task[] };

export function useTasks(from: string, to: string) {
  return useQuery({
    queryKey: ['tasks', from, to],
    queryFn: () => fetchTasks(from, to),
    select: (data) => data.tasks,
    // Poll while the tab is visible so tasks created via the Telegram bot (or on
    // another device) show up within a minute even if the user never leaves the
    // page. React Query pauses this automatically when the tab is hidden.
    refetchInterval: 60_000,
  });
}

export function useCreateTask() {
  const qc = useQueryClient();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  return useMutation({
    mutationFn: (input: CreateTaskInput) => createTask(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['tasks'] });
      showSnackbar('Задача создана', 'success');
    },
    onError: (err) => showSnackbar(err instanceof ApiError ? err.message : 'Не удалось создать задачу', 'error'),
  });
}

/** Turn a partial update payload into a patch applicable to a cached Task. */
function inputToPatch(input: UpdateTaskInput): Partial<Task> {
  const { clear_reminder, clear_type_id, ...rest } = input;
  const patch = { ...rest } as Partial<Task>;
  if (clear_reminder) patch.reminder_time = null;
  if (clear_type_id) patch.type_id = null;
  return patch;
}

export function useUpdateTask() {
  const qc = useQueryClient();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateTaskInput }) => updateTask(id, input),
    // Apply the change to every cached task list immediately so the checkbox,
    // completed styling and checklist toggles feel instant.
    onMutate: async ({ id, input }) => {
      await qc.cancelQueries({ queryKey: ['tasks'] });
      const snapshots = qc.getQueriesData<TasksCache>({ queryKey: ['tasks'] });
      const patch = inputToPatch(input);
      snapshots.forEach(([key, data]) => {
        if (!data) return;
        qc.setQueryData<TasksCache>(key, {
          ...data,
          tasks: data.tasks.map((t) => (t.id === id ? { ...t, ...patch } : t)),
        });
      });
      return { snapshots };
    },
    onError: (err, _vars, ctx) => {
      ctx?.snapshots.forEach(([key, data]) => qc.setQueryData(key, data));
      showSnackbar(err instanceof ApiError ? err.message : 'Не удалось обновить задачу', 'error');
    },
    // Reconcile with the server (recurrence spawn, normalized reminder_time…).
    onSettled: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  });
}

export function useDeleteTask() {
  const qc = useQueryClient();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  return useMutation({
    mutationFn: (id: string) => deleteTask(id),
    // Remove the task from every cached list right away and keep it gone unless
    // the request actually fails — no fade-out flicker, no reappearing row.
    onMutate: async (id: string) => {
      await qc.cancelQueries({ queryKey: ['tasks'] });
      const snapshots = qc.getQueriesData<TasksCache>({ queryKey: ['tasks'] });
      snapshots.forEach(([key, data]) => {
        if (!data) return;
        qc.setQueryData<TasksCache>(key, { ...data, tasks: data.tasks.filter((t) => t.id !== id) });
      });
      return { snapshots };
    },
    onError: (err, _id, ctx) => {
      ctx?.snapshots.forEach(([key, data]) => qc.setQueryData(key, data));
      showSnackbar(err instanceof ApiError ? err.message : 'Не удалось удалить задачу', 'error');
    },
    onSuccess: () => showSnackbar('Задача удалена', 'success'),
    onSettled: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  });
}
