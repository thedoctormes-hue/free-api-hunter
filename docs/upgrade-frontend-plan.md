# Frontend Upgrade Plan — free-api-hunter

> Исследование текущего состояния и план апгрейда.
> Дата: 2026-06-26
> Статус: Draft (не начинать реализацию без одобрения)

---

## 1. Текущее состояние

### 1.1 Стек технологий

| Категория | Библиотека | Версия | Статус |
|-----------|-----------|--------|--------|
| Framework | React | ^19.2.6 | ✅ Актуальный |
| Build | Vite | ^8.0.12 | ✅ Актуальный |
| Language | TypeScript | ~6.0.2 | ✅ Актуальный |
| CSS | Tailwind CSS | ^4.3.1 | ✅ Актуемый |
| Routing | react-router-dom | ^7.18.0 | ✅ Актуальный |
| Data Fetching | @tanstack/react-query | ^5.101.0 | ✅ Уже есть |
| Charts | recharts | ^3.8.1 | ✅ Уже есть |
| State | zustand | ^5.0.0 | ✅ Уже есть |
| UI Icons | lucide-react | ^1.21.0 | ✅ Актуальный |
| Animations | framer-motion | ^12.40.0 | ✅ Уже есть |
| Tables | @tanstack/react-virtual | ^3.14.3 | ✅ Уже есть |
| CSV | papaparse | ^5.4.1 | ✅ Уже есть |
| Date | date-fns | ^4.1.0 | ✅ Актуальный |
| CN | tailwind-merge + clsx | — | ✅ Стандарт |

**Вывод:** Стек уже современный. Нет необходимости в замене основных библиотек.

### 1.2 Структура проекта

```
web/src/
├── api/                    # API клиенты
│   ├── client.ts           # Базовый fetchJSON
│   ├── providers.ts        # /api/v1/providers
│   ├── findings.ts         # /api/v1/findings
│   ├── stats.ts            # /api/v1/stats
│   └── tts.ts              # /api/v1/tts/*
├── components/
│   ├── dashboard/
│   │   ├── stats-cards.tsx          # 4 метрик-карточки
│   │   ├── providers-chart.tsx      # PieChart статусы
│   │   └── findings-by-source-chart.tsx  # BarChart находки
│   ├── layout/
│   │   ├── header.tsx
│   │   └── sidebar.tsx
│   ├── tts/
│   │   ├── tts-card.tsx
│   │   └── tts-stats.tsx
│   ├── ui/                 # badge, card, skeleton
│   └── shared/
│       └── error-boundary.tsx
├── contexts/
│   └── theme.tsx           # dark/light toggle (уже есть!)
├── hooks/
│   ├── use-providers.ts    # React Query обёртка
│   ├── use-findings.ts     # React Query обёртка
│   ├── use-stats.ts        # React Query + 30s polling
│   └── use-tts.ts          # React Query обёртка
├── lib/
│   ├── types.ts            # Все интерфейсы
│   ├── store.ts            # Zustand (sidebar, search)
│   └── utils.ts            # cn(), formatDate(), formatNumber()
├── pages/
│   ├── dashboard.tsx       # Главная: графики + обзор + recent findings
│   ├── providers.tsx       # Карточки/таблица с фильтрами
│   ├── findings.tsx        # Список с фильтрами
│   ├── tts.tsx             # TTS провайдеры (НЕ ИСПОЛЬЗУЕТ общий layout!)
│   ├── stats.tsx           # Статистика
│   └── not-found.tsx
├── App.tsx                 # Роутинг + providers
└── index.css               # CSS-переменные (light/dark)
```

### 1.3 Что уже хорошо сделано

1. **React Query** — уже подключён, кэширование (5min stale, 10gc), auto-refresh для stats (30s)
2. **Recharts** — уже есть PieChart (статусы) и BarChart (находки по источникам)
3. **Zustand** — уже используется для sidebar/search
4. **Theme toggle** — ThemeProvider уже есть (dark/light, localStorage, system preference)
5. **Error Boundary** — уже оборачивает страницы
6. **Lazy loading** — все страницы загружаются через React.lazy + Suspense
7. **Skeleton loaders** — везде есть skeleton при загрузке
8. **Фильтрация** — уже есть на providers (status, CC, search) и findings (source, score)
9. **API contract** — все типы определены в `lib/types.ts`, API клиенты в `api/`

### 1.4 Что требует улучшения

1. **TTS страница не использует общий layout** — у неё свой `<div className="min-h-screen bg-[#0d1117]">` вместо общего лейаута с sidebar/header
2. **Нет экспорта данных** — нет кнопок Export JSON/CSV
3. **Нет визуализации пула ключей TTS** — нет progress bars для `free_tier.char_limit`
4. **Нет поиска на findings** — только фильтры
5. **Нет сортировки в таблице providers** — сортировка только по status
6. **Нет пагинации** — при большом количестве findings рендерится всё
7. **Нет уведомлений** — нет toast/alert при ошибках API
8. **Нет подтверждения удаления/действий** — нет confirmation dialogs

---

## 2. План апгрейда (по приоритету)

### P0 — Критичные (исправление архитектурных проблем)

#### 2.1 TTS: интеграция в общий layout

**Проблема:** `tts.tsx` рендерит свой собственный layout с тёмными цветами вместо использования общего `Header` + `Sidebar` + `ThemeProvider`. Это ломает навигацию и тему.

**Действия:**
- [ ] Переписать `tts.tsx` — убрать обёртку `min-h-screen`, использовать общий layout как другие страницы
- [ ] Убрать дублирующиеся стили (`bg-[#0d1117]`, `text-[#c9d1d9]`, `text-[#58a6ff]`)
- [ ] Использовать CSS-переменные (`var(--bg-base)`, `var(--text-primary)`, `var(--accent)`)

**Файлы:** `web/src/pages/tts.tsx`
**Объём:** ~30 мин

---

### P1 — Высокий приоритет (быстрые победы)

#### 2.2 Theme toggle: кнопка в Header

**Текущее:** ThemeProvider уже есть, но нет UI-кнопки для переключения.

**Действия:**
- [ ] Добавить кнопку Sun/Moon иконку в `Header`
- [ ] Подключить `useTheme()` из `contexts/theme`
- [ ] Анимация перехода через Tailwind `transition-colors`

**Файлы:** `web/src/components/layout/header.tsx`, `web/src/components/ui/theme-toggle.tsx` (новый)
**Объём:** ~20 мин

#### 2.3 Export: кнопки JSON/CSV

**Текущее:** `papaparse` уже в зависимостях, но не используется.

**Действия:**
- [ ] Создать компонент `ExportButton` с двумя вариантами: JSON и CSV
- [ ] Добавить на страницу Findings (экспорт отфильтрованных данных)
- [ ] Добавить на страницу Providers (экспорт отфильтрованных данных)
- [ ] JSON: `Blob` + `URL.createObjectURL` → download
- [ ] CSV: `papaparse.unparse()` → download

**Файлы:**
- `web/src/components/shared/export-button.tsx` (новый)
- `web/src/pages/findings.tsx` (добавить кнопку)
- `web/src/pages/providers.tsx` (добавить кнопку)

**Объём:** ~40 мин

#### 2.4 TTS: визуализация пула ключей (Progress Bars)

**Текущее:** `free_tier.char_limit` есть в данных, но не визуализирован.

**Действия:**
- [ ] Создать компонент `ProgressBar` (переиспользуемый)
- [ ] Добавить в `tts-card.tsx` секцию с прогресс-барами для:
  - `char_limit` (визуализация относительно максимума среди всех провайдеров)
  - `voice_clones` (если > 0)
- [ ] Цветовая индикация: зелёный (>50%), жёлтый (10-50%), красный (<10%)

**Файлы:**
- `web/src/components/ui/progress-bar.tsx` (новый)
- `web/src/components/tts/tts-card.tsx` (изменить)

**Объём:** ~30 мин

---

### P2 — Средний приоритет (UX улучшения)

#### 2.5 Findings: текстовый поиск

**Текущее:** Только фильтры по source и score.

**Действия:**
- [ ] Добавить input "Search findings..." над списком
- [ ] Поиск по `title`, `description`, `raw_text`
- [ ] Debounce 300ms для избежания частых перерисовок

**Файлы:** `web/src/pages/findings.tsx`
**Объём:** ~15 мин

#### 2.6 Providers: сортировка в таблице

**Текущее:** Сортировка только по status (захардкожена).

**Действия:**
- [ ] Добавить кликабельные заголовки колонок (Name, Models count, Discovered)
- [ ] Стрелка сортировки (↑↓)
- [ ] Поддержка sort через API (если бэкенд поддерживает) или клиентская сортировка

**Файлы:** `web/src/pages/providers.tsx`
**Объём:** ~25 мин

#### 2.7 Dashboard: улучшения графиков

**Текущее:** Базовые PieChart и BarChart. Функциональность есть, но можно улучшить.

**Действия:**
- [ ] Добавить анимацию появления графиков (framer-motion)
- [ ] Добавить "пустые" состояния с иконкой и текстом
- [ ] Tooltip с дополнительной информацией (проценты)
- [ ] Responsive: на мобильных — уменьшить размер / использовать легенду снизу

**Файлы:**
- `web/src/components/dashboard/providers-chart.tsx`
- `web/src/components/dashboard/findings-by-source-chart.tsx`

**Объём:** ~30 мин

#### 2.8 Пагинация для Findings

**Текущее:** Рендерятся все findings сразу.

**Действия:**
- [ ] Добавить `Pagination` компонент (или использовать `@tanstack/react-virtual` для virtual scroll)
- [ ] По 20-50 items на страницу
- [ ] Сохранение состояния страницы в URL hash

**Файлы:**
- `web/src/components/ui/pagination.tsx` (новый)
- `web/src/pages/findings.tsx`

**Объём:** ~35 мин

---

### P3 — Низкий приоритет (polish)

#### 2.9 Toast уведомления

**Действия:**
- [ ] Подключить `sonner` или `react-hot-toast` (лёгкие, ~2KB)
- [ ] Показывать при ошибках API
- [ ] Показывать при успешном экспорте

**Файлы:** `web/src/lib/toast.ts`, `web/src/components/ui/toaster.tsx`
**Объём:** ~20 мин

#### 2.10 Confirmation Dialog

**Действия:**
- [ ] Создать `ConfirmDialog` компонент
- [ ] Использовать перед деструктивными действиями (если появятся)

**Файлы:** `web/src/components/ui/confirm-dialog.tsx`
**Объём:** ~15 мин

#### 2.11 React Query DevTools

**Действия:**
- [ ] Добавить `@tanstack/react-query-devtools` (dev-only)
- [ ] Удобство для отладки запросов

**Файлы:** `web/src/App.tsx`
**Объём:** ~5 мин

---

## 3. Технические детали

### 3.1 Пакеты для установки

| Пакет | Назначение | Размер | Приоритет |
|-------|-----------|--------|-----------|
| `sonner` | Toast уведомления | ~2KB | P3 |
| `@tanstack/react-query-devtools` | Dev отладка запросов | dev-only | P3 |

**Остальное уже есть в проекте.** Дополнительные пакеты для графиков, экспорта, состояния не нужны.

### 3.2 Файлы для создания

| Файл | Назначение | Размер кода |
|------|-----------|-------------|
| `components/ui/theme-toggle.tsx` | Кнопка переключения темы | ~20 строк |
| `components/ui/progress-bar.tsx` | Переиспользуемый Progress Bar | ~30 строк |
| `components/ui/pagination.tsx` | Пагинация | ~40 строк |
| `components/ui/confirm-dialog.tsx` | Диалог подтверждения | ~50 строк |
| `components/shared/export-button.tsx` | Экспорт JSON/CSV | ~40 строк |
| `components/ui/toaster.tsx` | Контейнер для toast | ~10 строк |

### 3.3 Файлы для изменения

| Файл | Что изменить | Объём |
|------|-------------|-------|
| `pages/tts.tsx` | Убрать кастомный layout, использовать общий | -50/+10 строк |
| `pages/findings.tsx` | Добавить search input, export button, pagination | +30 строк |
| `pages/providers.tsx` | Добавить сортировку таблицы, export button | +20 строк |
| `components/layout/header.tsx` | Добавить theme toggle кнопку | +10 строк |
| `components/dashboard/providers-chart.tsx` | Анимация, улучшенный tooltip | +15 строк |
| `components/dashboard/findings-by-source-chart.tsx` | Анимация, улучшенный tooltip | +15 строк |
| `components/tts/tts-card.tsx` | Добавить progress bars | +25 строк |
| `App.tsx` | Добавить Toaster, DevTools (dev) | +5 строк |

### 3.4 Общая оценка объёма

| Приоритет | Кол-во файлов | Оценка времени |
|-----------|--------------|----------------|
| P0 (TTS layout fix) | 1 | 30 мин |
| P1 (Theme, Export, TTS bars) | 4 новых + 3 изменённых | ~2 часа |
| P2 (Search, Sort, Charts, Pagination) | 2 новых + 4 изменённых | ~2 часа |
| P3 (Toast, Dialog, DevTools) | 3 новых + 1 изменённый | ~40 мин |
| **Итого** | **~12 файлов** | **~5 часов** |

---

## 4. Архитектурные принципы

1. **API contract не меняется** — все улучшения на уровне UI
2. **Компонентный подход** — каждый feature = отдельный переиспользуемый компонент
3. **React Query остаётся** — кэширование, auto-refetch, optimistic updates
4. **Zustand для UI-состояния** — sidebar, theme, search (не для серверных данных)
5. **Tailwind CSS-переменные** — все цвета через `var(--*)`, никаких захардкоженных hex
6. **Респонсив** — mobile-first, breakpoints: sm/md/lg/xl
7. **Accessibility** — aria-labels, keyboard navigation, focus states

---

## 5. Порядок реализации (рекомендация)

```
1. P0: TTS layout fix (30 мин)
2. P1: Theme toggle (20 мин)
3. P1: Export buttons (40 мин)
4. P1: TTS progress bars (30 мин)
5. P2: Findings search (15 мин)
6. P2: Providers sorting (25 мин)
7. P2: Dashboard charts polish (30 мин)
8. P2: Findings pagination (35 мин)
9. P3: Toast + DevTools (25 мин)
10. P3: Confirm dialog (15 мин)
```

---

## 6. Риски и митигация

| Риск | Митигация |
|------|-----------|
| TTS layout fix сломает существующие стили | Тестить визуально после изменений |
| Экспорт больших datasets тормозит UI | Использовать Blob + URL, не рендерить в DOM |
| Пагинация сломает фильтрацию | Фильтровать на клиенте, пагинировать после |
| Sonner конфликтует с Tailwind | Sonner стилится через inline-стили, совместим |
