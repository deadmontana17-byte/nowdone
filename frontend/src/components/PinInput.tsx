import { useRef } from 'react';
import { Box, TextField } from '@mui/material';

interface PinInputProps {
  length?: number;
  value: string;
  onChange: (value: string) => void;
  onComplete?: (value: string) => void;
}

/** Masked PIN entry as separate single-digit fields, per frontend form rules. */
export function PinInput({ length = 4, value, onChange, onComplete }: PinInputProps) {
  const refs = useRef<Array<HTMLInputElement | null>>([]);

  function setDigit(index: number, digit: string) {
    const digits = value.split('');
    digits[index] = digit;
    const next = digits.join('').slice(0, length);
    onChange(next);

    if (digit && index < length - 1) {
      refs.current[index + 1]?.focus();
    }
    if (next.length === length && !next.includes(undefined as unknown as string)) {
      onComplete?.(next);
    }
  }

  function handleKeyDown(index: number, e: React.KeyboardEvent<HTMLDivElement>) {
    if (e.key === 'Backspace' && !value[index] && index > 0) {
      refs.current[index - 1]?.focus();
    }
  }

  return (
    <Box sx={{ display: 'flex', gap: 1.5, justifyContent: 'center' }}>
      {Array.from({ length }).map((_, i) => (
        <TextField
          key={i}
          inputRef={(el) => (refs.current[i] = el)}
          value={value[i] ?? ''}
          onChange={(e) => setDigit(i, e.target.value.replace(/\D/g, '').slice(-1))}
          onKeyDown={(e) => handleKeyDown(i, e)}
          type="password"
          inputProps={{ inputMode: 'numeric', maxLength: 1, style: { textAlign: 'center', fontSize: 24 } }}
          sx={{ width: 56 }}
        />
      ))}
    </Box>
  );
}
