import { useState } from 'react';
import { Box, LinearProgress, Typography } from '@mui/material';
import { useAuthStore } from '@/store/authStore';
import { CharacterDialog } from '@/components/CharacterDialog';

// Category thresholds and static character PNGs, per the spec (no Lottie).
// Index: 0 = "1 день", 1 = "1-9", 2 = "10-19", ..., 7 = "60-100", 8 = "100+".
const CATEGORIES = [
  { max: 1, label: '1 день' },
  { max: 9, label: '1–9 дней' },
  { max: 19, label: '10–19 дней' },
  { max: 29, label: '20–29 дней' },
  { max: 39, label: '30–39 дней' },
  { max: 49, label: '40–49 дней' },
  { max: 59, label: '50–59 дней' },
  { max: 100, label: '60–100 дней' },
  { max: Infinity, label: '100+ дней' },
];

function statusIndex(streak: number): number {
  if (streak <= 1) return 0;
  if (streak < 10) return 1;
  if (streak < 20) return 2;
  if (streak < 30) return 3;
  if (streak < 40) return 4;
  if (streak < 50) return 5;
  if (streak < 60) return 6;
  if (streak <= 100) return 7;
  return 8;
}

export function StreakBar() {
  const user = useAuthStore((s) => s.user);
  const [dialogOpen, setDialogOpen] = useState(false);
  if (!user) return null;

  const index = statusIndex(user.current_streak);
  const category = CATEGORIES[index];
  const prevMax = index === 0 ? 0 : CATEGORIES[index - 1].max;
  const span = category.max === Infinity ? 1 : category.max - prevMax;
  const progress = category.max === Infinity ? 100 : Math.min(100, ((user.current_streak - prevMax) / span) * 100);

  return (
    <>
      <Box
        role="button"
        tabIndex={0}
        aria-label="Открыть персонажа стрика"
        onClick={() => setDialogOpen(true)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setDialogOpen(true);
          }
        }}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1.5,
          px: 2,
          pb: 1.5,
          cursor: 'pointer',
          '&:hover': { opacity: 0.85 },
        }}
      >
        <Box
          component="img"
          src={`/characters/char_${index}.png`}
          alt="Статус стрика"
          sx={{ width: 56, height: 56, objectFit: 'contain' }}
        />
        <Box sx={{ flexGrow: 1 }}>
          <Typography variant="caption" color="text.secondary">
            Стрик: {user.current_streak} 🔥 · {category.label} (рекорд {user.max_streak})
          </Typography>
          <LinearProgress variant="determinate" value={progress} sx={{ height: 6, borderRadius: 3, mt: 0.5 }} />
        </Box>
      </Box>

      <CharacterDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        index={index}
        currentStreak={user.current_streak}
      />
    </>
  );
}
