import { useEffect, useState } from 'react';
import {
  Dialog, DialogTitle, DialogContent, DialogActions, Button, Box, IconButton, Typography,
} from '@mui/material';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';

const MONTHS = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

interface MonthYearPickerDialogProps {
  open: boolean;
  onClose: () => void;
  /** Currently selected date, "YYYY-MM-DD". */
  value: string;
  /** Called with the first day of the chosen month, "YYYY-MM-01". */
  onSelect: (isoDate: string) => void;
}

/**
 * Lightweight month + year picker built from plain MUI (no extra date-picker
 * dependency). The year is stepped with the chevrons; picking a month commits
 * the selection and closes the dialog.
 */
export function MonthYearPickerDialog({ open, onClose, value, onSelect }: MonthYearPickerDialogProps) {
  // Parse straight from the "YYYY-MM-DD" string so the month/year never shift
  // across a timezone boundary the way `new Date(str)` can.
  const [yearPart, monthPart] = value.split('-');
  const selectedYear = Number(yearPart);
  const selectedMonth = Number(monthPart) - 1; // 0-based, matches Date.getMonth()

  // Local year lets the user browse other years before committing a month.
  const [year, setYear] = useState(selectedYear);
  useEffect(() => {
    if (open) setYear(selectedYear);
  }, [open, selectedYear]);

  function pick(monthIndex: number) {
    const mm = String(monthIndex + 1).padStart(2, '0');
    onSelect(`${year}-${mm}-01`);
    onClose();
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle sx={{ pb: 1 }}>Перейти к месяцу</DialogTitle>
      <DialogContent>
        {/* Year stepper */}
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 2, mb: 2 }}>
          <IconButton onClick={() => setYear((y) => y - 1)} aria-label="Предыдущий год">
            <ChevronLeftIcon />
          </IconButton>
          <Typography variant="h6" sx={{ minWidth: 72, textAlign: 'center' }}>{year}</Typography>
          <IconButton onClick={() => setYear((y) => y + 1)} aria-label="Следующий год">
            <ChevronRightIcon />
          </IconButton>
        </Box>

        {/* Month grid */}
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 1 }}>
          {MONTHS.map((label, index) => {
            const isCurrent = year === selectedYear && index === selectedMonth;
            return (
              <Button
                key={label}
                onClick={() => pick(index)}
                variant={isCurrent ? 'contained' : 'outlined'}
                size="small"
              >
                {label}
              </Button>
            );
          })}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Отмена</Button>
      </DialogActions>
    </Dialog>
  );
}
