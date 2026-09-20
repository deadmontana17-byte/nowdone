import { useState } from 'react';
import {
  Dialog, DialogTitle, DialogContent, DialogActions, Button, TextField, Stack, List, ListItem, ListItemText,
  IconButton, Box, useTheme, useMediaQuery,
} from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import CloseIcon from '@mui/icons-material/Close';
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
  const fullScreenPicker = useMediaQuery(muiTheme.breakpoints.down('sm'));

  const [emoji, setEmoji] = useState('✅');
  const [name, setName] = useState('');
  const [pickerOpen, setPickerOpen] = useState(false);

  function handleCreate() {
    if (!name.trim()) return;
    createTaskType.mutate({ emoji, name: name.trim() }, { onSuccess: () => setName('') });
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="xs"
      // See TaskFormDialog: recompute the outlined TextField's border notch
      // after the open transition settles, so it never lands on a stale
      // mid-transition measurement (which shows as the label "crossed out").
      TransitionProps={{ onEntered: () => window.dispatchEvent(new Event('resize')) }}
    >
      <DialogTitle>Типы задач</DialogTitle>
      <DialogContent
        // Delegated: catches focus from any descendant TextField (React's
        // synthetic onFocus bubbles). Right after a field gets focus, the
        // mobile keyboard opens and can resize/zoom the viewport; recomputing
        // the outlined input's border-notch once that settles keeps the
        // "Название" label from ever landing crossed-out by the outline.
        onFocus={() => window.setTimeout(() => window.dispatchEvent(new Event('resize')), 150)}
      >
        {/* mt:1 gives the shrunk "Название" label room — MUI zeroes DialogContent's
            top padding right after a DialogTitle, which otherwise clips it. */}
        <Stack direction="row" spacing={1} sx={{ mt: 1, mb: 2 }}>
          <Button variant="outlined" onClick={() => setPickerOpen(true)} sx={{ minWidth: 56, fontSize: 20 }}>
            {emoji}
          </Button>
          <TextField
            label="Название"
            value={name}
            onChange={(e) => setName(e.target.value)}
            fullWidth
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
          />
          <Button variant="contained" onClick={handleCreate} disabled={!name.trim()}>
            +
          </Button>
        </Stack>

        {/* A full Dialog (not a Popover anchored to the emoji button) so the
            picker always gets generous, viewport-independent space — full
            screen on phones — instead of being squeezed by where the trigger
            button happens to sit. Combined with a bigger emojiButtonSize this
            gives each emoji a much larger, more reliable touch target. */}
        <Dialog
          open={pickerOpen}
          onClose={() => setPickerOpen(false)}
          fullScreen={fullScreenPicker}
          fullWidth
          maxWidth="xs"
        >
          <DialogTitle sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            Выберите эмодзи
            <IconButton onClick={() => setPickerOpen(false)} aria-label="Закрыть">
              <CloseIcon />
            </IconButton>
          </DialogTitle>
          <DialogContent sx={{ p: 0, display: 'flex', justifyContent: 'center' }}>
            {/* dynamicWidth makes Emoji Mart fill the available width instead of
                its fixed ~350px. Fewer columns (perLine) + a larger
                emojiButtonSize give each emoji a bigger tap target, reducing
                mis-taps on mobile. */}
            <Picker
              data={data}
              theme={muiTheme.palette.mode}
              dynamicWidth
              perLine={fullScreenPicker ? 7 : 6}
              emojiButtonSize={48}
              emojiSize={28}
              onEmojiSelect={(e: { native: string }) => {
                setEmoji(e.native);
                setPickerOpen(false);
              }}
            />
          </DialogContent>
        </Dialog>

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
