# e2e-tr

A self-hosted web portal that lets QA engineers and non-technical team members **record and run Playwright E2E tests from a browser** — no CLI required.

> **Status:** Demo / mock version. AWS infrastructure (Terraform) is planned for a future release.

---

## What you can do

### Record a scenario
1. Open the portal and enter a URL
2. Chromium launches with Playwright's codegen
3. Click through your app — actions are recorded automatically
4. Close the browser → the test is saved as a `.spec.ts` file

### Run tests
- **By tag** — run all tests tagged `@search`, `@cart`, etc.
- **By scenario** — pick any recorded `.spec.ts` file from the list
- Live log output streams in real-time via SSE
- When done, jump directly to the Playwright HTML report (screenshots, video, trace)

---

## Architecture

```
[Portal — Next.js :3000]
        │
        │  HTTP + SSE
        ▼
[Runner — Go :8080]
        │
        │  exec
        ▼
[Playwright (Chromium)]
        │
        ├── tests/tests/*.spec.ts   ← recorded scenarios
        └── playwright-report/      ← HTML report served at /report/
```

---

## Tech stack

| Layer | Technology |
|-------|------------|
| Portal UI | Next.js 14 (App Router) + TypeScript |
| API server | Go (net/http, SSE streaming) |
| Test execution | Playwright + TypeScript |
| Realtime logs | Server-Sent Events (SSE) |
| Container | Docker / docker-compose |
| Infrastructure | Terraform (planned) |

---

## Getting started

### Prerequisites

- Go 1.21+
- Node.js 20+
- Playwright installed in `tests/`

### 1. Set up tests

```bash
cd tests
npm install
npx playwright install chromium
```

### 2. Start the runner

```bash
cd runner
go run .
# → http://localhost:8080
```

### 3. Start the portal

```bash
cd portal
npm install
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev
# → http://localhost:3000
```

Open `http://localhost:3000` in your browser.

---

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/tags` | List test tags scanned from `.spec.ts` files |
| `POST` | `/api/run` | Run tests by tag or file |
| `GET` | `/api/stream?id=` | SSE stream of run logs |
| `POST` | `/api/codegen/start` | Start a Playwright codegen session |
| `GET` | `/api/codegen/stream?id=` | SSE stream of codegen status |
| `GET` | `/api/scenarios` | List saved scenario files |
| `DELETE` | `/api/scenarios?name=` | Delete a scenario file |
| `GET` | `/report/` | Playwright HTML report |

---

## Known limitations

- **Local display required for codegen.** `playwright codegen` opens a browser on the machine running the runner. Remote/cloud deployments need a VNC or noVNC setup to stream the display — not yet supported.
- **In-memory run history.** Completed runs are stored in memory only and lost on restart. Persistent storage is planned.
- **No authentication.** The portal has no login. Do not expose it to the public internet as-is.
- **Single runner.** Tests run sequentially on one machine. Parallel execution across ECS tasks is a planned feature.

---

## Roadmap

- [ ] AWS infrastructure via Terraform (App Runner + ECS Fargate + S3)
- [ ] Persistent run history
- [ ] Parallel test execution by tag
- [ ] Authentication

---

## License

MIT
