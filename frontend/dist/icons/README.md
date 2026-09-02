# PWA icons

Place the following files here before building for production:

- `icon-192.png` — 192×192 PNG, used as the PWA install icon and favicon.
- `icon-512.png` — 512×512 PNG, used as the splash/maskable icon.

Both are referenced in [vite.config.ts](../../vite.config.ts) (VitePWA manifest)
and [index.html](../../index.html). Any square PNG with your app logo works;
for a maskable icon keep the important content within the center ~80% safe zone.
