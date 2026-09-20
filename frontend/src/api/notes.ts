import { apiRequest } from './client';
import type { Note, TaskDescription } from '@/types';

export function fetchNotes(unlocked: boolean) {
  return apiRequest<{ notes: Note[] }>(`/notes?unlocked=${unlocked}`);
}

export interface NoteInput {
  title: string;
  // Structured { blocks } content (paragraph/checklist, same shape as task
  // descriptions) or a plain object for anything else.
  content?: TaskDescription | Record<string, unknown>;
  attachments?: Note['attachments'];
  is_hidden?: boolean;
}

export function createNote(input: NoteInput) {
  return apiRequest<{ note: Note }>('/notes', { method: 'POST', body: input });
}

export function updateNote(id: string, input: NoteInput) {
  return apiRequest<{ note: Note }>(`/notes/${id}`, { method: 'PATCH', body: input });
}

export function deleteNote(id: string) {
  return apiRequest<{ ok: boolean }>(`/notes/${id}`, { method: 'DELETE' });
}
