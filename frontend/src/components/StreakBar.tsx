import { useState } from 'react';
import { Box, LinearProgress, Typography } from '@mui/material';
import { useAuthStore } from '@/store/authStore';
import { CharacterDialog } from '@/components/CharacterDialog';
import { CHARACTER_NAMES, MAX_CHARACTER_INDEX, levelToIndex } from '@/utils/characterLevels';

export function StreakBar() {
  const user = useAuthStore((s) => s.user);
  const [dialogOpen, setDialogOpen] = useState(false);
  if (!user) return null;

  // current_streak now holds the character level (1..10): +1 per fully-done
  // day, -1 per day with unfinished tasks. See streak_service.go.
  const level = user.current_streak;
  const index = levelToIndex(level);
  const name = CHARACTER_NAMES[index];
  const progress = index === MAX_CHARACTER_INDEX ? 100 : ((level - 1) / MAX_CHARACTER_INDEX) * 100;

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
            Уровень {level} · {name} 🔥 (рекорд: {CHARACTER_NAMES[levelToIndex(user.max_streak)]})
          </Typography>
          <LinearProgress variant="determinate" value={progress} sx={{ height: 6, borderRadius: 3, mt: 0.5 }} />
        </Box>
      </Box>

      <CharacterDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        index={index}
        level={level}
      />
    </>
  );
}
