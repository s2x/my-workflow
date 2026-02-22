# Decodo Workflow - Agentic System Plan

## Wizja

System multi-agentowy do automatyzacji pracy z ticketami Jira. Użytkownik pisze na czat, **Go server** pobiera dane z Jira/GitLab, buduje prompt startowy z pełnym kontekstem i uruchamia sesję **opencode** dla odpowiedniego agenta. Agenci nie mają dostępu do kluczy API - dostają gotowe dane w prompcie.

---

## Architektura

### Zasada: "Server posiada klucze, agent dostaje kontekst"

- **Go server** - jedyny komponent z dostępem do Jira/GitLab/Slack API
- **Go server** buduje prompt z danymi i uruchamia `opencode run --agent <name> "<prompt>"`
- **Agenci opencode** - pracują tylko na kodzie i plikach, nie wiedzą nic o API
- **Custom tools** - minimalne: tylko `task-result.ts` do zwrócenia strukturalnego wyniku

### Diagram

```
                         ┌──────────────┐
                         │   Web UI     │
                         │  (Next.js)   │
                         └──────┬───────┘
                                │ HTTP/SSE
                                ▼
┌───────────────────────────────────────────────────────┐
│                    Go Server                           │
│                                                        │
│  ┌─────────┐  ┌─────────┐  ┌──────────┐  ┌────────┐ │
│  │ Jira    │  │ GitLab  │  │ Workflow  │  │  Chat  │ │
│  │ Client  │  │ Client  │  │ Engine    │  │  API   │ │
│  └─────────┘  └─────────┘  └──────────┘  └────────┘ │
│       │              │            │             │     │
│       └──────┬───────┘            │             │     │
│              ▼                    ▼             │     │
│     Buduje prompt           SQLite DB ◄─────────┘     │
│     z kontekstem         (tasks, messages,             │
│              │            workflows)                   │
│              ▼                                         │
│     exec: opencode run                                 │
│       --agent descriptor                               │
│       --format json                                    │
│       "<prompt z danymi z Jira>"                       │
│              │                                         │
│              ▼                                         │
│     Parsuje wynik (JSON)                               │
│     Zapisuje do DB                                     │
│     Decyduje o następnym kroku                         │
└───────────────────────────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    ▼                       ▼
            ┌──────────┐           ┌──────────────┐
            │ opencode │           │ Target Repo  │
            │ (agent   │──────────│ (kod na       │
            │  session)│  r/w     │  którym agent │
            └──────────┘          │  pracuje)     │
                                  └──────────────┘
```

### Flow szczegółowy

```
[1] User pisze: "zajmij się ticketem ABC-123"
         │
         ▼
[2] Go Server:
    - Sprawdza czy workflow dla ABC-123 już istnieje
    - Pobiera ticket z Jira API (summary, description, AC, labels)
    - Zapisuje workflow do DB (status: RUNNING)
    - Wysyła na czat: "Analizuję ticket ABC-123..."
    - Buduje prompt:
      """
      Przeanalizuj poniższy ticket Jira i stwórz specyfikację techniczną.

      ## Ticket: ABC-123
      **Summary**: Dodać endpoint do usuwania użytkownika
      **Description**: Jako admin chcę móc usunąć konto...
      **Acceptance Criteria**:
      - Endpoint DELETE /api/users/:id
      - Soft delete (flaga deleted_at)
      - Tylko admin może usuwać
      ...

      Odpowiedz w formacie JSON:
      { "code_changes": [...], "test_spec": [...] }
      """
    - Exec: opencode run --agent descriptor --format json "<prompt>"
         │
         ▼
[3] Descriptor (sesja opencode):
    - Czyta strukturę repozytorium (wbudowane narzędzia opencode)
    - Analizuje istniejący kod
    - Zwraca JSON z planem zmian i spec testów
         │
         ▼
[4] Go Server:
    - Parsuje JSON output od Descriptora
    - Zapisuje spec do DB (task output)
    - Buduje prompt dla Codera:
      """
      Zaimplementuj poniższe zmiany w kodzie.

      ## Specyfikacja
      <spec od Descriptora>

      ## Wytyczne
      - Utwórz branch: feature/ABC-123
      - Commituj zmiany
      - Napisz unit testy wg specyfikacji
      """
    - Exec: opencode run --agent coder "<prompt>"
         │
         ▼
[5] Coder (sesja opencode):
    - git checkout -b feature/ABC-123
    - Implementuje zmiany (read/write/edit/bash)
    - Pisze testy
    - git add + commit + push
    - Zwraca podsumowanie
         │
         ▼
[6] Go Server:
    - Parsuje wynik
    - Buduje prompt dla Testera:
      """
      Przetestuj zmiany na branchu feature/ABC-123.

      ## Co zostało zmienione
      <diff z git>

      ## Specyfikacja testów
      <spec od Descriptora>

      Uruchom testy, sprawdź pokrycie, napisz dodatkowe jeśli potrzeba.
      """
    - Exec: opencode run --agent tester "<prompt>"
         │
         ▼
[7] Tester -> Go Server -> (FAIL? retry Coder) -> Reviewer -> User approval -> Deployer
```

---

## Agenci (opencode agents)

| Agent | Uprawnienia | Co dostaje w prompcie | Co zwraca |
|-------|-------------|----------------------|-----------|
| **descriptor** | read-only (pliki repo) | Pełny ticket z Jira, struktura repo | JSON: plan zmian + spec testów |
| **coder** | read, write, edit, bash | Spec od Descriptora, wytyczne | Podsumowanie zmian, branch name |
| **tester** | read, write, bash | Git diff, spec testów | PASS/FAIL + raport, dodatkowe testy |
| **reviewer** | read-only | Git diff, zmienione pliki | APPROVED/CHANGES_REQUESTED + uwagi |
| **deployer** | bash (git only) | Branch name, target branch | DEPLOYED/FAILED |

**Brak orchestratora jako agenta** - Go server pełni tę rolę. Jest szybszy, deterministyczny, i nie marnuje tokenów na decyzje routingowe.

---

## Go Server - komponenty

### Workflow Engine
Deterministyczna maszyna stanów w Go:

```
CREATED -> DESCRIBING -> CODING -> TESTING -> REVIEWING -> AWAITING_APPROVAL -> DEPLOYING -> DONE
                           ▲          │            │
                           │          ▼            ▼
                           └──── FIX_NEEDED ◄──────┘
                                 (max 3 retry)
```

### Jira Client
- `GET /rest/api/3/issue/{key}` - pobierz ticket
- `POST /rest/api/3/issue/{key}/comment` - dodaj komentarz o statusie
- `PUT /rest/api/3/issue/{key}/transitions` - zmień status ticketa

### GitLab Client
- Pobieranie diff z MR
- Tworzenie MR po deploy na stage
- Sprawdzanie statusu CI pipeline

### Agent Runner
```go
func (r *AgentRunner) Run(agent string, prompt string) (string, error) {
    cmd := exec.Command("opencode", "run",
        "--agent", agent,
        "--format", "json",
        prompt,
    )
    cmd.Dir = r.repoPath
    output, err := cmd.CombinedOutput()
    // parse output...
}
```

### Chat API
- `POST /api/chat` - wiadomość od usera
- `GET /api/chat/stream` - SSE stream (czat + statusy agentów)
- `POST /api/workflow/:id/approve` - user akceptuje deploy
- `POST /api/workflow/:id/reject` - user odrzuca

---

## Model danych (SQLite)

### workflows
```sql
CREATE TABLE workflows (
    id          TEXT PRIMARY KEY,
    jira_ticket TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'CREATED',
    -- CREATED|DESCRIBING|CODING|TESTING|REVIEWING|AWAITING_APPROVAL|DEPLOYING|DONE|FAILED
    branch_name TEXT,
    spec        TEXT,            -- JSON: output od Descriptora
    retry_count INTEGER DEFAULT 0,
    error       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### tasks
```sql
CREATE TABLE tasks (
    id          TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id),
    type        TEXT NOT NULL,   -- DESCRIBE|CODE|TEST|REVIEW|DEPLOY|FIX
    status      TEXT NOT NULL DEFAULT 'QUEUED',
    -- QUEUED|RUNNING|COMPLETED|FAILED
    agent       TEXT NOT NULL,   -- descriptor|coder|tester|reviewer|deployer
    prompt      TEXT,            -- prompt wysłany do agenta
    output      TEXT,            -- raw output od agenta
    parsed      TEXT,            -- JSON: sparsowany wynik
    error       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at  DATETIME,
    completed_at DATETIME
);
```

### chat_messages
```sql
CREATE TABLE chat_messages (
    id          TEXT PRIMARY KEY,
    workflow_id TEXT REFERENCES workflows(id),
    role        TEXT NOT NULL,   -- user|agent|system
    agent_type  TEXT,            -- descriptor|coder|tester|reviewer|deployer|null
    content     TEXT NOT NULL,
    metadata    TEXT,            -- JSON
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

---

## Struktura projektu

```
decodo-workflow/
├── PLAN.md
├── .env.example                   # JIRA_URL, JIRA_TOKEN, GITLAB_TOKEN, etc.
│
├── server/                        # Go server
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── cmd/
│   │   └── server/
│   │       └── main.go            # Entry point
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go          # Env vars, config loading
│   │   ├── db/
│   │   │   ├── db.go              # SQLite connection
│   │   │   ├── migrations.go      # Auto-migrate on start
│   │   │   └── queries.go         # SQL queries
│   │   ├── models/
│   │   │   ├── workflow.go
│   │   │   ├── task.go
│   │   │   └── chat.go
│   │   ├── workflow/
│   │   │   ├── engine.go          # State machine
│   │   │   ├── transitions.go     # Logika przejść
│   │   │   └── prompt_builder.go  # Budowanie promptów z kontekstem
│   │   ├── agent/
│   │   │   ├── runner.go          # exec opencode run
│   │   │   └── parser.go          # Parsowanie outputu agentów
│   │   ├── integrations/
│   │   │   ├── jira/
│   │   │   │   ├── client.go
│   │   │   │   └── types.go
│   │   │   └── gitlab/
│   │   │       ├── client.go
│   │   │       └── types.go
│   │   └── api/
│   │       ├── router.go          # HTTP router (chi/echo/gin)
│   │       ├── handlers/
│   │       │   ├── chat.go
│   │       │   ├── workflow.go
│   │       │   └── health.go
│   │       └── sse/
│   │           └── broker.go      # SSE event broker
│   └── Makefile
│
├── opencode.json                  # Konfiguracja opencode (agents)
├── .opencode/
│   └── agents/                    # Definicje agentów
│       ├── descriptor.md
│       ├── coder.md
│       ├── tester.md
│       ├── reviewer.md
│       └── deployer.md
│
├── web/                           # Frontend - czat UI
│   ├── package.json
│   ├── src/
│   │   ├── app/
│   │   │   ├── layout.tsx
│   │   │   ├── page.tsx           # Dashboard
│   │   │   └── workflow/
│   │   │       └── [id]/
│   │   │           └── page.tsx   # Chat + timeline
│   │   ├── components/
│   │   │   ├── chat/
│   │   │   │   ├── chat-window.tsx
│   │   │   │   ├── message.tsx
│   │   │   │   └── input.tsx
│   │   │   ├── workflow/
│   │   │   │   ├── workflow-list.tsx
│   │   │   │   ├── task-timeline.tsx
│   │   │   │   └── agent-status.tsx
│   │   │   └── ui/
│   │   ├── lib/
│   │   │   ├── api.ts
│   │   │   └── store.ts
│   │   └── hooks/
│   │       ├── use-chat.ts
│   │       └── use-sse.ts
│   └── tailwind.config.ts
│
└── prompts/                       # Statyczne części promptów agentów
    ├── descriptor-guidelines.md   # Wytyczne jak opisywać zadania
    ├── coder-guidelines.md        # Konwencje kodu, styl
    ├── tester-guidelines.md       # Wytyczne testowania
    ├── reviewer-checklist.md      # Checklist review'u
    └── deployer-procedure.md      # Procedura deployu
```

---

## Prompt Builder - jak Go buduje prompty

```go
type PromptBuilder struct {
    jira    *jira.Client
    gitlab  *gitlab.Client
    db      *db.DB
}

func (pb *PromptBuilder) BuildDescribePrompt(ticket *jira.Issue) string {
    return fmt.Sprintf(`Przeanalizuj poniższy ticket Jira i stwórz specyfikację techniczną
opisującą jakie zmiany w kodzie należy wprowadzić i jakie testy napisać.

## Ticket: %s
**Summary**: %s
**Description**:
%s

**Acceptance Criteria**:
%s

**Priority**: %s
**Labels**: %s

## Repozytorium
Przeanalizuj strukturę projektu i zaproponuj konkretne pliki do zmiany.

## Format odpowiedzi
Odpowiedz WYŁĄCZNIE w formacie JSON:
{
  "summary": "krótki opis co trzeba zrobić",
  "code_changes": [
    {
      "file": "ścieżka/do/pliku",
      "action": "create|modify|delete",
      "description": "co zmienić"
    }
  ],
  "test_spec": [
    {
      "file": "ścieżka/do/testu",
      "cases": ["opis przypadku testowego"]
    }
  ]
}`,
        ticket.Key, ticket.Fields.Summary,
        ticket.Fields.Description,
        extractAC(ticket),
        ticket.Fields.Priority.Name,
        strings.Join(ticket.Fields.Labels, ", "),
    )
}
```

---

## Co potrzebuję aby zacząć implementację

### Wymagane od użytkownika
1. **Jira** - URL instancji, token API, email (do .env)
2. **GitLab** - URL, token API (do .env)
3. **LLM provider** - skonfigurowany w opencode (`opencode auth login`)
4. **Target repo** - ścieżka do repo na którym agenci pracują
5. **Preferencje**:
   - Branch bazowy (main/develop)?
   - Branch staging (stage)?
   - Go HTTP router (chi/echo/gin)?

### Wymagane na maszynie
- **Go 1.22+**
- **opencode** (zainstalowany globalnie)
- **Node.js 20+** (dla web UI)
- **pnpm** (dla web UI)
- **Git**

### NIE potrzebujemy
- ~~Docker~~ (SQLite, Go binary)
- ~~Redis~~ (Go in-memory + SQLite)
- ~~Node.js runtime do agentów~~ (opencode CLI)
- ~~Custom tools do API~~ (Go server buduje prompty)
- ~~Orchestrator agent~~ (Go workflow engine)

---

## Fazy implementacji

### Faza 1: Go Server + podstawowy flow
- [ ] Go project init (go mod, struktura katalogów)
- [ ] SQLite schema + migrations
- [ ] Config loading (.env)
- [ ] Agent runner (exec `opencode run`)
- [ ] Jira client (pobieranie ticketów)
- [ ] Prompt builder (descriptor)
- [ ] Workflow engine (state machine)
- [ ] HTTP API: POST /api/chat, GET /api/health
- [ ] Prosty test: ticket -> describe -> output w DB

### Faza 2: Pełny pipeline agentów
- [ ] Descriptor agent (.opencode/agents/descriptor.md)
- [ ] Coder agent + prompt builder
- [ ] Tester agent + prompt builder (z git diff)
- [ ] Reviewer agent + prompt builder
- [ ] Deployer agent
- [ ] Retry/fix loop
- [ ] SSE endpoint do streamowania statusów

### Faza 3: Web UI
- [ ] Next.js setup
- [ ] Chat window (wysyłanie wiadomości, odbiór SSE)
- [ ] Workflow dashboard (lista, statusy)
- [ ] Task timeline (wizualizacja kroków)
- [ ] Approve/reject buttons
- [ ] Agent status badges (kto teraz pracuje)

### Faza 4: Integracje + Polish
- [ ] GitLab client (MR, pipeline status)
- [ ] Jira status transitions
- [ ] Slack notifications (webhook)
- [ ] Audit log
- [ ] Error handling + graceful shutdown
- [ ] Rate limiting

### Faza 5: Production
- [ ] Auth (API key / JWT)
- [ ] Dockerfile (Go binary + static web)
- [ ] CI/CD dla samego decodo-workflow
- [ ] Monitoring + health checks
- [ ] Dokumentacja
