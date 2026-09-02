import { useEffect, useState } from 'react';
import {
  Box, Fab, List, ListItemButton, ListItemText, IconButton, Typography,
  Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';

import { PinInput } from '@/components/PinInput';
import { NoteDialog } from '@/components/NoteDialog';
import { NoteDetailDrawer } from '@/components/NoteDetailDrawer';
import { useNotes, useDeleteNote } from '@/hooks/useNotes';
import { useUiStore } from '@/store/uiStore';
import { useNotesStore } from '@/store/notesStore';
import { verifyPin } from '@/api/auth';
import type { Note } from '@/types';

/** One-line title with an ellipsis; long unbroken words still wrap. */
const titleSx = {
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
  wordBreak: 'break-word',
} as const;

/** Description preview capped at two lines. */
const snippetSx = {
  display: '-webkit-box',
  WebkitLineClamp: 2,
  WebkitBoxOrient: 'vertical',
  overflow: 'hidden',
  wordBreak: 'break-word',
} as const;

/** Short preview of a note's body for the list row. */
function noteSnippet(content: Record<string, unknown> | undefined): string {
  const text = (content as { text?: string } | undefined)?.text ?? '';
  return text.replace(/\s+/g, ' ').trim();
}

/**
 * Notes page.
 *
 * PIN protection (unchanged): a hidden note stays gated until the correct PIN is
 * entered this session (notesStore.unlock()), and re-locks on unmount / reload.
 * What changed here is only presentation: a locked note now looks like a normal
 * row — its real title is shown — but with the description preview withheld.
 * Clicking a locked row still opens the PIN dialog; clicking an unlocked row
 * opens the read-only card (never the editor).
 */
export function NotesPage() {
  const { data: notes = [], isLoading } = useNotes();
  const showSnackbar = useUiStore((s) => s.showSnackbar);
  const isUnlocked = useNotesStore((s) => s.isUnlocked);
  const unlock = useNotesStore((s) => s.unlock);
  const lock = useNotesStore((s) => s.lock);
  const deleteNote = useDeleteNote();

  // Re-lock hidden notes when the user navigates away from the page.
  useEffect(() => {
    return () => lock();
  }, [lock]);

  const [pinDialogOpen, setPinDialogOpen] = useState(false);
  const [pinValue, setPinValue] = useState('');

  const [formOpen, setFormOpen] = useState(false);
  const [editingNote, setEditingNote] = useState<Note | null>(null);

  // Note pending deletion — drives the confirmation dialog.
  const [noteToDelete, setNoteToDelete] = useState<Note | null>(null);

  // Track the opened card by id so it always shows live data and closes itself
  // if the note gets deleted (mirrors HomePage's task drawer).
  const [detailId, setDetailId] = useState<string | null>(null);
  const detailNote = notes.find((n) => n.id === detailId) ?? null;
  useEffect(() => {
    if (detailId && !isLoading && !detailNote) setDetailId(null);
  }, [detailId, detailNote, isLoading]);

  function openPinDialog() {
    setPinValue('');
    setPinDialogOpen(true);
  }

  async function handlePinComplete(pin: string) {
    try {
      await verifyPin(pin);
      unlock(); // reveal every hidden note for this session
      setPinDialogOpen(false);
      setPinValue('');
    } catch {
      showSnackbar('Неверный PIN', 'error');
      setPinValue('');
    }
  }

  function openCreate() {
    setEditingNote(null);
    setFormOpen(true);
  }

  function openEditFromCard(note: Note) {
    setDetailId(null);
    setEditingNote(note);
    setFormOpen(true);
  }

  function confirmDelete() {
    if (!noteToDelete) return;
    deleteNote.mutate(noteToDelete.id, { onSettled: () => setNoteToDelete(null) });
  }

  return (
    <Box>
      <Typography variant="h6" sx={{ mb: 2 }}>Заметки</Typography>

      <List>
        {notes.map((note) => {
          // A hidden note is "locked" until the PIN is entered this session.
          const locked = note.is_hidden && !isUnlocked;

          return (
            <ListItemButton
              key={note.id}
              onClick={() => (locked ? openPinDialog() : setDetailId(note.id))}
              sx={{ borderRadius: 2, mb: 1, bgcolor: 'background.paper' }}
            >
              <ListItemText
                sx={{ minWidth: 0, pr: 1 }}
                primary={
                  <>
                    {/* Padlock marker for a PIN-protected note. Driven by the raw
                        `is_hidden` flag, not `locked`, so the icon stays visible
                        even after the note has been unlocked this session. */}
                    {note.is_hidden && (
                      <LockOutlinedIcon
                        fontSize="small"
                        aria-label="Защищено PIN-кодом"
                        sx={{ fontSize: 16, flexShrink: 0, color: 'text.secondary' }}
                      />
                    )}
                    <Box component="span" sx={{ ...titleSx, minWidth: 0 }}>{note.title}</Box>
                  </>
                }
                // Render the primary line as a flex row so the lock icon sits
                // inline with the title without breaking the ellipsis.
                primaryTypographyProps={{ component: 'div', sx: { display: 'flex', alignItems: 'center', gap: 0.5, minWidth: 0 } }}
                // Locked notes show the title only — the description stays hidden
                // until the note is unlocked with the PIN.
                secondary={locked ? null : (noteSnippet(note.content) || new Date(note.created_at).toLocaleDateString('ru-RU'))}
                secondaryTypographyProps={{ sx: snippetSx }}
              />
              <IconButton
                edge="end"
                aria-label="Удалить заметку"
                onClick={(e) => { e.stopPropagation(); setNoteToDelete(note); }}
              >
                <DeleteOutlineIcon fontSize="small" />
              </IconButton>
            </ListItemButton>
          );
        })}

        {!isLoading && notes.length === 0 && (
          <Typography color="text.secondary">Заметок пока нет</Typography>
        )}
      </List>

      <Fab color="primary" onClick={openCreate} sx={{ position: 'fixed', bottom: 80, right: 24 }} aria-label="Добавить заметку">
        <AddIcon />
      </Fab>

      <NoteDetailDrawer
        note={detailNote}
        onClose={() => setDetailId(null)}
        onEdit={openEditFromCard}
      />

      <NoteDialog open={formOpen} onClose={() => setFormOpen(false)} note={editingNote} />

      <Dialog open={pinDialogOpen} onClose={() => setPinDialogOpen(false)}>
        <DialogTitle>Введите PIN</DialogTitle>
        <DialogContent>
          <PinInput value={pinValue} onChange={setPinValue} onComplete={handlePinComplete} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPinDialogOpen(false)}>Отмена</Button>
        </DialogActions>
      </Dialog>

      {/* Delete confirmation — nothing is removed until the user confirms here. */}
      <Dialog open={Boolean(noteToDelete)} onClose={() => setNoteToDelete(null)}>
        <DialogTitle>Удалить заметку?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Вы уверены, что хотите удалить заметку? Это действие нельзя отменить.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setNoteToDelete(null)}>Отмена</Button>
          <Button color="error" variant="contained" onClick={confirmDelete} disabled={deleteNote.isPending}>
            Удалить
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
