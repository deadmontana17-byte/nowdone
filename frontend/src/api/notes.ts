import { apiRequest } from './client';
import type { Note } from '@/types';

export function fetchNotes(unlocked: boolean) {
  return apiRequest<{ notes: Note[] }>(`/notes?unlocked=${unlocked}`);
}

export interface NoteInput {
  title: string;
  content?: Record<string, unknown>;
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
