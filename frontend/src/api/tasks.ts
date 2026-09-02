import { apiRequest } from './client';
import type { StoredDescription, Task } from '@/types';

export function fetchTasks(from: string, to: string) {
  return apiRequest<{ tasks: Task[] }>(`/tasks?from=${from}&to=${to}`);
}

export interface CreateTaskInput {
  type_id?: string | null;
  title: string;
  description?: StoredDescription;
  attachments?: Task['attachments'];
  date: string;
  reminder_time?: string | null;
  is_recurring?: boolean;
  recurrence_rule?: Task['recurrence_rule'];
}

export function createTask(input: CreateTaskInput) {
  return apiRequest<{ task: Task }>('/tasks', { method: 'POST', body: input });
}

export interface UpdateTaskInput extends Partial<CreateTaskInput> {
  is_done?: boolean;
  clear_type_id?: boolean;
  clear_reminder?: boolean;
}

export function updateTask(id: string, input: UpdateTaskInput) {
  return apiRequest<{ task: Task }>(`/tasks/${id}`, { method: 'PATCH', body: input });
}

export function deleteTask(id: string) {
  return apiRequest<{ ok: boolean }>(`/tasks/${id}`, { method: 'DELETE' });
}
