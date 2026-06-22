# Free API Hunter — Frontend Specification

> **Версия:** 1.0.0
> **Дата:** 2026-06-22
> **Автор:** Штрейкбрехер (streikbrecher)
> **Статус:** Draft → Review

---

## 1. Обзор

Веб-интерфейс для Free API Hunter — визуальный дашборд каталога бесплатных LLM API. Пользователь видит провайдеров, модели, лимиты, статусы, графики и историю изменений в реальном времени.

**Цель:** сделать каталог бесплатных LLM API доступным, красивым и полезным для разработчиков, исследователей и AI-агентов.

**Принципы:**
- Тёмная тема по умолчанию (dark mode first)
- Минимализм — никакого визуального мусора
- Мобильная адаптивность (responsive)
- Быстрая загрузка (SPA, code splitting)
- Доступность (a11y, keyboard navigation, ARIA)

---

## 2. Стек технологий

**Фреймворк:** React 18+ с TypeScript

Почему React:
- Самая большая экосистема (компоненты, библиотеки, документация)
- shadcn/ui построен на React
- Лёгкая интеграция с Vite
- Поддержка Suspense, Server Components

**Сборка:** Vite 6+

Почему Vite:
- Мгновальный HMR (hot module replacement)
- Встроенная поддержка TypeScript
- Tailwind CSS интеграция из коробки
- Быстрая production-сборка

**Стили:** Tailwind CSS 4+

Почему Tailwind:
- Utility-first подход — быстрая итерация
- Нулевой runtime (скомпилирован в чистый CSS)
- shadcn/ui использует Tailwind
- Тёмная тема через `dark:` префикс и CSS variables

**Компоненты:** shadcn/ui (Radix UI + Tailwind)

Почему shadcn/ui:
- Не библиотека, а набор копируемых компонентов — полный контроль
- Построен на Radix UI (доступность из коробки)
- Кастомизируемый через Tailwind
- Тёмная тема нативно

**Графики:** Recharts 3.x

Почему Recharts:
- Декларативный React API
- Компонентный подход (переиспользуемые графики)
- Лёгкий (~450KB)
- Адаптивный контейнер (ResponsiveContainer)

**Управление состоянием:** TanStack Query (React Query) 5.x

Почему TanStack Query:
- Кэширование API-запросов из коробки
- Автоматический refetch
- Optimistic updates
- Devtools

**Роутинг:** React Router 7.x

**HTTP-клиент:** Fetch API (нативный, через TanStack Query)

**Анимации:** Framer Motion (опционально, для микроанимаций)

**Шрифт:** JetBrains Mono (для кода/данных) + Inter (для текста)

---

## 3. Структура проекта

```
web/
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
├── tailwind.config.ts
├── postcss.config.js
├── public/
│   ├── favicon.svg
│   └── og-image.png
├── src/
│   ├── main.tsx                    # Точка входа
│   ├── App.tsx                     # Корневой компонент
│   ├── env.ts                      # Переменные окружения
│   ├── api/
│   │   ├── client.ts               # HTTP-клиент (fetch wrapper)
│   │   ├── providers.ts            # API-запросы провайдеров
│   │   ├── findings.ts             # API-запросы находок
│   │   └── stats.ts                # API-запросы статистики
│   ├── components/
│   │   ├── ui/                     # shadcn/ui компоненты
│   │   │   ├── badge.tsx
│   │   │   ├── button.tsx
│   │   │   ├── card.tsx
│   │   │   ├── dialog.tsx
│   │   │   ├── dropdown-menu.tsx
│   │   │   ├── input.tsx
│   │   │   ├── select.tsx
│   │   │   ├── skeleton.tsx
│   │   │   ├── table.tsx
│   │   │   ├── tabs.tsx
│   │   │   └── tooltip.tsx
│   │   ├── layout/
│   │   │   ├── header.tsx          # Шапка с навигацией
│   │   │   ├── sidebar.tsx         # Боковая навигация
│   │   │   ├── footer.tsx          # Подвал
│   │   │   └── theme-toggle.tsx    # Переключатель темы
│   │   ├── dashboard/
│   │   │   ├── stats-cards.tsx     # Карточки статистики
│   │   │   ├── providers-chart.tsx # График провайдеров
│   │   │   ├── models-chart.tsx    # График моделей
│   │   │   └── recent-findings.tsx # Последние находки
│   │   ├── providers/
│   │   │   ├── provider-card.tsx   # Карточка провайдера
│   │   │   ├── provider-table.tsx  # Таблица провайдеров
│   │   │   ├── provider-detail.tsx # Детальная страница
│   │   │   ├── provider-filters.tsx # Фильтры
│   │   │   └── status-badge.tsx    # Бейдж статуса
│   │   ├── findings/
│   │   │   ├── finding-card.tsx    # Карточка находки
│   │   │   ├── finding-list.tsx    # Список находок
│   │   │   └── finding-filters.tsx # Фильтры
│   │   └── shared/
│   │       ├── loading-spinner.tsx
│   │       ├── error-boundary.tsx
│   │       ├── empty-state.tsx
│   │       └── page-header.tsx
│   ├── hooks/
│   │   ├── use-providers.ts        # Хук загрузки провайдеров
│   │   ├── use-findings.ts         # Хук загрузки находок
│   │   ├── use-stats.ts            # Хук статистики
│   │   └── use-debounce.ts         # Хук дебаунса
│   ├── lib/
│   │   ├── utils.ts                # Утилиты (cn, formatDate, etc.)
│   │   ├── constants.ts            # Константы
│   │   └── types.ts                # TypeScript типы
│   ├── pages/
│   │   ├── dashboard.tsx           # / — Главная страница
│   │   ├── providers.tsx           # /providers — Список провайдеров
│   │   ├── provider-detail.tsx     # /providers/:name — Детали
│   │   ├── findings.tsx            # /findings — Список находок
│   │   └── not-found.tsx           # /404 — Не найдено
│   └── styles/
│       └── globals.css             # Глобальные стили + Tailwind
```

---

## 4. Дизайн-система

### Цветовая палитра (тёмная тема)

```css
:root {
  /* Фоны */
  --background: #0a0a0a;        /* Основной фон */
  --surface: #111111;           /* Карточки, панели */
  --surface-hover: #1a1a1a;     /* Hover на карточках */
  --surface-active: #222222;    /* Активный элемент */

  /* Текст */
  --foreground: #fafafa;        /* Основной текст */
  --muted: #71717a;             /* Вторичный текст */
  --accent: #3b82f6;            /* Акцент (синий) */

  /* Статусы */
  --success: #22c55e;           /* Verified */
  --warning: #f59e0b;           /* Confirmed */
  --info: #3b82f6;              /* Claimed */
  --error: #ef4444;             /* Expired */
  --neutral: #71717a;           /* Unverified */

  /* Границы */
  --border: #27272a;            /* Разделители */
  --border-hover: #3f3f46;      /* Hover на границах */

  /* Тени */
  --shadow-sm: 0 1px 2px rgba(0,0,0,0.3);
  --shadow-md: 0 4px 12px rgba(0,0,0,0.4);
  --shadow-lg: 0 8px 24px rgba(0,0,0,0.5);

  /* Скругления */
  --radius-sm: 6px;
  --radius-md: 8px;
  --radius-lg: 12px;
  --radius-xl: 16px;
}
```

### Светлая тема

```css
.light {
  --background: #ffffff;
  --surface: #f4f4f5;
  --surface-hover: #e4e4e7;
  --surface-active: #d4d4d8;
  --foreground: #09090b;
  --muted: #71717a;
  --border: #e4e4e7;
  --border-hover: #d4d4d8;
}
```

### Типографика

```
Заголовок H1:  Inter, 36px, font-weight: 700, line-height: 1.2
Заголовок H2:  Inter, 24px, font-weight: 600, line-height: 1.3
Заголовок H3:  Inter, 18px, font-weight: 600, line-height: 1.4
Тело:          Inter, 14px, font-weight: 400, line-height: 1.6
Моно (код):    JetBrains Mono, 13px, font-weight: 400
Мелкий текст:  Inter, 12px, font-weight: 400
```

### Статусы провайдеров (бейджи)

```
Verified:      🟢 зелёный фон (#22c55e/20), зелёный текст, иконка ✓
Confirmed:     🟡 жёлтый фон (#f59e0b/20), жёлтый текст, иконка ◉
Claimed:       🔵 синий фон (#3b82f6/20), синий текст, иконка ○
Unverified:    ⚪ серый фон (#71717a/20), серый текст, иконка ◌
Expired:       🔴 красный фон (#ef4444/20), красный текст, иконка ✕
Deprioritized: ⛔ серый фон, зачёркнутый текст
```

---

## 5. Страницы и компоненты

### 5.1 Dashboard (Главная) — `/`

**Назначение:** Обзорная панель с ключевыми метриками и последними данными.

**Структура:**

```
┌─────────────────────────────────────────────────────────┐
│  Header: Logo | 🔍 Search | 🌙 Theme | ⚙️ Settings     │
├──────────┬──────────────────────────────────────────────┤
│          │  Stats Cards (4 колонки)                     │
│ Sidebar  │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐        │
│          │  │  17  │ │  7   │ │ 156  │ │  42  │        │
│ Dashboard│  │Total │ │Verif.│ │Models│ │No CC │        │
│ Providers│  └──────┘ └──────┘ └──────┘ └──────┘        │
│ Findings │                                              │
│ Stats    │  ┌────────────────────┐ ┌─────────────────┐  │
│          │  │ Providers by Status│ │ Models per Prov │  │
│          │  │    (Pie Chart)     │ │  (Bar Chart)    │  │
│          │  └────────────────────┘ └─────────────────┘  │
│          │                                              │
│          │  ┌─────────────────────────────────────────┐ │
│          │  │  Recent Findings (Last 10)              │ │
│          │  │  ┌───────────────────────────────────┐  │ │
│          │  │  │ 🆕 Ollama Cloud — free tier      │  │ │
│          │  │  │    Score: 0.7 | Source: costgoat  │  │ │
│          │  │  ├───────────────────────────────────┤  │ │
│          │  │  │ 🆕 ModelScope — 2000 req/day      │  │ │
│          │  │  │    Score: 0.7 | Source: github    │  │ │
│          │  │  └───────────────────────────────────┘  │ │
│          │  └─────────────────────────────────────────┘ │
└──────────┴──────────────────────────────────────────────┘
```

**Компоненты:**

- `StatsCards` — 4 карточки с ключевыми числами (всего провайдеров, verified, моделей, без кредитки)
- `ProvidersByStatusChart` — круговая диаграмма (Recharts PieChart)
- `ModelsPerProviderChart` — столбчатая диаграмма (Recharts BarChart)
- `RecentFindings` — список последних 10 находок с бейджами

**API-запросы:**
- `GET /api/v1/stats` — статистика для карточек и графиков
- `GET /api/v1/findings?limit=10` — последние находки

---

### 5.2 Список провайдеров — `/providers`

**Назначение:** Полный каталог провайдеров с фильтрацией и поиском.

**Структура:**

```
┌─────────────────────────────────────────────────────────┐
│  Filters: [Status ▾] [Credit Card ▾] [Search...    🔍] │
│                                                         │
│  View: [Table ▦] [Cards ▤]                              │
│                                                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Provider     │ Status │ Models │ Limits │ CC    │  │
│  ├───────────────┼────────┼────────┼────────┼───────┤  │
│  │  OpenRouter   │ 🟢 Ver │   27   │ 200/d  │  No   │  │
│  │  Google AI    │ 🟢 Ver │    4   │ 1500/d │  No   │  │
│  │  Groq         │ 🟢 Ver │    5   │ 1000/d │  No   │  │
│  │  Cerebras     │ 🟢 Ver │    2   │ 14400/d│  No   │  │
│  │  Mistral      │ 🟢 Ver │    6   │ 1B/mo  │  No   │  │
│  │  ...          │        │        │        │       │  │
│  └───────────────────────────────────────────────────┘  │
│                                                         │
│  Showing 17 of 17 providers    [< 1 2 3 >]              │
└─────────────────────────────────────────────────────────┘
```

**Фильтры:**
- По статусу (verified, confirmed, claimed, unverified, expired)
- По наличию кредитной карты (yes/no/any)
- Поиск по имени (debounced, 300ms)
- Сортировка (по имени, статусу, количеству моделей)

**Компоненты:**
- `ProviderFilters` — панель фильтров
- `ProviderTable` — табличный вид
- `ProviderCards` — карточный вид (grid)
- `StatusBadge` — цветной бейдж статуса
- `CreditCardIcon` — иконка кредитной карты

**API-запросы:**
- `GET /api/v1/providers` — список с фильтрами
- `GET /api/v1/providers?name=openrouter` — поиск

---

### 5.3 Детали провайдера — `/providers/:name`

**Назначение:** Полная информация о конкретном провайдере.

**Структура:**

```
┌─────────────────────────────────────────────────────────┐
│  ← Back to providers                                    │
│                                                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │  OpenRouter (free models)                    🟢    │  │
│  │  https://openrouter.ai                            │  │
│  │  API Keys: https://openrouter.ai/keys             │  │
│  │  Docs: https://openrouter.ai/docs/free-models     │  │
│  │                                                   │  │
│  │  Status: Verified | Source: manual                │  │
│  │  Discovered: 2026-06-18 | Last verified: 2026-06  │  │
│  └───────────────────────────────────────────────────┘  │
│                                                         │
│  ┌─────────────────────┐ ┌────────────────────────────┐ │
│  │ Limits              │ │ Models (27)                │ │
│  │ RPM: 20             │ │ • gpt-oss-120b             │ │
│  │ RPD: 200            │ │ • claude-3-haiku           │ │
│  │                     │ │ • llama-3.1-70b            │ │
│  │ No credit card      │ │ • ...                      │ │
│  └─────────────────────┘ └────────────────────────────┘ │
│                                                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Context Length Distribution                      │  │
│  │  ████████████████████ 128K+  (8 models)           │  │
│  │  ██████████████ 64K-128K (12 models)              │  │
│  │  ████████ 32K-64K (7 models)                      │  │
│  └───────────────────────────────────────────────────┘  │
│                                                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Related Findings                                 │  │
│  │  • "OpenRouter adds 5 new free models" (HN, 0.8)  │  │
│  │  • "OR free tier limits updated" (blog, 0.6)      │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

**API-запросы:**
- `GET /api/v1/providers/:name` — детали провайдера
- `GET /api/v1/findings?source=:source` — связанные находки

---

### 5.4 Находки — `/findings`

**Назначение:** Лента обнаруженных находок с фильтрацией.

**Структура:**

```
┌─────────────────────────────────────────────────────────┐
│  Filters: [Source ▾] [Min Score ▾] [Search...     🔍]  │
│                                                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Score: 0.9  │  🆕 27 Free Models on OpenRouter   │  │
│  │  Source: costgoat  │  2026-06-22                  │  │
│  │  "CostGoat live ranking shows 27 free models..."  │  │
│  │  [View Source →]                                  │  │
│  ├───────────────────────────────────────────────────┤  │
│  │  Score: 0.7  │  🆕 Ollama Cloud Free Tier        │  │
│  │  Source: hackernews  │  2026-06-21                │  │
│  │  "Ollama launches cloud hosting with free..."     │  │
│  │  [View Source →]                                  │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

**API-запросы:**
- `GET /api/v1/findings` — список с фильтрами и пагинацией

---

### 5.5 Статистика — `/stats`

**Назначение:** Детальная аналитика и графики.

**Компоненты:**
- `ProvidersTimelineChart` — таймлаин открытия провайдеров
- `ModelsGrowthChart` — рост количества моделей
- `FindingsBySourceChart` — находки по источникам
- `ScoreDistributionChart` — распределение скоринга
- `ContextLengthDistribution` — распределение по длине контекста

---

## 6. Компоненты shadcn/ui (список)

Используемые компоненты из shadcn/ui:

```
Badge          — Статусы провайдеров, скоринг
Button         — Действия, фильтры
Card           — Карточки провайдеров, находок
Dialog         — Модальные окна деталей
DropdownMenu   — Фильтры, меню
Input          — Поиск
Select         — Выбор фильтров
Table          — Таблица провайдеров
Tabs           — Переключение видов (table/cards)
Tooltip        — Подсказки
Skeleton       — Лоадеры
Separator      — Разделители
Avatar         — Иконки провайдеров (первые буквы)
```

---

## 7. API интеграция

Бэкенд уже предоставляет:

```
GET /health                    — Health check
GET /api/v1/providers          — Список провайдеров (?status=&credit_card=&name=)
GET /api/v1/providers/:name    — Провайдер по имени
GET /api/v1/findings           — Список находок (?source=&limit=&offset=)
GET /api/v1/stats              — Статистика
GET /metrics                   — Prometheus метрики (JSON)
```

**Клиентский API-слой:**

```typescript
// src/api/client.ts
const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

async function fetchJSON<T>(path: string, params?: Record<string, string>): Promise<T> {
  const url = new URL(`${API_BASE}${path}`);
  if (params) Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v));
  const res = await fetch(url.toString());
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

// src/api/providers.ts
export function getProviders(filters?: ProviderFilters) {
  return fetchJSON<Provider[]>('/api/v1/providers', filters);
}

export function getProvider(name: string) {
  return fetchJSON<Provider>(`/api/v1/providers/${encodeURIComponent(name)}`);
}

// src/api/findings.ts
export function getFindings(filters?: FindingFilters) {
  return fetchJSON<Finding[]>('/api/v1/findings', filters);
}

// src/api/stats.ts
export function getStats() {
  return fetchJSON<Stats>('/api/v1/stats');
}
```

---

## 8. Типы данных (TypeScript)

```typescript
// src/lib/types.ts

interface Provider {
  name: string;
  url: string;
  api_key_url: string;
  credit_card: boolean;
  status: 'verified' | 'confirmed' | 'claimed' | 'unverified' | 'expired' | 'deprioritized';
  models: string[];
  limits: string;
  notes: string;
  source: string;
  priority: number;
  discovered_at: string;
  last_verified?: string;
}

interface Finding {
  source_id: string;
  title: string;
  url: string;
  description: string;
  raw_text: string;
  discovered_at: string;
  provider_name?: string;
  is_duplicate: boolean;
  quality_score: number;
  filtered_out: boolean;
  filter_reason: string;
}

interface Stats {
  providers_total: number;
  providers_by_status: Record<string, number>;
  providers_no_cc: number;
  findings_total: number;
  findings_by_source: Record<string, number>;
  models_total: number;
  server_time: string;
}

interface ProviderFilters {
  status?: string;
  credit_card?: 'true' | 'false';
  name?: string;
  sort_by?: 'name' | 'status' | 'models_count';
  sort_order?: 'asc' | 'desc';
}

interface FindingFilters {
  source?: string;
  min_score?: number;
  limit?: number;
  offset?: number;
}
```

---

## 9. Анимации и микроинтеракции

**Framer Motion анимации:**

- `layout` — плавная перестройка карточек при фильтрации
- `initial={{ opacity: 0, y: 10 }}` — появление карточек снизу
- `whileHover={{ scale: 1.02 }}` — увеличение карточки при наведении
- `AnimatePresence` — анимация удаления элементов из списка
- `staggerChildren: 0.05` — каскадное появление элементов списка

**Переходы:**

- Смена темы: плавный transition цветов (300ms ease)
- Переход между страницами: fade + slide (200ms)
- Открытие деталей: scale + fade из центра карточки
- Лоадер: pulsing skeleton с shimmer-эффектом

---

## 10. Адаптивность

**Breakpoints:**

```
sm:  640px   — Мобильные (1 колонка)
md:  768px   — Планшеты (2 колонки)
lg:  1024px  — Десктоп (3 колонки + sidebar)
xl:  1280px  — Широкий десктоп (4 колонки)
```

**Мобильная адаптация:**

- Sidebar сворачивается в bottom navigation
- Таблица провайдеров превращается в карточки
- Фильтры в выдвижной панели (sheet/drawer)
- Графики упрощаются (меньше данных, вертикальные бары)

---

## 11. Производительность

**Оптимизации:**

- Code splitting по роутам (React.lazy + Suspense)
- Виртуализация длинных списков (TanStack Virtual)
- Кэширование API-запросов (TanStack Query, staleTime: 5min)
- Дебаунс поиска (300ms)
- Skeleton-загрузчики вместо спиннеров
- Ленивая загрузка графиков (динамический импорт Recharts)
- Prefetch данных при наведении на ссылку

**Метрики (цели):**

- First Contentful Paint: < 1.5s
- Largest Contentful Paint: < 2.5s
- Time to Interactive: < 3s
- Cumulative Layout Shift: < 0.1
- Lighthouse Performance: > 90

---

## 12. Доступность (a11y)

- Все интерактивные элементы доступны с клавиатуры (Tab, Enter, Escape)
- ARIA-лейблы для всех иконок и кнопок
- Цветовой контраст ≥ 4.5:1 (WCAG AA)
- Skip-to-content ссылка
- Focus indicators на всех интерактивных элементах
- Screen reader поддержка (role, aria-live для динамического контента)
- Reduced motion поддержка (prefers-reduced-motion)

---

## 13. Безопасность

- CSP (Content Security Policy) заголовки
- XSS-защита через React (автоматическое экранирование)
- API URL через environment variables
- Нет хранения секретов в клиенте
- Rate limiting на API (backoff при 429)

---

## 14. Деплой

**Вариант 1: Docker (рекомендуемый)**

```dockerfile
FROM node:22-alpine AS builder
WORKDIR /app
COPY web/ .
RUN npm ci && npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

**Вариант 2: Vercel/Netlify**

- Автоматический деплой при push в main
- Preview deployments для PR
- Edge functions для API proxy

**Вариант 3: Встроенный в Go-бинарник**

- Статические файлы встраиваются в бинарник через `embed`
- Единый бинарник: API + фронтенд
- Идеально для self-hosting

---

## 15. Дорожная карта реализации

**Этап 1 — Скаркас (1-2 дня):**
- Инициализация проекта (Vite + React + TypeScript)
- Настройка Tailwind CSS
- Установка shadcn/ui компонентов
- Базовый layout (header, sidebar, footer)
- Роутинг (React Router)
- API-клиент

**Этап 2 — Dashboard (1-2 дня):**
- StatsCards компонент
- ProvidersByStatusChart (PieChart)
- ModelsPerProviderChart (BarChart)
- RecentFindings список
- Интеграция с API

**Этап 3 — Провайдеры (2-3 дня):**
- ProviderTable (табличный вид)
- ProviderCards (карточный вид)
- ProviderFilters (фильтрация и поиск)
- ProviderDetail (детальная страница)
- Пагинация

**Этап 4 — Находки (1-2 дня):**
- FindingCard компонент
- FindingList с фильтрами
- Пагинация / infinite scroll

**Этап 5 — Статистика (1-2 дня):**
- Детальные графики
- Таймлаин провайдеров
- Распределение по скорингу

**Этап 6 — Полировка (1-2 дня):**
- Анимации (Framer Motion)
- Мобильная адаптивность
- Тёмная/светлая тема
- Доступность
- Оптимизация производительности
- Тесты (Vitest + React Testing Library)

**Итого: 7-13 дней**

---

## 16. Зависимости (package.json)

```json
{
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "react-router-dom": "^7.0.0",
    "@tanstack/react-query": "^5.60.0",
    "@tanstack/react-virtual": "^3.10.0",
    "recharts": "^3.8.0",
    "framer-motion": "^11.15.0",
    "lucide-react": "^0.460.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.5.0"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vite": "^6.0.0",
    "@vitejs/plugin-react": "^4.3.0",
    "tailwindcss": "^4.0.0",
    "@tailwindcss/vite": "^4.0.0",
    "vitest": "^2.1.0",
    "@testing-library/react": "^16.1.0",
    "@testing-library/jest-dom": "^6.6.0"
  }
}
```

---

## 17. Переменные окружения

```env
# .env
VITE_API_URL=http://localhost:8080
VITE_APP_TITLE=Free API Hunter
VITE_APP_VERSION=0.7.0
```

---

_Этот спек — основа для реализации. Каждый этап можно запускать независимо. Готов начать кодить по команде._
