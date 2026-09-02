import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createNote, deleteNote, fetchNotes, updateNote, type NoteInput } from '@/api/notes';
import { ApiError } from '@/api/client';
import { useUiStore } from '@/store/uiStore';
import { useAuthStore } from '@/store/authStore';

export function useNotes() {
  const isPinUnlocked = useAuthStore((s) => s.isPinUnlocked);

  return useQuery({
    queryKey: ['notes', isPinUnlocked],
    queryFn: () => fetchNotes(isPinUnlocked),
    select: (data) => data.notes,
    // Same rationale as tasks: pick up notes added via the Telegram bot without
    // a manual reload. Paused while the tab is hidden.
    refetchInterval: 60_000,
  });
}

export function useCreateNote() {
  const qc = useQueryClient();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  return useMutation({
    mutationFn: (input: NoteInput) => createNote(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notes'] });
      showSnackbar('Заметка создана', 'success');
    },
    onError: (err) => showSnackbar(err instanceof ApiError ? err.message : 'Не удалось создать заметку', 'error'),
  });
}

export function useUpdateNote() {
  const qc = useQueryClient();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: NoteInput }) => updateNote(id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notes'] });
      showSnackbar('Заметка обновлена', 'success');
    },
    onError: (err) => showSnackbar(err instanceof ApiError ? err.message : 'Не удалось обновить заметку', 'error'),
  });
}

export function useDeleteNote() {
  const qc = useQueryClient();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  return useMutation({
    mutationFn: (id: string) => deleteNote(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notes'] });
      showSnackbar('Заметка удалена', 'success');
    },
    onError: (err) => showSnackbar(err instanceof ApiError ? err.message : 'Не удалось удалить заметку', 'error'),
  });
}
