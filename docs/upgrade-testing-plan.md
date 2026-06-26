# План апгрейда тестирования и CI/CD для free-api-hunter

**Дата:** 2026-06-26  
**Автор:** Исследование subagent  
**Статус:** Draft (план действий)

---

## 1. Текущее состояние тестов

### 1.1 Актуальные метрики покрытия

Результат `go test ./... -cover -count=1`:

| Пакет | Покрытие | Статус |
|-------|----------|--------|
| `models` | **100.0%** | ✅ Отлично |
| `filter` | **91.1%** | ✅ Хорошо |
| `ocr` | **90.8%** | ✅ Хорошо |
| `verifier` | **90.2%** | ✅ Хорошо |
| `vault` | **85.7%** | ⚠️ Ниже цели |
| `orex` | **86.1%** | ⚠️ Ниже цели |
| `storage` | **83.2%** | ⚠️ Ниже цели |
| `tts` | **65.8%** | ❌ Слабо |
| `alerter` | **46.5%** | ❌ Очень слабо |
| `api` | **38.2%** | ❌ Очень слабо |
| `pollinations` | **12.9%** | ❌ Критично |
| `scraper` | **6.7%** | ❌ Критично |
| `cmd/hunter` | **0.0%** | ❌ Нет тестов |

### 1.2 Пакеты с покрытиением < 80%

1. **`cmd/hunter` (0%)** — main() не покрыт, нет тестов билда
2. **`scraper` (6.7%)** — только парсинг тестируется, HTTP-клиент — нет
3. **`pollinations` (12.9%)** — только функции классификации моделей покрыты
4. **`api` (38.2%)** — базовые HTTP-хендлеры есть, но нет TTS endpoints, ошибок
5. **`alerter` (46.5%)** — форматирование покрыто, SendTelegram — нет (реальный HTTP)
6. **`tts` (65.8%)** — scorer и verifier покрыты, KeyPool — частично, нет concurrency

### 1.3 Race conditions

- `go test -race ./...` — **падает на build**: `internal/scraper` и `cmd/hunter` не собираются в race-режиме (нет тестовых в _test.go файлах для cmd)
- Сами тесты при сборке **проходят успешно** включая scraper с таймерами
- **Нет специфических race-тестов** для KeyPool (sync.Mutex), storage → concurrent access, filter → seenFingerprints map

### 1.4 Текущая структура тестов

- **37 тестов** в пакете alerter (alerter_test + alerter_extra)
- **18 тестов** в api (server_test + server_extra)
- **14 тестов** в tts (scorer + verifier)
- ~10 тестов в scraper
- ~8 тестов в pollinations
- ~10 тестов в filter
- ~6 тестов в verifier, ocr, orex
- ~4 теста в storage, vault, models

**Итого: ~110 unit-тестов** в проекте.

---

## 2. План улучшения тестирования

### 2.1 Фаза 1: Быстрые wins (довести до 80%+)

#### 2.1.1 scraper (6.7% → 80%+)

**Текущее:** Только isChangelogLine, isProviderEntry, getRandomUserAgent, waitForRedditRateLimit.

**Добавить:**
- `TestFetchURLTimeout` — httptest.NewServer с sleep > timeout → ожидаем ошибку
- `TestFetchURLInvalidURL` — уже есть в scraper_test_extended.go, влить в основной
- `TestScrapeRedditSuccess` — mock Reddit JSON: `/r/search.json?q=API&...` с валидным ответом
- `TestScrapeHNAPI` — mock HN API (Algolia) с валидным JSON
- `TestScrapeGitHubSearch` — mock GitHub code search
- `TestScrapeEmptyResponses` — все источники возвращают пустые ответы
- `TestScrapeRateLimit` — 429 ответ → ожидаем retry/graceful handling
- `TestScrapeNetworkError` — без сети → graceful handling

**Подход:** httptest.NewServer + перехват FetchURL через инъекцию URL. Заменить хардкод-URL на InjectableTransport.

#### 2.1.2 pollinations (12.9% → 80%+)

**Текущее:** isPaidOnlyModel, isNonTextModel, ToProvider, ModelTestResult.

**Добавить:**
- `TestGetModelsMockServer` — httptest NewServer → валидный PollinationsModel[] → парсинг
- `TestGetModelsLegacyFormat` — массив вместо Objects (legacy)
- `TestGetModelsHTTPError` — 500 ответ → ошибка
- `TestTestModelSuccess` — mock /v1/chat/completions успех
- `TestTestModelPaidModel` — ответ с "balance" → IsFree=false
- `TestTestModelAuthError` — 401 → auth_error
- `TestTestModelNotFound` — 404 → model_not_found
- `TestTestModelTimeout` — медленный сервер → таймаут
- `TestToProviderEmptyModels` — ModelsFree=0 → fallback на Models
- `TestIsPaidOnlyModelEdgeCases` — регистр, пустые строки, unicode
- `TestIsNonTextModelCohere` — cohere-embed → non-text, but cohere → text

**Подход:** `httpClient` — переменная пакета, переопределять в тестах через `httpClient = &http.Client{Transport: roundTripperFunc}`

#### 2.1.3 alerter (46.5% → 80%+)

**Текущее:** LoadConfig (vault/fallback/placeholder), Format*, SendTelegram с nil/empty.

**Добавить:**
- `TestSendTelegramSuccess` — httptest → 200 → проверить payload Telegram
- `TestSendTelegramHTTPError` — httptest → 403/500 → ошибка с HTTP code
- `TestSendTelegramNetworkError` — несуществующий URL → ошибка
- `TestFormatTTSReport` — проверка форматирования отчёта
- `TestFormatTTSKeyStatus` — active/inactive статус
- `TestFormatTTSScoreReport` — проверка форматирования
- `TestFormatOrexSyncReport` — проверка форматирования
- `TestLoadConfigNotFound` — vault + file отсутствуют → ошибка
- `TestLoadConfigEmpty` — пустой файл → ошибка

**Подход:** Сделать SendTelegram принимать http.Client для DI.

#### 2.1.4 api (38.2% → 80%+)

**Текущее:** Providers, Findings, Stats, Health, Index — базовые GET.

**Добавить:**
- TTS endpoints: `TestTTSProviders`, `TestTTSProviderByID`, `TestTTSProviderNotFound`, `TestTTSStats`
- Обработка ошибок: `TestHandleProvidersReadError` (fail to read file), `TestHandleFindingsReadError`
- `TestProviderByIDEmptyID` — пустой ID → bad request
- `TestFindingsMultipleFilters` — source + limit combo
- Graceful shutdown: `TestServerGracefulShutdown` — ListenAndServeGraceful с SIGTERM
- `TestMethodNotAllowed` — POST/PUT/DELETE на все endpoints
- `TestProviderByIDSpecialChars` — URL-encoded ID

#### 2.1.5 tts (65.8% → 80%+)

**Текущее:** scorer покрыт хорошо, verifier — базовые кейсы, keypool — round-robin + exhaustion.

**Добавить:**
- `TestKeyPoolConcurrentNext` — 50 goroutines вызывают Next() параллельно → no panic
- `TestKeyPoolConcurrentReportUsage` — concurrent ReportUsage → нет race
- `TestKeyPoolNextForProvider` — NextForProvider с round-robin
- `TestKeyPoolReload` — Reload сохраняет использование ключей
- `TestKeyPoolSaveState` — SaveState → JSON file корректен
- `TestVerifyTTSKeyTimeout` — mock server с задержкой > timeout
- `TestVerifyTTSKeyNoResponseBody` — пустое тело ответа
- `TestVerifyTTSKeyRateLimit` — 429 ответ
- `TestVerifyTTSKeyVoicesList` — проверка что voices парсятся

#### 2.1.6 storage (83.2% → 85%+)

**Текущее:** LoadProviders, SaveProviders, LoadFindings, SaveFindings.

**Добавить:**
- `TestSaveProvidersInvalidPath` — невалидный путь → ошибка
- `TestSaveFindingsInvalidPath` — невалидный путь → ошибка
- `TestSaveTTSScore` — сохранение TTS score в JSON
- `TestLoadTTSScore` — загрузка TTS score
- `TestEnsureDirAlreadyExists` — директория уже существует → no error
- `TestConcurrentReadWrite` — concurrent Load/Save → no race

#### 2.1.7 cmd/hunter (0% → 50%+)

**Добавить:**
- `TestMainVersion` — бинарка с --version → вывод
- `TestMainHelp` — --help → usage output
- `TestBuildFlag` — go build компилируется без ошибок
- `TestCLIArgs` — парсинг CLI-флагов (--api, --data-dir и т.д.)

---

### 2.2 Фаза 2: Интеграционные тесты

#### 2.2.1 Mock HTTP Server для каждого источника

Создать `internal/scraper/mock_server_test.go`:

```go
// TestScrapeRedditAPIMock — полный flow через mock Reddit API
// TestScrapeGitHubMock — полный flow через mock GitHub code search API
// TestScrapeCommunityJSONMock — парсинг JSON-файла с провайдерами
```

Каждый mock server возвращает:
- 200 с валидными данными → ожидаем N findings
- 429 rate limit → ожидаем graceful handling
- 500 server error → ожидаем graceful handling
- Network timeout → ожидаем graceful handling

#### 2.2.2 Integration storage pipeline

`internal/storage/integration_test.go`:

```go
// TestStoragePipeline_FullFlow — SaveProviders → LoadProviders → верификация roundtrip
// TestStoragePipeline_FindingsFlow — SaveFindings → LoadFindings → верификация
// TestStoragePipeline_ConcurrentAccess — 10 goroutines читают/пишут одновременно
```

#### 2.2.3 API server integration

`internal/api/integration_test.go`:

```go
// TestAPI_FullFlow — создать сервер, GET /api/v1/providers?status=verified
// TestAPI_TTSFlow — GET /api/v1/tts/providers, /api/v1/tts/stats
// TestAPI_HealthDuringScan — проверить /health во время активного сканирования
```

#### 2.2.4 Pollinations integration

`internal/pollinations/integration_test.go`:

```go
// TestPollinationsGetModels_FullFlow — mock API → GetModels() → верификация количества
// TestPollinationsTestModel_FullFlow — mock chat endpoint → TestModel() → IsFree
// TestPollinationsTestAllModels_FullFlow — mock API → TestAllModels() → ProviderInfo
```

---

### 2.3 Фаза 3: Специализированные тесты

#### 2.3.1 Fuzzing для filter (`internal/filter/fuzz_test.go`)

```go
go test -fuzz=FuzzFilterEngine -fuzztime=30s
```

Цели fuzzing-тестов:
- `FuzzFilterEngine_Apply` — случайные Findings → нет паник
- `FuzzFingerprint` — случайные строки → fingerprint стабилен
- `FuzzSpamDetection` — случайный текст → детерминированный результат
- `FuzzAssignPriority` — случайные комбинации status + CreditCard → нет паник

#### 2.3.2 Race-тесты для KeyPool (`internal/tts/keypool_race_test.go`)

```go
// TestKeyPoolRace_ParallelNext — go test -race: 100 goroutines, Next()
// TestKeyPoolRace_NextWithReportUsage — Next() + ReportUsage() параллельно
// TestKeyPoolRace_NextWithReportError — Next() + ReportError() параллельно
// TestKeyPoolRace_ReloadWhileActive — Reload() + Next() параллельно
// TestKeyPoolRace_StatsWhileActive — Stats() + Next() параллельно
```

#### 2.3.3 Benchmark для pipeline (`internal/*/bench_test.go`)

```go
// BenchmarkFilterPipeline — 10000 findings → filter throughput (ops/sec)
// BenchmarkKeyPoolNext — Next() latency (ns/op, allocs/op)
// BenchmarkFormatScanReport — format throughput
// BenchmarkScrapeReddit — mock server scrape speed
// BenchmarkStorageLoadProviders — 1000 провайдеров JSON load time
```

#### 2.3.4 Edge-case тесты

```go
// internal/filter/edge_test.go — пустые finding, unicode, very long strings
// internal/storage/edge_test.go — корrupted JSON, empty file, permission denied
// internal/models/edge_test.go — пустые модели, nil slices
// internal/verifier/edge_test.go — 0 latency, very high latency, timeouts
```

---

## 3. CI/CD Pipeline

### 3.1 GitHub Actions — `.github/workflows/ci.yml`

```yaml
name: CI
on:
  push: { branches: [main, develop] }
  pull_request: { branches: [main] }

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - name: golangci-lint
        uses: golangci-lint/action@v6
        with:
          version: latest
          args: --timeout=5m

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: go test ./... -race -count=1 -coverprofile=coverage.out
      - name: Coverage threshold check
        run: |
          go tool cover -func=coverage.out | tail -1
          # Enforce 80% minimum
          COVERAGE=$(go tool cover -func=coverage.out | tail -1 | grep -oP '\d+\.\d+' | tail -1)
          if (( $(echo "$COVERAGE < 80.0" | bc -l) )); then
            echo "Coverage $COVERAGE% is below 80% threshold"
            exit 1
          fi

  build:
    runs-on: ubuntu-latest
    needs: [lint, test]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: go build -ldflags "-X main.Version=${{ github.sha }}" -o bin/hunter ./cmd/hunter
      - uses: actions/upload-artifact@v4
        with: { name: hunter, path: bin/hunter }

  docker:
    runs-on: ubuntu-latest
    needs: [test]
    steps:
      - uses: actions/checkout@v4
      - run: docker build -t free-api-hunter:${{ github.sha }} .
      - run: docker run --rm free-api-hunter:${{ github.sha }} --version
```

### 3.2 Goreleaser — автоматический релиз при теге

`.github/workflows/release.yml`:

```yaml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 3.3 `.goreleaser.yml`

```yaml
project_name: free-api-hunter
builds:
  - env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    main: ./cmd/hunter
    ldflags: -s -w -X main.Version={{.Version}} -X main.Commit={{.Commit}}
archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip
dockers:
  - image_templates: ["ghcr.io/{{ .Env.GITHUB_REPOSITORY }}:{{ .Tag }}"]
    dockerfile: Dockerfile
    build_flag_templates: ["--label=org.opencontainers.image.version={{.Version}}"]
checksum:
  name_template: "checksums.txt"
changelog:
  sort: asc
  filters:
    exclude: ["^docs:", "^test:", "^* ": ]
```

---

## 4. Dockerfile — Multi-stage Upgrade

### 4.1 Текущий этап

Уже multi-stage (builder + alpine). Но есть проблемы:
- Используем `git describe` — в Docker layer может не работать
- `go.sum` (пусто? — wc -l go.sum) может не совпадать с go.mod
- Нет static linking проверки
- Нет non-root для конфига

### 4.2 Рекомендации

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app

# Кэш dependencies
COPY go.* ./
RUN go mod download && go mod verify

# Build with version injection
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.Version=${VERSION} -X main.Commit=${COMMIT} -s -w" \
    -a -installsuffix cgo \
    -o /hunter ./cmd/hunter

# Security scan
FROM aquasec/trivy:latest AS scanner
COPY --from=builder /hunter /hunter
RUN trivy fs --severity HIGH,CRITICAL --exit-code 0 /hunter

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=builder /hunter /hunter
COPY --from=builder /app/configs/ /configs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

ENV HUNTER_DATA_DIR=/data
ENV HUNTER_CONFIG_DIR=/configs

VOLUME ["/data"]
EXPOSE 8080
USER:nonroot:nonroot
ENTRYPOINT ["/hunter"]
CMD ["--api", ":8080"]
```

---

## 5. Качество кода

### 5.1 golangci-lint — `.golangci.yml`

```yaml
run:
  timeout: 5m
  go: '1.23'
output:
  formats: colored-line-number
linters:
  enable:
    # Default
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    # Extra
    - gofmt
    - goimports
    - gosec
    - misspell
    - unconvert
    - unparam
    - gocyclo
    - depguard
    - dupl
    - bodyclose
    - gocognit
    - gocritic
    - goconst
    - errorlint
    - revive
linters-settings:
  gocyclo:
    min-complexity: 15
  gocognit:
    min-complexity: 20
  depguard:
    rules:
      main:
        allow:
          - $gostd
          - free-api-hunter
        deny:
          - pkg: "github.com/pkg/errors"
            desc: "use stdlib errors"
    forbid:
      - pkg: "reflect"
        desc: "avoid reflection in hot paths"
  gocritic:
    enabled-tags:
      - diagnostic
      - style
      - performance
      - experimental
  gosec:
    excludes:
      - G104 # errcheck is used instead
issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gocyclo
        - gocognit
        - dupl
        - gosec
    - linters:
        - staticcheck
      text: "SA1019:"
```

### 5.2 Makefile

```makefile
.PHONY: all test lint build clean

all: lint test build

test:
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...
	go vet ./...

fmt:
	goimports -w .
	gofmt -w .

build:
	CGO_ENABLED=0 go build -ldflags "-X main.Version=dev" -o bin/hunter ./cmd/hunter

clean:
	rm -rf bin/ coverage.out coverage.html
```

### 5.3 Pre-commit hooks — `.pre-commit-config.yaml`

```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-json
      - id: check-added-large-files
      - id: detect-private-key
      - id: no-commit-to-branch

  - repo: https://github.com/golangci/golangci-lint
    rev: v1.63.0
    hooks:
      - id: golangci-lint-full

  - repo: local
    hooks:
      - id: go-test
        name: go test (cached)
        entry: go test ./... -count=1 -race
        language: system
        pass_filenames: false
        always_run: true

      - id: go-fmt
        name: gofmt
        entry: gofmt -l .
        language: system
        types: [go]
        pass_filenames: false
```

### 5.4 Дополнительное форматирование

- **goimports** — запускать в CI (проверка колонки не обязательно, diff-only)
- **gofmt -s** — упрощение синтаксиса
- **go mod tidy** — проверка чистоты go.mod в CI (задача `go mod tidy && git diff --exit-code`)

---

## 6. Приоритеты и Timeline

### Sprint 1 (Неделя 1) — Foundation

| Задача | Пакеты | Effort | Покрытие после |
|--------|--------|--------|-----------------|
| Fuzzing + edge cases для filter | filter | 2h | 93% |
| Race-тесты для KeyPool | tts | 2h | 72% |
| Concurrent storage тесты | storage | 1h | 87% |
| TTS endpoint тесты в api | api | 2h | 55% |
| SendTelegram mock тесты | alerter | 1h | 55% |
| **Итого** | | **8h** | |

### Sprint 2 (Неделя 2) — Integration & Mock

| Задача | Пакеты | Effort | Покрытие после |
|--------|--------|--------|-----------------|
| Mock servers для scraper | scraper | 4h | 70% |
| Интеграционные pollinations | pollinations | 3h | 60% |
| Pipeline benchmarks | all | 2h | — |
| API error handling тесты | api | 2h | 65% |
| **Итого** | | **11h** | |

### Sprint 3 (Неделя 3) — Polish & CI/CD

| Задача | Пакеты | Effort | Покрытие после |
|--------|--------|--------|-----------------|
| Довести все до 80%+ | all remaining | 6h | 80%+ |
| golangci-lint setup | project | 2h | — |
| GitHub Actions CI | project | 2h | — |
| Pre-commit hooks | project | 1h | — |
| Goreleaser config | project | 2h | — |
| Dockerfile upgrade | project | 1h | — |
| **Итого** | | **14h** | |

---

## 7. Ожидаемый результат

| Метрика | Текущее | После плана |
|---------|---------|-------------|
| Общее покрытие | ~70% (среднее) | **>85%** |
| Пакетов < 80% | 6 | **0** |
| Race-тесты | 0 | **10+** |
| Fuzzing-тесты | 0 | **4+** |
| Benchmark | 0 | **5+** |
| Интеграционные тесты | 0 | **10+** |
| CI/CD | базовый Build+Test | **Full: lint + race + build + docker** |
| Авто-релиз | нет | **Goreleaser при теге** |
| Makefile | нет | **all-in-one** |
| Pre-commit | нет | **gofmt + lint + test** |
| Linting | go vet | **golangci-lint (30+ linters)** |

---

## 8. Заметки и риски

### 8.1 Код-смелл для рефакторинга

1. **`scraper.FetchURL`** — использует глобальный `http.Client`, не инжектируется. Для тестов нужен `var httpClient = &http.Client{}` свозможностью замены.
2. **`alerter.SendTelegram`** — хардкод-URL `api.telegram.org`, нужен `http.Client` DI.
3. **`pollinations.httpClient`** — уже переменная пакета, можно переопределять в тестах ✅
4. **`storage.DataDir`** — переменная пакета, тесты уже переопределяют ✅
5. **`vault.VaultPath`** — переменная пакета ✅
6. **Нет interface для HTTP-клиентов** — scraper, pollinations, alerter используют напрямую `*http.Client`. Рекомендуется ввести интерфейсы `HTTTPClient { Do(req) (*Response, error) }`.

### 8.2 go.sum

Файл `go.sum` существует но может быть неполным. Рекомендуется:
```bash
go mod tidy
go mod verify
```

### 8.3 Зависимости

В `go.mod` нет внешних зависимостей (только `free-api-hunter`). Это означает:
- Нет проблем с версиями для golangci-lint
- Быстрая сборка в CI
- Минимальный supply-chain риск

### 8.4 Сетевые зависимости

Некоторые тесты (верификация TTS, Pollinations) требуют сетевого доступа. Все тесты должны быть изолированы:
- По умолчанию: mock-серверы (без сети)
- Интеграционные тесты: `go test -tags=integration` (опционально, не в CI)
