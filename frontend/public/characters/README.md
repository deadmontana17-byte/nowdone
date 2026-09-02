# Streak status characters

Static PNG images (no Lottie) shown by [StreakBar.tsx](../../src/components/StreakBar.tsx)
based on the user's current streak. Place 9 files here, one per status
category:

| File          | Streak range |
|---------------|--------------|
| `char_0.png`  | 1 day        |
| `char_1.png`  | 1–9 days     |
| `char_2.png`  | 10–19 days   |
| `char_3.png`  | 20–29 days   |
| `char_4.png`  | 30–39 days   |
| `char_5.png`  | 40–49 days   |
| `char_6.png`  | 50–59 days   |
| `char_7.png`  | 60–100 days  |
| `char_8.png`  | 100+ days    |

Recommended size: 128×128 (or any square size). Displayed at 56×56 in the
header; tapping the header opens a dialog that shows the current character at
160×160 and the next one dimmed at 110×110.
