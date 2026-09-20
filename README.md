# noteboard

Заметки + канбан-доска + голосовые заметки с автоматической расшифровкой.
Backend на Go (SQLite, без CGO), frontend на Nuxt 4 + Nuxt UI (SPA),
API описан в `.proto` и ходит через **ConnectRPC** (gRPC для браузера).

Собирается в **один бинарник** со вшитым фронтом — деплой это `scp` файла.
Либо `docker compose up -d`, и расшифровка работает сразу, без внешних ключей.

## Быстрый старт — Docker Compose

```sh
docker compose up -d      # app + локальный STT (speaches, CPU)
# http://localhost:8080
```

Три сервиса:

| Сервис | Что делает |
|---|---|
| `app` | сам noteboard, том `data` (SQLite, аудио, картинки) |
| `stt` | [speaches](https://github.com/speaches-ai/speaches), OpenAI-совместимый `/v1/audio/transcriptions` |
| `stt-init` | одноразово качает модель в том `models` — без этого speaches отвечает 404 |

Первый запуск качает образ speaches (~3 ГБ) и модель (~1,6 ГБ для large-v3-turbo),
дальше всё локально и офлайн. Порт меняется через `PORT`, модель — через `STT_MODEL`:

```sh
PORT=8081 STT_MODEL=Systran/faster-whisper-small docker compose up -d   # быстрее, но врёт
```

Имена собственные и термины модель коверкает — заменяет созвучной чепухой
(«транслигация» вместо «транскрибации»). Лечится подсказкой: compose ставит
общую фразу по умолчанию (она возвращает пунктуацию и заглавные), а свой
словарь дописывается сверху:

```sh
STT_PROMPT="Рабочая заметка. Дима Санечков, виртуализация, Sandbox, Nuxt, транскрибация" \
  docker compose up -d
```

Подсказка работает только на слова, которые в ней есть: без них модель снова
угадывает по звучанию.

Названия проектов и другие английские слова лучше отдавать не подсказкой, а
`STT_HOTWORDS` — отдельным полем faster-whisper. На фразе «проверить проекты
InfoKiosk, TaskComponent, Helmet, AppSettings, HomePage» turbo без hotwords
расслышал «Task-компонент», «аппсеттингс», «хомпейдж», с hotwords — все имена
верно. В compose уже лежит список по умолчанию, свой ставится через
`STT_HOTWORDS`.

Русские дообученные модели (`pav88/whisper-large-v3-russian-ct2`,
`bpnov/faster-whisper-large-v3-ru-podlodka` и другие из реестра speaches)
проверены на этих же записях и базовую `large-v3` не обошли: те же ошибки,
местами испорченная грамматика, в 3–4 раза медленнее. Смысла менять модель нет.
А вот `deepdml/faster-whisper-large-v3-turbo-ct2` смысл имеет и стоит на обоих
профилях: тот же энкодер, 4 слоя декодера вместо 32, на GPU 0,31–0,49 с против
0,70–0,99 с у large-v3 на тех же двух записях и вдвое меньше видеопамяти.
Качество с hotwords не хуже: turbo верно расслышал «на состав» и «для ведения»
там, где large-v3 выдал «на соство» и «для введения».
Если всё же пробовать свою — в репозитории на Hugging Face обязан лежать
`tokenizer.json`. Без него faster-whisper молча берёт токенизатор от
`whisper-tiny`, и вместо текста приходит `R. R. R. R.` или английский бред. Второй рычаг — сама запись: браузер по умолчанию гонит
микрофон через автоусиление и шумодав от голосовых звонков, они смазывают
окончания. Фронтенд их выключает (`useRecorder.ts`), Whisper на сыром звуке
точнее.

### GPU

С видеокартой NVIDIA распознавание идёт в разы быстрее: 19 с записи — **меньше
секунды** против ~10 с на CPU. Модель та же turbo, но в float16 и с
`WHISPER__TTL=-1` — висит в видеопамяти, а не грузится заново под каждую заметку.

Нужен NVIDIA Container Toolkit; проверить можно так:

```sh
docker info | grep -i runtimes            # должен быть nvidia
docker run --rm --gpus all ubuntu:24.04 nvidia-smi -L
```

Если runtime нет (Ubuntu/Debian):

```sh
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
  | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
echo 'deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://nvidia.github.io/libnvidia-container/stable/deb/amd64 /' \
  | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
sudo nvidia-ctk runtime configure --runtime=docker && sudo systemctl restart docker
```

Дальше:

```sh
docker compose -f compose.yaml -f compose.gpu.yaml up -d
```

Оверлей ставит `float16`, держит модель в видеопамяти (`WHISPER__TTL=-1`, иначе
первая заметка после паузы ждёт загрузку ~10 с) и подсовывает `stt-init` ту же
модель, что просит `app`. Первый запуск качает образ CUDA и модель (~1,6 ГБ).

## Быстрый старт — без Docker

```sh
make all        # bun install + nuxt generate + go build -> bin/noteboard
./bin/noteboard # http://localhost:8080
```

Данные лежат в `./data`: `noteboard.db` (SQLite), `audio/` (записи), `uploads/` (картинки
из редактора), `tmp/` (конвертация).

## Разработка

Два терминала:

```sh
make dev-api    # Go на :8080
make dev-web    # Nuxt на :3000, /api проксируется на :8080
make test       # go test ./...
make lint       # go vet + gofmt
```

Изменил `.proto` — перегенерируй клиент и сервер:

```sh
make tools      # один раз: buf, protoc-gen-go, protoc-gen-connect-go
make proto      # buf lint + buf generate
```

## Архитектура

Проект разложен по Clean Architecture: четыре слоя, зависимости **только внутрь**.
Внутренний слой никогда не импортирует внешний — поэтому `usecase` ничего не знает
ни про SQLite, ни про HTTP, и его можно тестировать без них.

```
        cmd/noteboard  ──>  internal/app        сборка: кто кого создаёт
                                  │
   ┌──────────────────────────────┼──────────────────────────────┐
   │ controller/rpc   controller/rest   repo/sqlite   repo/blob  │  адаптеры
   │      stt            webui            config                 │
   └──────────────────────────────┼──────────────────────────────┘
                                  ▼
                            internal/usecase      сценарии + интерфейсы
                                  ▼
                            internal/entity       типы и правила домена
```

| Слой | Пакет | Что внутри и чего там быть не должно |
|---|---|---|
| Entities | `internal/entity` | `Note`, `Card`, `Column`, статусы, `NotePatch`, `ErrNotFound`/`ErrInvalid`. Ни одного импорта проекта. |
| Use Cases | `internal/usecase` | Сценарии (`NotesUseCase`, `BoardUseCase`, воркер расшифровки) и **интерфейсы** к внешнему миру (`interfaces.go`). Ни `database/sql`, ни `net/http`. |
| Adapters | `internal/controller/*`, `internal/repo/*`, `internal/stt` | Перевод: JSON/proto ↔ сценарий, сценарий ↔ SQL, сценарий ↔ Whisper. |
| Infrastructure | `internal/app`, `internal/config`, `internal/webui`, `cmd` | Чтение окружения, создание всех объектов, graceful shutdown. |

Три правила, которые тут держат структуру:

1. **Интерфейс объявляет потребитель.** `usecase.NoteRepo` лежит в `usecase`, а не
   в `repo/sqlite`. Поэтому репозиторий можно заменить, не трогая сценарии, и
   стрелка зависимости смотрит внутрь.
2. **Ошибки домена — в домене.** Репозиторий возвращает `entity.ErrNotFound`, и
   ровно два места (`rest.writeError`, `rpc.errorOf`) знают, что это 404 и
   `CodeNotFound`. Добавился новый вид ошибки — правятся эти два switch.
3. **`os.Getenv` встречается только в `internal/config`.** Иначе настройки
   расползаются по коду и «откуда взялся этот таймаут» становится квестом.

### Где «правильная» многослойность была бы лишней

Проект — 2400 строк Go, и часть энтерпрайз-практик здесь сознательно не сделана:

- **DTO на каждый слой.** JSON-теги живут прямо на `entity.Note`. Отдельные
  `NoteDTO` для транспорта дали бы +300 строк перекладывания полей и ноль пользы;
  когда формат API разойдётся с БД — тогда и появятся. Для RPC перекладывание
  всё-таки есть (`rpc.notePB`), но не потому что «так правильно», а потому что
  типы генерирует protoc.
- **DI-контейнер** (wire, fx). Вся сборка — 30 строк в `internal/app/app.go`,
  читается сверху вниз. Контейнер заменил бы явный код на магию.
- **Repository + UnitOfWork + Specification.** Транзакция нужна ровно в одном
  месте (`Reorder`), и она написана руками на `sql.Tx`.
- **Отдельный `pkg/`.** Наружу мы ничего не публикуем, значит всё `internal/`.

Правило простое: абстракцию вводим, когда у неё появился **второй потребитель**.

### Почему ConnectRPC, а не чистый gRPC

Из браузера чистый gRPC не работает: нужен доступ к HTTP/2-фреймам, которого у
`fetch` нет. Стандартный обход — gRPC-Web плюс прокси (Envoy). [Connect](https://connectrpc.com)
решает это без прокси: тот же `.proto`, те же пути `/noteboard.v1.NotesService/Method`,
но поверх обычного HTTP/1.1, а тело — protobuf или JSON на выбор.

Из этого следует практичное:

- RPC можно дёрнуть простым `curl` (см. раздел API) — не нужен grpcurl;
- сервер завёрнут в `h2c`, поэтому на тот же порт придут и обычные gRPC-клиенты;
- **загрузка файлов осталась на REST**: браузерный Connect не умеет стримить тело
  запроса, а гнать мегабайты аудио base64-ом внутри JSON — плохая идея.

Итог: два транспорта на одном порту, один набор сценариев под ними.
`internal/app/app.go` монтирует их в `http.ServeMux` — это и есть демонстрация
того, ради чего слои: добавление второго API не потребовало правок в `usecase`.

## Как связаны заметки и доска

Карточка может ссылаться на заметку (`cards.note_id`). Тогда она показывает живой
заголовок заметки и статус расшифровки, а клик ведёт на саму заметку. Два пути:

- на доске — кнопка с иконкой блокнота в подвале колонки, поиск по заметкам;
- на странице заметки — «На доску» с выбором колонки.

Обычные карточки без заметки тоже остаются.

## Расшифровка голоса

Через Compose уже настроена. Вручную движок выбирается переменными окружения
(см. `.env.example`); без них запись сохраняется, но статус будет `error` с подсказкой.

**Вариант 1 — HTTP API, OpenAI-совместимый** (OpenAI, speaches, Groq, `whisper.cpp server`):

```sh
TRANSCRIBE_API_URL=https://api.openai.com/v1/audio/transcriptions
TRANSCRIBE_API_KEY=sk-...
TRANSCRIBE_MODEL=whisper-1
```

**Вариант 2 — локальный whisper.cpp:**

```sh
WHISPER_BIN=/opt/whisper.cpp/build/bin/whisper-cli
WHISPER_MODEL=/opt/whisper.cpp/models/ggml-large-v3-turbo.bin
```

Браузер пишет webm/opus без длительности и Cues, и не всякий сервер такое декодирует,
поэтому перед отправкой файл конвертируется в 16 кГц mono WAV через `ffmpeg`
(в образе он есть; если `ffmpeg` нет — отправляется оригинал).

Расшифровка идёт в фоне: заметка сохраняется сразу, текст подтягивается позже.
Очередь живёт в самой таблице `notes` (`transcribe_status`), после рестарта
зависшие `running` возвращаются в `pending`.

## Переменные окружения

| Переменная | По умолчанию | Смысл |
|---|---|---|
| `ADDR` | `:8080` | адрес HTTP-сервера |
| `DATA_DIR` | `./data` | база, аудио, временные файлы |
| `WEB_DIR` | — | отдавать фронт из папки вместо вшитого |
| `TRANSCRIBE_API_URL` | — | OpenAI-совместимый endpoint |
| `TRANSCRIBE_API_KEY` | — | ключ, если нужен |
| `TRANSCRIBE_MODEL` | `whisper-1` | имя модели для API |
| `WHISPER_BIN` / `WHISPER_MODEL` | — | локальный whisper.cpp |
| `TRANSCRIBE_LANG` | `ru` | язык распознавания |
| `TRANSCRIBE_PROMPT` | — | подсказка модели: имена, термины, жаргон (compose подставляет общую фразу) |
| `TRANSCRIBE_HOTWORDS` | — | те же слова, но точнее; понимает faster-whisper, не OpenAI |
| `TRANSCRIBE_VAD` | — | `true` — резать тишину, меньше галлюцинаций на паузах |
| `FFMPEG_BIN` | `ffmpeg` | путь к ffmpeg |
| `PORT` | `8080` | только для compose: внешний порт |
| `STT_MODEL` | `deepdml/faster-whisper-large-v3-turbo-ct2` | только для compose: модель speaches |
| `STT_PROMPT` | общая фраза о заметке | только для compose: значение `TRANSCRIBE_PROMPT` |
| `STT_HOTWORDS` | список имён проектов | только для compose: значение `TRANSCRIBE_HOTWORDS` |

## API

Контракт: [`api/proto/noteboard/v1/noteboard.proto`](api/proto/noteboard/v1/noteboard.proto).
Все вызовы — `POST /api/rpc/<пакет>.<Сервис>/<Метод>`, тело JSON или protobuf.

```
noteboard.v1.NotesService
  ListNotes      {query}              -> {notes:[...]}      поиск по title/body/transcript
  GetNote        {id}                 -> {note}
  CreateNote     {title, body}        -> {note}
  UpdateNote     {id, title?, body?, transcript?, pinned?} -> {note}
  DeleteNote     {id}                 -> {}
  Retranscribe   {id}                 -> {note}

noteboard.v1.BoardService
  GetBoard       {}                   -> {columns:[{cards:[...]}]}
  CreateColumn   {title}              -> {column}
  RenameColumn   {id, title}          -> {}
  DeleteColumn   {id}                 -> {}
  CreateCard     {columnId, title?, body?, noteId?} -> {card}
  AttachNote     {noteId, columnId}   -> {card}      columnId=0 — первая колонка
  UpdateCard     {id, title?, body?, noteId?}       -> {}
  DeleteCard     {id}                 -> {}
  Reorder        {columns:[{columnId, cardIds:[...]}]} -> {}   одной транзакцией
```

Проверить руками:

```sh
curl -X POST localhost:8080/api/rpc/noteboard.v1.NotesService/CreateNote \
  -H 'content-type: application/json' -d '{"title":"тест","body":"<p>тело</p>"}'
```

Поля `optional` в proto — это presence: не переданное поле сервер не трогает.
`int64` в JSON приезжает строкой (правило JSON-маппинга protobuf), а в TypeScript
становится `bigint` — отсюда `BigInt(...)`/`String(id)` во фронте.

Файлы и служебное остались на REST:

```
GET    /api/health                   {"ok":true,"transcribe":"api|whisper.cpp|off"}
POST   /api/notes/{id}/audio         multipart, поле audio
GET    /api/notes/{id}/audio
POST   /api/uploads                  multipart, поле file — картинка из редактора
GET    /api/uploads/{name}           отдача картинки, immutable-кэш
```

## Структура проекта

```
.
├── cmd/noteboard/main.go       точка входа: прочитать конфиг, запустить app
├── api/proto/noteboard/v1/     контракт API (.proto) — источник правды
├── buf.yaml / buf.gen.yaml     кодогенерация: Go-сервер и TS-клиент из одного файла
├── Makefile                    all / web / server / proto / test / lint / clean
├── Dockerfile                  3 стадии: bun (фронт) -> golang (бинарник) -> alpine
├── compose.yaml                app + stt + stt-init, якоря, лимиты, read-only rootfs
├── compose.gpu.yaml            оверлей: speaches на CUDA, модель крупнее
│
├── internal/
│   ├── entity/                 ДОМЕН: Note, Card, Column, статусы, ошибки
│   │   ├── note.go             Note + NotePatch (частичное обновление)
│   │   ├── board.go            Column, Card, CardPatch, шаг позиций
│   │   └── errors.go           ErrNotFound, ErrInvalid
│   │
│   ├── usecase/                СЦЕНАРИИ: вся логика приложения
│   │   ├── interfaces.go       что сценариям нужно снаружи (репозитории, файлы, STT)
│   │   ├── notes.go            заметки: файлы на диске, очередь, чистка при удалении
│   │   ├── board.go            доска: позиции, карточка из заметки
│   │   ├── title.go            правило: как из заметки получается подпись карточки
│   │   └── transcribe.go       фоновый воркер очереди расшифровки
│   │
│   ├── controller/             АДАПТЕРЫ ВХОДА
│   │   ├── rpc/                ConnectRPC: notes.go, board.go, маппинг entity <-> proto
│   │   └── rest/               REST: аудио, картинки, health, статика фронта
│   │
│   ├── repo/                   АДАПТЕРЫ ВЫХОДА
│   │   ├── sqlite/             DB, версионированные миграции (embed), notes/board/queue
│   │   └── blob/               файлы на диске: имя, сохранение, удаление
│   │
│   ├── stt/                    АДАПТЕРЫ РАСПОЗНАВАНИЯ
│   │   ├── engine.go           интерфейс Engine + выбор движка по конфигу
│   │   ├── openai.go           OpenAI-совместимый HTTP (speaches, OpenAI, Groq)
│   │   ├── whispercpp.go       локальный бинарник whisper.cpp
│   │   └── ffmpeg.go           webm -> 16 кГц mono WAV
│   │
│   ├── app/app.go              СБОРКА: кто кого создаёт, два транспорта, shutdown
│   ├── config/config.go        единственное место с os.Getenv + валидация
│   ├── gen/                    сгенерированный Go из .proto (руками не править)
│   └── webui/                  embed.FS + SPA-fallback + Cache-Control
│
├── web/                        Nuxt 4, ssr: false, генерится статикой
│   ├── nuxt.config.ts          @nuxt/ui, devProxy /api -> :8080
│   └── app/
│       ├── app.vue             UDashboardGroup + сайдбар: навигация, поиск, список
│       ├── types.ts            ре-экспорт типов из кодогенерации
│       ├── gen/                сгенерированный TypeScript из .proto
│       ├── composables/
│       │   ├── useRpc.ts       Connect-транспорт и клиенты notesRpc/boardRpc
│       │   ├── useApi.ts       $fetch для REST-эндпоинтов + formatDate, stripHtml
│       │   ├── useNotes.ts     общее состояние заметок (useState), CRUD, statusMeta
│       │   └── useRecorder.ts  MediaRecorder: start/stop/cancel, таймер, mmss
│       ├── components/
│       │   ├── BoardColumn.vue колонка: drag&drop, карточки, ссылка на заметку
│       │   └── NoteEditor.vue  TipTap: форматирование, картинки (вставка, drag&drop)
│       └── pages/
│           ├── index.vue       новая заметка + запись голоса
│           ├── board.vue       доска: адаптивная сетка, модалка «заметку на доску»
│           └── notes/[id].vue  заметка: правка, аудио, расшифровка, «На доску»
│
└── data/                       (создаётся при запуске) noteboard.db, audio/, uploads/, tmp/
```

## Тесты

```sh
make test
```

Три уровня, по одному примеру каждого:

- `internal/usecase/title_test.go` — чистая функция, никаких зависимостей;
- `internal/repo/sqlite/notes_test.go` — настоящая база в `t.TempDir()`, те же миграции;
- `internal/controller/rpc/rpc_test.go` — весь стек на `httptest` и сгенерированный
  клиент; проверяет в том числе, что `ErrNotFound` приезжает как `CodeNotFound`.

Моков нет намеренно: SQLite файловый и быстрый, интеграционный тест честнее.

## Ограничения

- Аутентификации нет — рассчитано на локальный запуск или порт за reverse proxy.
- `MediaRecorder` работает только на `localhost` или по HTTPS.
- Один процесс, одно соединение к SQLite на запись — для личного использования хватает.
- Поиск регистрозависимый для кириллицы: `LIKE` в SQLite складывает регистры
  только для ASCII. Лечится FTS5, но это уже CGO или отдельная таблица.
- По умолчанию large-v3-turbo на CPU: ~10 с на 30 с записи. Нужно быстрее — `small`,
  но качество заметно хуже. Нужно и быстро, и точно — GPU-профиль (меньше секунды).
