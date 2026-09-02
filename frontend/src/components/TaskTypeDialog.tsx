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
        >
          <Picker
            data={data}
            theme={muiTheme.palette.mode}
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
