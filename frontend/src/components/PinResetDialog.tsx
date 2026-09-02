import { useEffect, useRef, useState } from 'react';
import {
  Dialog, DialogTitle, DialogContent, DialogActions, Button, Stack, Typography,
  Link as MuiLink, CircularProgress, TextField,
} from '@mui/material';

import { PinInput } from '@/components/PinInput';
import { startPinReset, redeemPinReset, setNewPin } from '@/api/auth';
import { ApiError } from '@/api/client';
import { useAuthStore } from '@/store/authStore';
import { useUiStore } from '@/store/uiStore';

interface PinResetDialogProps {
  open: boolean;
  onClose: () => void;
  /** Called after the new PIN has been saved successfully. */
  onDone: () => void;
}

type Step = 'sending' | 'code' | 'new';

const CODE_LENGTH = 6;

/**
 * Self-contained PIN reset flow shown as a modal.
 *
 * On open it asks the backend to send a 6-digit code to the user's Telegram
 * chat (no bot launch / deep link). The user types that code here, then picks a
 * new 4-digit PIN twice. The new PIN becomes the one used both for login and for
 * unlocking hidden notes.
 */
export function PinResetDialog({ open, onClose, onDone }: PinResetDialogProps) {
  const setPinUnlocked = useAuthStore((s) => s.setPinUnlocked);
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  const [step, setStep] = useState<Step>('sending');
  const [code, setCode] = useState('');
  const [pin, setPin] = useState('');
  const [pinConfirm, setPinConfirm] = useState('');
  const [busy, setBusy] = useState(false);

  // Guards against a second submit while the first request is still in flight
  // (state updates are async, so `busy` alone can be stale inside a handler).
  const submittingRef = useRef(false);

  // Keep the latest onClose without making it an effect dependency, so a parent
  // re-render while the dialog is open can't retrigger the code request.
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  // Request a fresh code once, each time the dialog transitions to open.
  useEffect(() => {
    if (!open) return;
    setStep('sending');
    setCode('');
    setPin('');
    setPinConfirm('');
    setBusy(false);
    submittingRef.current = false;

    let cancelled = false;
    startPinReset()
      .then(() => {
        if (cancelled) return;
        setStep('code');
        showSnackbar('Код отправлен в Telegram', 'info');
      })
      .catch((err) => {
        if (cancelled) return;
        showSnackbar(err instanceof ApiError ? err.message : 'Не удалось отправить код сброса', 'error');
        onCloseRef.current();
      });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  async function handleResend() {
    setBusy(true);
    try {
      await startPinReset();
      setCode('');
      showSnackbar('Новый код отправлен в Telegram', 'info');
    } catch (err) {
      showSnackbar(err instanceof ApiError ? err.message : 'Не удалось отправить код', 'error');
    } finally {
      setBusy(false);
    }
  }

  // `submitted` is the freshly typed value passed straight from onChange, so we
  // never redeem a stale (still-rendering) `code` state.
  async function handleConfirmCode(submitted?: string) {
    const value = (submitted ?? code).replace(/\D/g, '').slice(0, CODE_LENGTH);
    if (value.length !== CODE_LENGTH || submittingRef.current) return;

    submittingRef.current = true;
    setBusy(true);
    try {
      await redeemPinReset(value);
      setStep('new');
      setPin('');
      setPinConfirm('');
    } catch (err) {
      showSnackbar(err instanceof ApiError ? err.message : 'Код неверен или истёк', 'error');
      setCode('');
    } finally {
      submittingRef.current = false;
      setBusy(false);
    }
  }

  function handleCodeChange(raw: string) {
    const next = raw.replace(/\D/g, '').slice(0, CODE_LENGTH);
    setCode(next);
    if (next.length === CODE_LENGTH) handleConfirmCode(next);
  }

  async function handleSaveNewPin() {
    if (pin.length !== 4 || pin !== pinConfirm) {
      showSnackbar('PIN-коды не совпадают', 'error');
      return;
    }
    setBusy(true);
    try {
      await setNewPin(pin);
      setPinUnlocked(true);
      showSnackbar('PIN обновлён', 'success');
      onDone();
    } catch (err) {
      showSnackbar(err instanceof ApiError ? err.message : 'Не удалось обновить PIN', 'error');
      setPin('');
      setPinConfirm('');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onClose={busy ? undefined : onClose} fullWidth maxWidth="xs">
      <DialogTitle>
        {step === 'new' ? 'Новый PIN' : 'Сброс PIN'}
      </DialogTitle>
      <DialogContent>
        {step === 'sending' && (
          <Stack alignItems="center" sx={{ py: 3 }}>
            <CircularProgress />
          </Stack>
        )}

        {step === 'code' && (
          <Stack spacing={2} sx={{ pt: 1 }}>
            <Typography variant="body2" color="text.secondary" textAlign="center">
              Мы отправили 6-значный код в бота NowDone. Введите его ниже — код действителен 5 минут.
            </Typography>
            <TextField
              value={code}
              onChange={(e) => handleCodeChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleConfirmCode();
              }}
              autoFocus
              fullWidth
              disabled={busy}
              inputProps={{
                inputMode: 'numeric',
                autoComplete: 'one-time-code',
                maxLength: CODE_LENGTH,
                'aria-label': 'Код из Telegram',
                style: { textAlign: 'center', fontSize: 28, letterSpacing: '0.4em', fontWeight: 600 },
              }}
            />
            <MuiLink
              component="button"
              type="button"
              variant="body2"
              disabled={busy}
              onClick={handleResend}
              sx={{ alignSelf: 'center' }}
            >
              Отправить код повторно
            </MuiLink>
          </Stack>
        )}

        {step === 'new' && (
          <Stack spacing={2.5} alignItems="center" sx={{ pt: 1 }}>
            <Typography variant="body2" color="text.secondary" textAlign="center">
              Придумайте новый 4-значный PIN и повторите его.
            </Typography>
            <Stack spacing={1} alignItems="center">
              <Typography variant="caption" color="text.secondary">Новый PIN</Typography>
              <PinInput value={pin} onChange={setPin} />
            </Stack>
            <Stack spacing={1} alignItems="center">
              <Typography variant="caption" color="text.secondary">Повторите PIN</Typography>
              <PinInput value={pinConfirm} onChange={setPinConfirm} />
            </Stack>
          </Stack>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={busy}>Отмена</Button>
        {step === 'new' && (
          <Button
            variant="contained"
            onClick={handleSaveNewPin}
            disabled={pin.length !== 4 || pinConfirm.length !== 4 || busy}
          >
            Сохранить PIN
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
