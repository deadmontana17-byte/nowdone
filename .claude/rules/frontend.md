---
description: Правила для фронтенда
globs: "*.tsx,*.ts"
always: true
---

# Frontend Rules

- **Структура папок**: 
  - `src/components/` — переиспользуемые компоненты.
  - `src/pages/` — страницы (главная, ЛК, настройки).
  - `src/store/` — стейт-менеджер (Zustand).
  - `src/api/` — вызовы к бэкенду через React Query.
  - `src/hooks/` — кастомные хуки.

- **Стилизация**: Только MUI (sx-пропсы). Не использовать CSS-модули или styled-components.
- **Анимация**: Для частиц и вспышек — Framer Motion. Для Lottie — использовать `lottie-react`.
- **Тёмная тема**: Всегда поддерживать тёмную тему через MUI ThemeProvider.
- **Роутинг**: Использовать React Router v6. Защищённые маршруты обёрнуты в `<ProtectedRoute>`.
- **Работа с формами**: Для ввода PIN — отдельные поля с маскировкой.