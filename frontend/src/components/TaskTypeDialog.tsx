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
          transitionDuration={0}
          // maxHeight only — no fixed/forced width. Emoji Mart sizes itself
          // (see the Picker comment below): letting it size itself and the
          // Popover just wrap that size is what keeps the two in agreement.
          PaperProps={{
            sx: { maxWidth: '95vw', maxHeight: '70vh', overflow: 'auto' },
          }}
        >
          {/* No dynamicWidth: it sizes the grid off a ResizeObserver reading of
              the container's width, which in this Popover was consistently
              landing on roughly half the panel's actual final width — emoji
              only filled half the window, and the tap coordinates it computed
              from that wrong width no longer lined up with what was on
              screen, hence the wrong-emoji-selected bug. Without it, Emoji
              Mart lays itself out from perLine × emojiButtonSize directly (no
              measurement step), so what's drawn and what's tappable always
              match. A bigger emojiButtonSize than the ~36px default still
              gives comfortably large touch targets. */}
          <Picker
            data={data}
            theme={muiTheme.palette.mode}
            perLine={8}
            emojiButtonSize={40}
            emojiSize={24}
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
