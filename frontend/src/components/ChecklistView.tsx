import { Box, Typography, List, ListItem, ListItemButton, ListItemIcon, ListItemText, LinearProgress } from '@mui/material';
import CheckCircleRoundedIcon from '@mui/icons-material/CheckCircleRounded';
import RadioButtonUncheckedRoundedIcon from '@mui/icons-material/RadioButtonUncheckedRounded';

import type { ChecklistItem } from '@/types';

interface ChecklistViewProps {
  items: ChecklistItem[];
  onToggle: (id: string, done: boolean) => void;
}

/** Read-and-toggle checklist shown in the task detail card. Round checkboxes;
 * clicking a row flips that item. Completed items are muted, never struck out. */
export function ChecklistView({ items, onToggle }: ChecklistViewProps) {
  if (items.length === 0) return null;

  const done = items.filter((i) => i.done).length;
  const progress = Math.round((done / items.length) * 100);

  return (
    <Box>
      <Typography variant="caption" color="text.secondary">
        Чек-лист · {done}/{items.length}
      </Typography>
      <LinearProgress
        variant="determinate"
        value={progress}
        sx={{ my: 0.75, height: 6, borderRadius: 3 }}
        color={done === items.length ? 'success' : 'primary'}
      />
      <List dense disablePadding>
        {items.map((item) => (
          <ListItem key={item.id} disableGutters disablePadding>
            <ListItemButton onClick={() => onToggle(item.id, !item.done)} dense sx={{ borderRadius: 1, px: 0.5 }}>
              <ListItemIcon sx={{ minWidth: 34 }}>
                {item.done ? (
                  <CheckCircleRoundedIcon fontSize="small" color="success" />
                ) : (
                  <RadioButtonUncheckedRoundedIcon fontSize="small" color="disabled" />
                )}
              </ListItemIcon>
              <ListItemText
                primary={item.text}
                primaryTypographyProps={{
                  variant: 'body2',
                  sx: { color: item.done ? 'text.disabled' : 'text.primary' },
                }}
              />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </Box>
  );
}
