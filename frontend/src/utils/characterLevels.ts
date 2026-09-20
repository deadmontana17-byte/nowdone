/** The 10 character levels, in order (index 0 = level 1). Mirrors the
 * backend's StatusIndex mapping in streak_service.go — keep both in sync. */
export const CHARACTER_NAMES = [
  'Зелёный',
  'Всё под контролем',
  'Босс дедлайнов',
  'Деловая жаба',
  'Мастер спокойствия',
  'Самурай дедлайнов',
  'Маг тайм-менеджмента',
  'Повелитель задач',
  'Легенда продуктивности',
  'Бог планирования',
] as const;

export const MAX_CHARACTER_INDEX = CHARACTER_NAMES.length - 1;

/** Maps a character level (1..10, from user.current_streak) to its character
 * image/name index (0..9), clamped so an out-of-range value is still safe. */
export function levelToIndex(level: number): number {
  return Math.min(Math.max(level - 1, 0), MAX_CHARACTER_INDEX);
}
