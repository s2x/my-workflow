# AI Ticket Creator - Specyfikacja Funkcji

## Część Biznesowa (Business Context)

### Problem
Tworzenie dobrych ticketów JIRA wymaga:
- Zrozumienia kontekstu biznesowego
- Znajomości technicznych szczegółów kodu
- Jasnego opisu wymagań i kryteriów akceptacji
- Czasu na analizę i dokumentację

Często deweloperzy tworzą zbyt ogólne lub nieprecyzyjne tickety, co prowadzi do:
- Nieporozumień w zespole
- Wielokrotnych iteracji
- Opóźnień w realizacji

### Rozwiązanie
System AI Ticket Creator pozwala na:
1. **Szybkie generowanie** - Wpisz ogólny pomysł, AI przeanalizuje kod i wygeneruje profesjonalny ticket
2. **Ulepszanie opisów** - Iteracyjne doprecyzowywanie za pomocą AI
3. **Analizę kodu** - AI bada istniejącą bazę kodu, aby zaproponować techniczne podejście
4. **Refinement** - Możliwość dodania uwag przed finalizacją

### Korzyści
- **Szybkość** - Tworzenie ticketu w minuty zamiast godzin
- **Jakość** - Spójne, profesjonalne opisy z analizą techniczną
- **Dokładność** - AI analizuje kod i sugeruje techniczne szczegóły
- **Standaryzacja** - Wszystkie tickety mają tę samą strukturę

---

## Część Techniczna (Technical Specification)

### Architektura

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   UI (React)    │────▶│  API (Go/Node)   │────▶│   AI Service    │
│                 │     │                  │     │  (OpenAI/Qwen)  │
│ - Nowy Ticket   │◀────│ - /api/tickets   │◀────│                 │
│ - Ulepsz        │     │   /generate      │     │ - Analiza kodu  │
│ - Refine        │     │ - /api/tickets   │     │ - Generowanie   │
│ - Done          │     │   /refine        │     │ - Ulepszanie    │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │   Database       │
                       │   (SQLite/PSQL)  │
                       └──────────────────┘
```

### UI Flow

#### 1. Przycisk "New AI Ticket"
**Lokalizacja:** Główna nawigacja / Dashboard
**Wygląd:** 
- Primary button z ikoną AI (⚡ lub 🤖)
- Tooltip: "Create ticket with AI assistance"

#### 2. Modal: Initial Input
**Tytuł:** "Create New Ticket with AI"
**Pola:**
```
┌─────────────────────────────────────────────┐
│  Describe what you want to build           │
│  ┌─────────────────────────────────────┐   │
│  │ [TextArea]                          │   │
│  │ "Add user authentication with       │   │
│  │  OAuth2 and session management"     │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  [Cancel]              [Generate with AI ▶]│
└─────────────────────────────────────────────┘
```

**Walidacja:**
- Minimum 10 znaków
- Maksimum 2000 znaków

#### 3. Modal: AI Processing
**Stan:** Loading
```
┌─────────────────────────────────────────────┐
│  🤖 AI is analyzing your request...         │
│                                             │
│  Analyzing codebase structure... ✓          │
│  Identifying affected components... ✓       │
│  Generating business description...         │
│  Generating technical specification...      │
│                                             │
│  [Spinner]                                  │
└─────────────────────────────────────────────┘
```

**Timeout:** 30 sekund
**Retry:** Możliwość ponowienia w przypadku błędu

#### 4. Modal: Generated Ticket Preview
**Tytuł:** "Review AI Generated Ticket"
**Sekcje:**
```
┌─────────────────────────────────────────────┐
│  Generated Ticket                           │
│                                             │
│  Title:                                     │
│  ┌─────────────────────────────────────┐   │
│  │ Implement OAuth2 Authentication     │   │
│  │ with Session Management             │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  Business Description:                      │
│  ┌─────────────────────────────────────┐   │
│  │ As a user, I want to log in using   │   │
│  │ my Google/Apple account so that...  │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  Technical Description:                     │
│  ┌─────────────────────────────────────┐   │
│  │ - Add OAuth2 provider integration   │   │
│  │ - Implement session middleware      │   │
│  │ - Store tokens securely             │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  [◀ Back]  [🔄 Regenerate]  [✓ Looks Good] │
└─────────────────────────────────────────────┘
```

**Przyciski:**
- **Back** - Powrót do initial input z zachowaniem tekstu
- **Regenerate** - Ponowne generowanie z tym samym inputem
- **Looks Good** - Przejście do refinement

#### 5. Modal: Refinement
**Tytuł:** "Refine Ticket Details"
**Layout:**
```
┌─────────────────────────────────────────────┐
│  Refine Description                         │
│                                             │
│  Current Description:                       │
│  ┌─────────────────────────────────────┐   │
│  │ [Generated text - read-only]        │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  Your Notes / Refinements:                  │
│  ┌─────────────────────────────────────┐   │
│  │ "Also add logout functionality      │   │
│  │  and remember me checkbox"          │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  [◀ Back]  [🔄 Apply Refinement]  [✓ Done] │
└─────────────────────────────────────────────┘
```

**Przyciski:**
- **Back** - Powrót do preview
- **Apply Refinement** - Wysłanie uwag do AI, które ulepszy opis
- **Done** - Finalizacja i utworzenie ticketu

**Flow Refinement:**
1. Użytkownik wpisuje uwagi
2. Klika "Apply Refinement"
3. AI przetwarza (loading state)
4. Wyświetla zaktualizowany opis
5. Można dodać kolejne refinements lub kliknąć Done

#### 6. Success Screen
```
┌─────────────────────────────────────────────┐
│  ✅ Ticket Created Successfully!            │
│                                             │
│  Ticket: LOCAL-12345                        │
│  Title: Implement OAuth2 Authentication...  │
│                                             │
│  [Go to Ticket]  [Create Another]  [Close]  │
└─────────────────────────────────────────────┘
```

**Akcje:**
- **Go to Ticket** - Przekierowanie do edycji ticketu
- **Create Another** - Reset i nowy ticket
- **Close** - Zamknięcie modala

### API Endpoints

#### 1. POST /api/tickets/generate
**Opis:** Generuje ticket na podstawie opisu użytkownika

**Request:**
```json
{
  "project_id": "uuid",
  "description": "Add user authentication with OAuth2",
  "context": {
    "files": ["auth.go", "middleware.go"],
    "tech_stack": ["Go", "React", "PostgreSQL"]
  }
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "title": "Implement OAuth2 Authentication with Session Management",
    "business_description": "As a user...",
    "technical_description": "Technical approach...",
    "suggested_priority": "HIGH",
    "estimated_complexity": "MEDIUM",
    "affected_components": ["auth", "middleware", "database"]
  }
}
```

#### 2. POST /api/tickets/refine
**Opis:** Ulepsza istniejący opis na podstawie uwag

**Request:**
```json
{
  "current_description": "...",
  "refinement_notes": "Also add logout functionality",
  "project_id": "uuid"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "title": "Updated title...",
    "business_description": "Updated business desc...",
    "technical_description": "Updated technical desc..."
  }
}
```

#### 3. POST /api/tickets
**Opis:** Tworzy finalny ticket (istniejący endpoint)

**Request:**
```json
{
  "project_id": "uuid",
  "title": "...",
  "description": "...",
  "priority": "HIGH",
  "complexity": "MEDIUM",
  "ai_generated": true,
  "ai_metadata": {
    "original_prompt": "...",
    "refinement_count": 2
  }
}
```

### AI Prompt Engineering

#### System Prompt dla Generowania
```
You are a senior product owner and technical lead. 
Analyze the codebase and user request to create a professional JIRA ticket.

Output format (JSON):
{
  "title": "Clear, actionable title (max 100 chars)",
  "business_description": "User story format: As a [role], I want [goal], so that [benefit]. Include acceptance criteria.",
  "technical_description": "Technical approach, affected files/components, suggested implementation steps",
  "priority": "HIGH|MEDIUM|LOW",
  "complexity": "SIMPLE|MEDIUM|COMPLEX",
  "affected_components": ["list of components"]
}

Rules:
- Be specific and actionable
- Include measurable acceptance criteria
- Reference existing code patterns
- Suggest testing approach
```

#### System Prompt dla Refinement
```
You are improving an existing ticket description based on user feedback.

Current ticket: {current_description}
User feedback: {refinement_notes}

Update the ticket to incorporate the feedback while maintaining:
- Clarity and actionability
- Consistent structure
- Technical accuracy

Return updated JSON with the same structure as generation.
```

### Database Schema Changes

#### Tabela: tickets
Dodać kolumny:
```sql
ALTER TABLE tickets ADD COLUMN ai_generated BOOLEAN DEFAULT FALSE;
ALTER TABLE tickets ADD COLUMN ai_metadata JSON DEFAULT '{}';
ALTER TABLE tickets ADD COLUMN refinement_count INTEGER DEFAULT 0;
```

#### Nowa Tabela: ai_ticket_sessions (opcjonalnie)
```sql
CREATE TABLE ai_ticket_sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    user_id TEXT NOT NULL,
    original_prompt TEXT NOT NULL,
    generated_title TEXT,
    generated_description TEXT,
    refinement_history JSON DEFAULT '[]',
    status TEXT DEFAULT 'DRAFT', -- DRAFT, REFINING, COMPLETED
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Frontend Components

#### Nowe komponenty:
1. **AITicketCreatorModal** - Główny modal zarządzający flow
2. **InitialInputStep** - Formularz początkowy
3. **ProcessingStep** - Loading state z progress
4. **PreviewStep** - Podgląd wygenerowanego ticketu
5. **RefinementStep** - Formularz z uwagami
6. **SuccessStep** - Ekran sukcesu

#### Modyfikacje istniejących:
1. **Navigation** - Dodanie przycisku "New AI Ticket"
2. **TicketList** - Oznaczenie AI-generated ticketów (ikona 🤖)

### Backend Changes

#### Nowe endpointy:
- `POST /api/tickets/generate`
- `POST /api/tickets/refine`

#### Modyfikacje:
- `POST /api/tickets` - obsługa `ai_generated` i `ai_metadata`
- `GET /api/tickets/:id` - zwracanie AI metadata

#### Nowy serwis: AITicketService
```go
type AITicketService interface {
    GenerateTicket(ctx context.Context, req GenerateRequest) (*GeneratedTicket, error)
    RefineTicket(ctx context.Context, req RefineRequest) (*GeneratedTicket, error)
    AnalyzeCodebase(ctx context.Context, projectID string) (*CodebaseAnalysis, error)
}
```

### Security & Rate Limiting

#### Rate Limiting:
- Generowanie: 10 req/hour per user
- Refinement: 20 req/hour per user

#### Walidacja:
- Maksymalna długość promptu: 2000 znaków
- Sanitizacja inputu (XSS prevention)
- Walidacja project_id (user ma dostęp?)

### Error Handling

#### Scenariusze błędów:
1. **AI Timeout** - Pokazać "AI is taking longer than expected. Try again?"
2. **Invalid Input** - Walidacja inline z sugestiami
3. **Code Analysis Failed** - Kontynuować bez analizy kodu (basic mode)
4. **Rate Limit** - Pokazać cooldown timer

### Future Enhancements

1. **Templates** - Predefiniowane szablony ticketów (Bug, Feature, Refactor)
2. **Voice Input** - Dyktowanie opisu
3. **Image Analysis** - Upload screenshot/mockup, AI generuje opis
4. **Batch Creation** - Tworzenie wielu ticketów z jednego opisu (epic breakdown)
5. **Smart Suggestions** - AI proponuje tickety na podstawie zmian w kodzie

---

## Acceptance Criteria

- [ ] Użytkownik może kliknąć "New AI Ticket" z głównego menu
- [ ] Modal pokazuje się z polem tekstowym na opis
- [ ] Po kliknięciu "Generate" pokazuje się loading state
- [ ] AI generuje tytuł, opis biznesowy i techniczny
- [ ] Użytkownik może kliknąć "Regenerate" aby wygenerować ponownie
- [ ] Użytkownik może dodać uwagi w textarea "Refinement"
- [ ] Po kliknięciu "Apply Refinement" AI ulepsza opis
- [ ] Po kliknięciu "Done" ticket jest tworzony w systemie
- [ ] Użytkownik jest przekierowany do edycji ticketu
- [ ] AI-generated tickety mają oznaczenie w liście (ikona 🤖)
- [ ] Rate limiting działa (max 10 generacji/godz)
- [ ] Wszystkie błędy są obsłużone z przyjaznymi komunikatami

---

## Estymacja

**Frontend:** 2-3 dni
**Backend:** 2-3 dni  
**AI Integration:** 1-2 dni
**Testing:** 1-2 dni

**Razem:** 6-10 dni roboczych
