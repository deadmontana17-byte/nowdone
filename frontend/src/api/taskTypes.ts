import { apiRequest } from './client';
import type { TaskType } from '@/types';

export function fetchTaskTypes() {
  return apiRequest<{ task_types: TaskType[] }>('/task-types');
}

export function createTaskType(emoji: string, name: string) {
  return apiRequest<{ task_type: TaskType }>('/task-types', { method: 'POST', body: { emoji, name } });
}

export function deleteTaskType(id: string) {
  return apiRequest<{ ok: boolean }>(`/task-types/${id}`, { method: 'DELETE' });
}
