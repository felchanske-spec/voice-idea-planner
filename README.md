# Voice Idea Planner Bot 🎙️🧠

A highly performant, asynchronous Telegram bot built with Go using Clean Architecture principles. The system intercepts text and voice IT project ideas, transcribes audio streams, executes concurrent AI analysis via Google Gemini API, and renders structured project roadmaps alongside Mermaid.js architecture diagrams.

## 🛠 Technology Stack

- **Backend Core:** Go (1.27) inside Clean Architecture modules (`domain`, `usecase`, `infrastructure`, `delivery`).
- **Transport Layer:** Asynchronous Telegram Long Polling (`go-telegram-bot-api/v5`) and an isolated HTTP diagnostics server.
- **Database & Migrations:** PostgreSQL 16 (`jackc/pgx/v5` connection pooling) with embedded automated schema migrations (`pressly/goose`).
- **AI & Audio Pipeline:** Google Gemini API (`v1beta`) secured via custom HTTP headers (`X-Goog-Api-Key`) for Speech-to-Text transcription and parallel content generation.
- **Diagram Rendering:** Automated REST integration with `mermaid.ink` utilizing URL-safe Base64 encoding.
- **Infrastructure & Quality:** Multi-stage Docker builds (~30MB Alpine runtime footprint), Docker Compose orchestration, and boundary Unit Testing (`testify/assert`, `httptest`).

## 🏗 Architecture Design

The system routes incoming user requests through a thread-safe rate limiter into a concurrent worker pipeline, persists structured records into PostgreSQL, and dynamically compiles visual architecture schemas.

![Architecture Flow](./assets/scheme.png)

## ⚡ Key Architectural Patterns Implemented

1. **Concurrent Async Worker Pipeline:** Leverages `sync.WaitGroup` and channel-bounded semaphores to execute 5 parallel AI requests simultaneously (Title, MVP Scope, Tech Stack, Risk Analysis, and Mermaid Diagram syntax), drastically reducing total API round-trip latency.
2. **Resilient Multi-Tier Markdown Engine:** Implements a graceful degradation parser (`ModeMarkdownV2` ➡ `ModeMarkdown` ➡ Plain Text) combined with header-aware text chunking (`##`) to prevent Telegram API entity truncation errors.
3. **Thread-Safe In-Memory Rate Limiter:** Guarded by `sync.Mutex` locks with isolated bucket cooldowns (60s for heavy AI generation, 10s for database telemetry lookups) to protect upstream API quotas.
4. **Cloud-Native Health Probes:** Dedicated HTTP server listening on port `:8080` exposing `/health/liveness` and `/health/readiness` endpoints with strict 2-second context timeouts for container orchestration.
5. **Deterministic Container Startup & Sanitization:** Enforces Docker Compose `service_healthy` database readiness checks alongside automated regex/string sanitizers that strip malformed LLM syntax before graph rendering.

## 🎬 End-to-End System Demonstration

### 🛠 Scenario 1: Docker Cluster Deployment & Bot Startup

Spinning up the PostgreSQL master store, executing embedded `goose` migrations, and initializing the Telegram polling gateway inside Docker containers:

![Docker & Bot Startup](./assets/deploy.gif)

### 📝 Scenario 2: Textual Idea Processing & Diagram Rendering

Submitting a raw project concept via text, triggering parallel AI section compilation, and receiving structured Markdown chunks followed by a rendered PNG architecture chart:

![Textual Idea Processing](./assets/text-message.gif)

### 🎙️ Scenario 3: Audio Voice Stream Transcription

Recording a voice message, downloading binary `.ogg` audio streams, converting speech to text via Gemini multimodal endpoints, and generating the full project roadmap:

![Audio Voice Stream Transcription](./assets/voice-message.gif)

## 🗺 Engineering & Development Process

Explore the implementation roadmap, repository commit lineage, and workspace layout built during the development lifecycle:

- **📋 Development Roadmap & Checklists:**

![Development Checklist](./assets/checklist.png)

- **🧬 Git Commit History & Branching Strategy:**

![Git Commit Tree](./assets/graph.png)

## 🚀 Local Deployment (Quick Start)

### 1. Environment Configuration

Create a `.env` file in the project root directory based on `.env.example` and provide your credentials:

```env
GEMINI_API_KEY=your_gemini_api_key_here
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here

POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_secure_password_here
POSTGRES_DB=voice_idea_planner

DATABASE_URL=postgres://postgres:your_secure_password_here@postgres:5432/voice_idea_planner?sslmode=disable
```

### 2. Build and Run Containers

Spin up the database cluster and the compiled Go bot in detached mode using Docker Compose:

```bash
docker compose up --build -d
```

### 3. Verify Logs and Health Probes

Check the automated database migrations and bot authorization logs, or query the readiness probe:

```bash
docker compose logs -f bot
curl http://localhost:8080/health/readiness
```

## 🧪 Testing Boundary Execution

Run the unit test suite covering domain usecases, rate limiters, and HTTP healthcheck handlers:

```bash
go test -v ./...
```
