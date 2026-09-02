import { Stack, Checkbox, TextField, IconButton, Button, Typography } from '@mui/material';
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import CloseIcon from '@mui/icons-material/Close';
import AddIcon from '@mui/icons-material/Add';

import type { ChecklistItem } from '@/types';
import { newChecklistItem } from '@/utils/description';

interface ChecklistEditorProps {
  items: ChecklistItem[];
  onChange: (items: ChecklistItem[]) => void;
}

/** Editable checklist used inside the task form: round checkbox + text field +
 * remove button per row, plus an "add item" action. Stored as
 * { id, text, done }[] in the task description. */
export function ChecklistEditor({ items, onChange }: ChecklistEditorProps) {
  function patch(id: string, next: Partial<ChecklistItem>) {
    onChange(items.map((i) => (i.id === id ? { ...i, ...next } : i)));
  }
  function remove(id: string) {
    onChange(items.filter((i) => i.id !== id));
  }
  function add() {
    onChange([...items, newChecklistItem()]);
  }

  return (
    <Stack spacing={1}>
      <Typography variant="caption" color="text.secondary">
        Чек-лист
      </Typography>

      {items.map((item) => (
        <Stack key={item.id} direction="row" alignItems="center" spacing={0.5}>
          <Checkbox
            size="small"
            checked={item.done}
            onChange={(e) => patch(item.id, { done: e.target.checked })}
            icon={<RadioButtonUncheckedIcon />}
            checkedIcon={<CheckCircleIcon />}
            sx={{ p: 0.5 }}
          />
          <TextField
            value={item.text}
            onChange={(e) => patch(item.id, { text: e.target.value })}
            placeholder="Пункт списка"
            variant="standard"
            fullWidth
            // Keep the row separation but make the underline barely there.
            // Theme tokens so it reads correctly in both light and dark mode.
            sx={{
              '& .MuiInput-underline:before': { borderBottomColor: 'divider' },
              '& .MuiInput-underline:hover:not(.Mui-disabled):before': {
                borderBottomColor: 'text.disabled',
              },
            }}
          />
          <IconButton size="small" onClick={() => remove(item.id)} aria-label="Удалить пункт">
            <CloseIcon fontSize="small" />
          </IconButton>
        </Stack>
      ))}

      <Button size="small" startIcon={<AddIcon />} onClick={add} sx={{ alignSelf: 'flex-start' }}>
        Добавить пункт
      </Button>
    </Stack>
  );
}
