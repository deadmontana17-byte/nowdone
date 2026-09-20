import { useState } from 'react';
import { Dialog, DialogTitle, DialogContent, DialogActions, Button, TextField, Stack, List, ListItem, ListItemText, IconButton, Box, Popover, useTheme } from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import Picker from '@emoji-mart/react';
import data from '@emoji-mart/data';

import type { TaskType } from '@/types';
import { useCreateTaskType, useDeleteTaskType, useTaskTypes } from '@/hooks/useTaskTypes';

interface TaskTypeDialogProps {
  open: boolean;
  onClose: () => void;
}

/** Modal for creating/deleting task types, with an emoji picker from Emoji
 * Mart, per the task-types spec. */
export function TaskTypeDialog({ open, onClose }: TaskTypeDialogProps) {
  const { data: taskTypes = [] } = useTaskTypes();
  const createTaskType = useCreateTaskType();
  const deleteTaskType = useDeleteTaskType();
  const muiTheme = useTheme();

  const [emoji, setEmoji] = useState('✅');
  const [name, setName] = useState('');
  const [pickerAnchor, setPickerAnchor] = useState<HTMLElement | null>(null);

  function handleCreate() {
    if (!name.trim()) return;
    createTaskType.mutate({ emoji, name: name.trim() }, { onSuccess: () => setName('') });
  }

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle>Типы задач</DialogTitle>
      <DialogContent>
        {/* mt:1 gives the shrunk "Название" label room — MUI zeroes DialogContent's
            top padding right after a DialogTitle, which otherwise clips it. */}
        <Stack direction="row" spacing={1} sx={{ mt: 1, mb: 2 }}>
          <Button variant="outlined" onClick={(e) => setPickerAnchor(e.currentTarget)} sx={{ minWidth: 56, fontSize: 20 }}>
            {emoji}
          </Button>
          <TextField
            label="Название"
            value={name}
            onChange={(e) => setName(e.target.value)}
            fullWidth
            // Forces the label into its small "floating" position and the
            // outline's notch open, in lock-step, from the very first paint —
            // see TaskFormDialog for why this fixes the label-crossed-by-border
            // glitch (they'd otherwise fall out of sync while filled/focused
            // state settles, which showed up worst on mobile).
            InputLabelProps={{ shrink: true }}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
          />
          <Button variant="contained" onClick={handleCreate} disabled={!name.trim()}>
            +
          </Button>
        </Stack>

        <Popover
          open={Boolean(pickerAnchor)}
          anchorEl={pickerAnchor}
          onClose={() => setPickerAnchor(null)}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
          transformOrigin={{ vertical: 'top', horizontal: 'left' }}
          // Keep a gap from the screen edges; Popover then flips/repositions so
          // the panel never spills off-screen.
          marginThreshold={8}
          // No open/scale animation: emoji-mart measures its container's width
          // (dynamicWidth) as soon as it mounts, and Popover's default Grow
          // transition scales that container from 0 during the animation — if
          // the measurement lands mid-scale, emoji-mart lays its grid out for
          // a too-small width, so the visual size is fine but the tap
          // coordinates it registers no longer line up with what's on screen.
          // Skipping the animation removes that race entirely.
          transitionDuration={0}
          // A fixed, generous width (not just a max-width cap) so the panel is
          // never the cramped ~280px it could shrink to near a screen edge —
          // "очень узкое" on phones — and, just as importantly, doesn't change
          // size after mount for the same reason as transitionDuration above.
          PaperProps={{
            sx: { width: 'min(94vw, 380px)', maxHeight: '70vh', overflow: 'auto' },
          }}
        >
          {/* dynamicWidth makes Emoji Mart fill the (now-stable) Popover width
              instead of its fixed ~350px. Fewer columns (perLine) + a larger
              emojiButtonSize give each emoji a bigger, more reliable tap
              target on mobile. */}
          <Picker
            data={data}
            theme={muiTheme.palette.mode}
            dynamicWidth
            perLine={6}
            emojiButtonSize={48}
            emojiSize={28}
            onEmojiSelect={(e: { native: string }) => {
              setEmoji(e.native);
              setPickerAnchor(null);
            }}
          />
        </Popover>

        <List dense>
          {taskTypes.map((t: TaskType) => (
            <ListItem
              key={t.id}
              secondaryAction={
                <IconButton edge="end" onClick={() => deleteTaskType.mutate(t.id)}>
                  <DeleteOutlineIcon fontSize="small" />
                </IconButton>
              }
            >
              <Box sx={{ mr: 1.5, fontSize: 18 }}>{t.emoji}</Box>
              <ListItemText primary={t.name} />
            </ListItem>
          ))}
        </List>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Закрыть</Button>
      </DialogActions>
    </Dialog>
  );
}
