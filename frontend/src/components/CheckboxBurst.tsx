import { useEffect, useMemo } from 'react';
import { motion } from 'framer-motion';

import type { BurstVariant } from '@/utils/burst';

/**
 * Celebratory burst rendered directly on top of a task checkbox when it is
 * ticked. Four visually distinct variants are picked at random per click
 * (see randomBurstVariant in utils/burst). The host must be `position: relative`.
 */

const COLORS = ['#7c6bff', '#ff6ba8', '#4ade80', '#fbbf24', '#38bdf8'];

interface CheckboxBurstProps {
  variant: BurstVariant;
  /** Called once the animation has finished so the host can unmount it. */
  onDone: () => void;
}

const overlaySx = {
  position: 'absolute' as const,
  inset: 0,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  pointerEvents: 'none' as const,
  zIndex: 3,
};

export function CheckboxBurst({ variant, onDone }: CheckboxBurstProps) {
  useEffect(() => {
    const t = setTimeout(onDone, 800);
    return () => clearTimeout(t);
  }, [onDone]);

  return (
    <motion.div style={overlaySx} initial={{ opacity: 1 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
      {variant === 'confetti' && <Confetti />}
      {variant === 'ring' && <Ring />}
      {variant === 'starburst' && <Starburst />}
      {variant === 'sparkle' && <Sparkle />}
    </motion.div>
  );
}

function Confetti() {
  const bits = useMemo(
    () =>
      Array.from({ length: 14 }, (_, i) => ({
        angle: (i / 14) * Math.PI * 2 + Math.random() * 0.5,
        distance: 26 + Math.random() * 22,
        color: COLORS[i % COLORS.length],
        rotate: (Math.random() - 0.5) * 540,
        size: 4 + Math.random() * 4,
      })),
    [],
  );
  return (
    <>
      {bits.map((b, i) => (
        <motion.span
          key={i}
          initial={{ x: 0, y: 0, opacity: 1, rotate: 0, scale: 1 }}
          animate={{
            x: Math.cos(b.angle) * b.distance,
            y: Math.sin(b.angle) * b.distance + 14, // slight gravity
            opacity: 0,
            rotate: b.rotate,
            scale: 0.6,
          }}
          transition={{ duration: 0.7, ease: 'easeOut' }}
          style={{
            position: 'absolute',
            width: b.size,
            height: b.size * 1.6,
            borderRadius: 1,
            background: b.color,
          }}
        />
      ))}
    </>
  );
}

function Ring() {
  return (
    <>
      <motion.span
        initial={{ scale: 0.3, opacity: 0.9 }}
        animate={{ scale: 2.6, opacity: 0 }}
        transition={{ duration: 0.55, ease: 'easeOut' }}
        style={{ position: 'absolute', width: 26, height: 26, borderRadius: '50%', border: '2px solid #7c6bff' }}
      />
      <motion.span
        initial={{ scale: 0.3, opacity: 0.6 }}
        animate={{ scale: 3.4, opacity: 0 }}
        transition={{ duration: 0.65, ease: 'easeOut', delay: 0.05 }}
        style={{ position: 'absolute', width: 26, height: 26, borderRadius: '50%', border: '2px solid #4ade80' }}
      />
      <motion.span
        initial={{ scale: 0, opacity: 0.8 }}
        animate={{ scale: 1.6, opacity: 0 }}
        transition={{ duration: 0.4, ease: 'easeOut' }}
        style={{
          position: 'absolute',
          width: 24,
          height: 24,
          borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(124,107,255,0.85) 0%, rgba(124,107,255,0) 70%)',
        }}
      />
    </>
  );
}

function Starburst() {
  const spikes = 9;
  return (
    <motion.div
      initial={{ rotate: 0, scale: 0.6, opacity: 1 }}
      animate={{ rotate: 40, scale: 1.4, opacity: 0 }}
      transition={{ duration: 0.6, ease: 'easeOut' }}
      style={{ position: 'absolute', width: 0, height: 0 }}
    >
      {Array.from({ length: spikes }).map((_, i) => (
        <motion.span
          key={i}
          initial={{ scaleY: 0.2 }}
          animate={{ scaleY: 1 }}
          transition={{ duration: 0.3, ease: 'easeOut' }}
          style={{
            position: 'absolute',
            width: 2.5,
            height: 16,
            borderRadius: 2,
            background: COLORS[i % COLORS.length],
            transformOrigin: 'center bottom',
            transform: `rotate(${(i / spikes) * 360}deg) translateY(-14px)`,
          }}
        />
      ))}
    </motion.div>
  );
}

function Sparkle() {
  const stars = useMemo(
    () =>
      Array.from({ length: 6 }, (_, i) => ({
        x: (Math.random() - 0.5) * 44,
        y: (Math.random() - 0.5) * 44,
        delay: Math.random() * 0.15,
        rotate: (Math.random() - 0.5) * 120,
        char: i % 2 === 0 ? '✦' : '★',
        color: COLORS[i % COLORS.length],
      })),
    [],
  );
  return (
    <>
      {stars.map((s, i) => (
        <motion.span
          key={i}
          initial={{ x: 0, y: 0, scale: 0, opacity: 0, rotate: 0 }}
          animate={{ x: s.x, y: s.y, scale: [0, 1.4, 0], opacity: [0, 1, 0], rotate: s.rotate }}
          transition={{ duration: 0.6, ease: 'easeOut', delay: s.delay }}
          style={{ position: 'absolute', fontSize: 14, lineHeight: 1, color: s.color }}
        >
          {s.char}
        </motion.span>
      ))}
    </>
  );
}
