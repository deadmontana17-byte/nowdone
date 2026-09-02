import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createTaskType, deleteTaskType, fetchTaskTypes } from '@/api/taskTypes';
import { ApiError } from '@/api/client';
import { useUiStore } from '@/store/uiStore';

export function useTaskTypes() {
  return useQuery({
    queryKey: ['task-types'],
    queryFn: fetchTaskTypes,
    select: (data) => data.task_types,
    // Task types change rarely; don't refetch them on every window focus.
    staleTime: 5 * 60_000,
  });
}

export function useCreateTaskType() {
  const qc = useQueryClient();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  return useMutation({
    mutationFn: ({ emoji, name }: { emoji: string; name: string }) => createTaskType(emoji, name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['task-types'] });
      showSnackbar('Тип задачи создан', 'success');
    },
    onError: (err) => showSnackbar(err instanceof ApiError ? err.message : 'Не удалось создать тип задачи', 'error'),
  });
}

export function useDeleteTaskType() {
  const qc = useQueryClient();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  return useMutation({
    mutationFn: (id: string) => deleteTaskType(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['task-types'] });
      showSnackbar('Тип задачи удалён', 'success');
    },
    onError: (err) => showSnackbar(err instanceof ApiError ? err.message : 'Не удалось удалить тип задачи', 'error'),
  });
}
