/** The four checkbox celebration styles, one picked at random per tick. */
export type BurstVariant = 'confetti' | 'ring' | 'starburst' | 'sparkle';

const VARIANTS: BurstVariant[] = ['confetti', 'ring', 'starburst', 'sparkle'];

export function randomBurstVariant(): BurstVariant {
  return VARIANTS[Math.floor(Math.random() * VARIANTS.length)];
}
