import { Box, Dialog, DialogContent, IconButton, Stack, Typography } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';

// Category labels mirror StreakBar / the backend StatusIndex (0..8).
const CATEGORY_LABELS = [
  '1 день',
  '1–9 дней',
  '10–19 дней',
  '20–29 дней',
  '30–39 дней',
  '40–49 дней',
  '50–59 дней',
  '60–100 дней',
  '100+ дней',
];

const MAX_INDEX = CATEGORY_LABELS.length - 1;

interface CharacterDialogProps {
  open: boolean;
  onClose: () => void;
  /** Current status index (0..8). */
  index: number;
  currentStreak: number;
}

/** Modal shown when the user taps the streak header: the current character
 * large, the next one dimmed, and a motivational line. */
export function CharacterDialog({ open, onClose, index, currentStreak }: CharacterDialogProps) {
  const safeIndex = Math.min(Math.max(index, 0), MAX_INDEX);
  const nextIndex = Math.min(safeIndex + 1, MAX_INDEX);
  const isMaxed = safeIndex === MAX_INDEX;

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <IconButton onClick={onClose} aria-label="Закрыть" sx={{ position: 'absolute', right: 8, top: 8 }}>
        <CloseIcon />
      </IconButton>
      <DialogContent sx={{ pt: 4 }}>
        <Stack spacing={3} alignItems="center">
          <Typography variant="h6" sx={{ fontWeight: 700 }}>
            Твой персонаж
          </Typography>

          <Stack direction="row" spacing={3} alignItems="flex-end" justifyContent="center">
            <Stack spacing={1} alignItems="center">
              <Box
                component="img"
                src={`/characters/char_${safeIndex}.png`}
                alt="Текущий персонаж"
                sx={{ width: 160, height: 160, objectFit: 'contain' }}
              />
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                Сейчас · {CATEGORY_LABELS[safeIndex]}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                Стрик: {currentStreak} 🔥
              </Typography>
            </Stack>

            <Stack spacing={1} alignItems="center" sx={{ opacity: 0.4 }}>
              <Box
                component="img"
                src={`/characters/char_${nextIndex}.png`}
                alt="Следующий персонаж"
                sx={{ width: 110, height: 110, objectFit: 'contain', filter: 'grayscale(1)' }}
              />
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {isMaxed ? 'Максимум' : `Далее · ${CATEGORY_LABELS[nextIndex]}`}
              </Typography>
            </Stack>
          </Stack>

          <Typography variant="body1" align="center" color="text.secondary">
            Ежедневно выполняй все задачи и прокачивай своего персонажа!
          </Typography>
        </Stack>
      </DialogContent>
    </Dialog>
  );
}
