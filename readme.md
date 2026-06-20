[![日本語](https://img.shields.io/badge/README-日本語-red?style=flat-square&logo=googletranslate)](readme.ja.md)

# e2e-tr

A self-hosted web portal that lets QA engineers and non-technical team members **record and run Playwright E2E tests from a browser** — no CLI required.

> **Status:** Demo / mock version. AWS infrastructure (Terraform) is planned for a future release.

---

## Demo

### Record a scenario

> [!NOTE]
> **GIF coming soon** — replace with `docs/assets/demo-record.gif`
> Capture: enter URL → click "Start recording" → Chromium appears in noVNC iframe → interact → `.spec.ts` saved

### Run tests

> [!NOTE]
> **GIF coming soon** — replace with `docs/assets/demo-run.gif`
> Capture: select scenario → run → SSE log streams → jump to HTML report

---

## What you can do

### Record a scenario
1. Open the portal and enter a URL
2. Chromium launches with Playwright's codegen
3. Click through your app — actions are recorded automatically
4. When `USE_NOVNC=true`, the live browser session is embedded in the portal via a noVNC iframe
5. Close the browser → the test is saved as a `.spec.ts` file

### Run tests
- **By tag** — run all tests tagged `@search`, `@cart`, etc.
- **By scenario** — pick any recorded `.spec.ts` file from the list
- Live log output streams in real-time via SSE
- When done, jump directly to the Playwright HTML report (screenshots, video, trace)

---

## Architecture

```mermaid
flowchart TD
    Browser["Browser"]
    Portal["Portal — Next.js :3000"]
    Runner["Runner — Go :8080"]
    PW["Playwright (Chromium)"]
    Files["tests/tests/*.spec.ts"]
    Report["playwright-report/"]

    Browser -->|"HTTP + SSE"| Portal
    Portal -->|"HTTP + SSE"| Runner
    Runner -->|"exec"| PW
    PW --> Files
    PW --> Report
    Report -->|"GET /report/"| Browser
```

**Why KasmVNC?** When running inside Docker, Chromium has no physical display — you can't see or interact with the codegen browser from your machine. KasmVNC solves this: its `Xvnc` server bundles the X server, VNC, and a WebSocket-capable web server into a single process, streaming the virtual display over WebSocket and embedding it as an iframe in the portal. This is the primary use case: **Docker Compose with `USE_NOVNC=true` is the fully supported path.** → [browser preview architecture](docs/novnc-architecture.md)

---

## Tech stack

| Layer | Technology |
|-------|------------|
| Portal UI | Next.js 16 (App Router) + TypeScript |
| API server | Go (net/http, SSE streaming) |
| Test execution | Playwright + TypeScript |
| Realtime logs | Server-Sent Events (SSE) |
| Browser preview | KasmVNC (Xvnc — X server + VNC + Web in one process) |
| Container | Docker / docker-compose |
| Infrastructure | Terraform (planned) |

---

## Getting started

### Option A — Docker Compose (recommended)

All you need is Docker.

```bash
make up
# Portal  → http://localhost:3000
# Runner  → http://localhost:8080
# noVNC   → http://localhost:6080–6089 (one port per codegen session)
```

`docker-compose.yml` sets `USE_NOVNC=true` and `NEXT_PUBLIC_NOVNC_HOST` automatically.

### Option B — Manual setup

Prerequisites: Go 1.25+ and Node.js 20+.

**1. Set up tests**

```bash
cd tests
npm install
npx playwright install chromium
```

**2. Start the runner**

```bash
cd runner
go run .
# → http://localhost:8080
```

**3. Start the portal**

```bash
cd portal
npm install
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev
# → http://localhost:3000
```

Open `http://localhost:3000` in your browser.

---

## Environment variables

| Variable | Service | Default | Description |
|----------|---------|---------|-------------|
| `TESTS_DIR` | runner | `../tests` | Path to the tests directory |
| `PORT` | runner | `127.0.0.1:8080` | HTTP listen address. Defaults to localhost-only; set `0.0.0.0:8080` to expose (auth required, see Known limitations) |
| `ALLOWED_ORIGINS` | runner | (localhost only) | Comma-separated extra origins allowed by `SameOriginGuard`; `localhost`/`127.0.0.1` (any port) are always allowed |
| `DB_PATH` | runner | `<TESTS_DIR>/.runs.db` | SQLite path for run history. Defaults under `TESTS_DIR` so it persists on the mounted volume; set explicitly to override |
| `USE_NOVNC` | runner | `true` | Enable KasmVNC (Xvnc) for codegen browser preview (up to 10 concurrent sessions) |
| `RUN_TIMEOUT_MINUTES` | runner | `30` | Maximum minutes a single test run is allowed to execute before it is forcibly killed |
| `MAX_CONCURRENT_RUNS` | runner | `4` | Maximum number of test runs that may execute in parallel; excess requests receive HTTP 429 |
| `NEXT_PUBLIC_API_URL` | portal | `http://localhost:8080` | Runner API base URL |
| `NEXT_PUBLIC_NOVNC_HOST` | portal | `http://localhost` | Hostname used to build noVNC iframe URLs |

---

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/tags` | List test tags scanned from `.spec.ts` files |
| `POST` | `/api/run` | Run tests by tag or file |
| `GET` | `/api/runs` | List run history (persisted in SQLite), newest first |
| `GET` | `/api/stream?id=` | SSE stream of run logs |
| `POST` | `/api/codegen/start` | Start a Playwright codegen session; returns `noVNCPort` when `USE_NOVNC=true` |
| `GET` | `/api/codegen/stream?id=` | SSE stream of codegen status |
| `GET` | `/api/codegen/code?id=` | Get the live source of the recording session's `.spec.ts` |
| `GET` | `/api/scenarios` | List saved scenario files |
| `PATCH` | `/api/scenarios?name=&to=` | Rename a scenario file (migrates tag assignments) |
| `DELETE` | `/api/scenarios?name=` | Delete a scenario file |
| `GET` | `/api/scenarios/code?name=` | Get the source of a saved scenario file |
| `PUT` | `/api/scenarios/code?name=` | Overwrite the source of an existing scenario file |
| `PUT` | `/api/scenarios/tags` | Replace the tag assignments of a scenario |
| `GET` | `/api/environments` | List execution environments (passwords masked) |
| `POST` | `/api/environments` | Create an execution environment |
| `PATCH` | `/api/environments?id=` | Update an execution environment |
| `DELETE` | `/api/environments?id=` | Delete an execution environment |
| `GET` | `/report/` | Playwright HTML report |

---

## Testing

Test strategy and pyramid for this repository → [Testing strategy](docs/testing-strategy.md)

### Run tests

```bash
just ci                   # CI-equivalent local run (runner build/vet/test + portal build + compose E2E)
make test-runner          # Go unit tests only
make test-portal          # Portal E2E tests (requires: make up)
make test-e2e             # User scenario tests (requires: make up)
make test                 # Run all (runner unit + portal E2E, starts/stops the stack automatically)
```

BDD specifications: [`spec/runner.md`](spec/runner.md) | [`spec/portal-ui.md`](spec/portal-ui.md)

---

## Known limitations

- **Run history is persisted in SQLite.** Completed runs survive restarts (stored at `DB_PATH`, default under `TESTS_DIR`). Active (running) runs live in memory and are lost if the process dies mid-run; on restart any run left in `running` state is marked `failed`.
- **No authentication.** The runner has no login and can execute arbitrary specs, so an exposed instance is effectively a remote code execution (RCE) endpoint. To stay safe by default, the runner binds to `127.0.0.1` (localhost only) and a `SameOriginGuard` rejects cross-origin browser requests. **If you expose it publicly**, widen the bind via `PORT` (e.g. `0.0.0.0:8080`) and allow your front-end origins via `ALLOWED_ORIGINS` — and you **must** put authentication in front of it (e.g. a reverse proxy / the Web edition's auth middleware). Do not expose it as-is.
- **Configurable concurrency, single machine.** `MAX_CONCURRENT_RUNS` (default `4`) controls how many tests run in parallel, but all runs share one machine. Parallel execution across ECS tasks is a planned feature.

---

## Roadmap

- [ ] AWS infrastructure via Terraform (App Runner + ECS Fargate + S3)
- [x] Persistent run history
- [ ] Parallel test execution by tag
- [ ] Authentication

---

## License

MIT
