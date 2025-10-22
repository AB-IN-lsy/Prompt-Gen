# **Repository Guidelines**

This repository combines an Electron shell, a Go HTTP backend, and a Vite/React frontend. Use this guide to stay aligned with the existing workflows.

---

## Project Structure & Module Organization

* `main.js` boots Electron and points the renderer to the compiled frontend while delegating API traffic to the Go service on port 9090.
* `backend/cmd/server` contains the production entrypoint; shared domain, service, and infra logic live under `backend/internal`, with static uploads in `backend/public`.
* Automated coverage is split by intent: `backend/tests/unit`, `backend/tests/integration` (requires tagged build), and `backend/tests/e2e`.
* `frontend/src` hosts React components, state stores, and Tailwind styles; helper scripts reside in `frontend/scripts`, and production bundles land in `frontend/dist`.
* Design references and longer-form documentation are tracked in `design/` and `docs/`, respectively.
* Environment Variable Loading: The root .env / .env.local files are read by the Electron main process, Go backend, and Vite frontend simultaneously; new fields should be added to .env.example as well.

---

## Build, Test, and Development Commands

* `npm install && npm --prefix frontend install`: install Electron shell and frontend dependencies.
* `go run ./backend/cmd/server`: start the API with environment variables like `JWT_SECRET`, `SERVER_PORT`, and database credentials preset.
* `npm run dev:frontend`: hot-reload the React app on Vite; point Electron to the Vite URL during UI work.
* `npm start`: launch the packaged Electron client (expects the backend available on `http://localhost:9090`).
* `npm run build:frontend`: produce production assets consumed by `main.js`.
* `go test ./...` (unit), `go test -tags=integration ./backend/tests/integration`, and `go test -tags=e2e ./backend/tests/e2e` for broader suites; capture logs in the per-suite `logs/` folders.

---

## Coding Style & Naming Conventions

* Go code must stay `gofmt`-clean with idiomatic CamelCase exports and lower_snake package names; keep handlers thin and push logic into `internal/service`.
* New TypeScript files follow the existing PascalCase component pattern under `frontend/src/**`; prefer hooks over classes and keep Tailwind utility sets in `index.css`.
* Run `npm --prefix frontend run lint` before submitting UI work; it enforces TS strictness and the “no implicit any / no-js” policy.
* Use structured logging via `logger` helpers in the handler layer for errors, rather than in the service layer, which does not handle logging directly.
* **New Requirements:**

  1. **.env Files:** Never overwrite the `.env` file; if you need to add new environment variables, add them to the `.env.example` file instead.
  2. **Function Documentation:** When adding backend functions, always write Chinese comments above the function explaining its purpose.
  3. **Avoid Magic Numbers:** Avoid using magic numbers (hardcoded numbers in the code); instead, always load such values from environment variables.
  4. **Reading Project Info:** You can find project information and upcoming product requirements in both the backend and frontend README files.
  5. **Update Documentation:** When modifying both frontend and backend code, remember to update both the frontend and backend README files, especially with API details and project architecture changes.
  6. **i18n for Copy**: If frontend needs to add new copy or text, it should be loaded from the i18n system. The default language for text should be Chinese.
  7. **Post-Answer Documentation**: After answering a question, always write a brief summary of what you’ve done in Chinese. If you ask me to write a git commit, it should be written in Chinese and ensure there are no quotation marks in the commit message.

---

## Code Modification Visibility

* When adding or modifying code, **do not use hidden or non-transparent patch commands** (for example, `apply_patch <<'PATCH'` or similar automated patch scripts).
* All code edits must be **visibly shown in the terminal or pull request**, with clear additions and deletions.
* Acceptable examples include visible diffs such as `git diff` or PR views that highlight line changes with `+` and `-`.
* Avoid summarized patch outputs that only display generic messages like “Updated file X (+58 lines)” without showing the actual line differences.
* This rule applies to both frontend and backend changes and ensures that all modifications are **fully visible and reviewable**.
* In short: always prefer **explicit visible diffs** over **hidden patch logs**, so that every code change can be directly inspected.

---

## Testing Guidelines

* Place new fast checks beside the subject in `backend/tests/unit`; mimic existing table-driven tests.
* Integration tests rely on sqlite or configurable services—document required env vars in the test file docstrings and gate them behind `//go:build integration`.
* E2E suites hit live infra; skip them unless credentials for MySQL, Redis, and Nacos are available, and never hardcode secrets.
* Frontend changes should include story-driven manual steps in the PR when automated coverage is not available.

---

## Commit & Pull Request Guidelines

* Follow the conventional commit style seen in history (`feat: ...`, `fix: ...`, `chore: ...`); keep subjects under ~72 characters and prefer imperative verbs.
* Each PR should link the relevant issue, summarize backend vs. frontend impact, list the commands/tests you ran, and attach UI screenshots or API samples when behavior changes.
* Ensure `.env` and other secrets stay untracked; share configs through `.env.example` snippets or docs updates instead.

---

## Security & Configuration Tips

* Sensitive runtime knobs are environment-driven (`MODEL_CREDENTIAL_MASTER_KEY`, `CAPTCHA_*`, Redis, and SMTP credentials); validate them locally with `.env` files but do not commit them.
* The backend writes structured logs to `backend/logs`; rotate or truncate before pushing branches to avoid noisy diffs.
* When touching auth or credential flows, double-check AES key sizes and JWT TTL defaults in `internal/bootstrap` to avoid breaking production deployments.

---
