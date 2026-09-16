# Plan: taskd — Task Manager TUI + CLI para IA

## Resumen

`taskd` es un gestor de tareas TUI+CLI diseñado para programadores que trabajan en 1-3 proyectos simultáneamente. La clave es la **integración total con IA**: una CLI que los agentes IA pueden ejecutar vía shell para crear, asignar, iniciar, y completar tareas.

---

## Decisiones Cerradas

| Decisión | Elección | Razón |
|----------|----------|-------|
| Lenguaje | Go 1.26.3 | Consistente con vroom/gitdash en el mismo directorio |
| CLI framework | Manual (`Run(args) bool`) | Match con convenciones existentes |
| TUI | Bubbletea v2 (`charm.land/*`) | Misma librería que vroom/gitdash |
| Storage | SQLite (WAG mode) | Soporte concurrencia CLI+TUI simultáneo |
| Driver | `modernc.org/sqlite` | Pure Go, sin CGO, distribución simple |
| Config | TOML via BurntSushi/toml | XDG paths, defaults + warning |
| Integración IA | CLI subcommands con output JSON | Más simple que MCP para este caso |
| Descripciones | Markdown | Soporte completo en TUI con viewport |
| Proyectos | Registro explícito (`taskd project add`) | Sin auto-detección |
| Workflows | Configurables por proyecto | Cada proyecto define sus propios estados |
| Assignee | Campo texto libre en task | Simple, sin sistema de miembros formal |

---

## Estructura del Proyecto

```
taskd/
├── cmd/taskd/
│   └── main.go                    # Entry point: CLI dispatch → TUI
├── internal/
│   ├── cli/
│   │   └── cli.go                 # Run(args) bool — todos los subcomandos
│   ├── config/
│   │   └── config.go              # TOML config, XDG paths, defaults+warning
│   ├── db/
│   │   ├── db.go                  # SQLite connection, WAL setup, auto-migrate
│   │   ├── migrations.go          # embed: schema SQL por versión
│   │   ├── projects.go            # Project CRUD + workflow management
│   │   └── tasks.go               # Task CRUD + queries
│   ├── model/
│   │   ├── project.go             # Project struct + workflow helpers
│   │   ├── task.go                # Task struct + Status type
│   │   └── view.go                # Dashboard/Kanban/List data types
│   └── tui/
│       ├── app.go                 # Model principal, Init, Update, View
│       ├── dashboard.go           # Dashboard panel (stats + team)
│       ├── list.go                # Lista ordenada por prioridad
│       ├── kanban.go              # Kanban board dinámico
│       ├── styles.go              # Lipgloss styles
│       ├── keybindings.go         # Key handling configurable
│       └── filters.go             # Barra de filtro por proyecto/status/assignee
├── go.mod
├── go.sum
└── AGENTS.md
```

---

## Schema SQLite

```sql
-- Migration 001: initial schema

CREATE TABLE IF NOT EXISTS _meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    path       TEXT NOT NULL,
    workflow   TEXT NOT NULL DEFAULT '["backlog","todo","in_progress","review","done"]',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id   INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'backlog',
    priority     INTEGER NOT NULL DEFAULT 0,
    assignee     TEXT NOT NULL DEFAULT '',
    position     INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);
```

### Notas del Schema

- **`projects.workflow`**: JSON array de strings. El último elemento es el estado terminal "done". `cancelled` siempre está implícito (disponible desde cualquier estado).
- **`tasks.status`**: Sin CHECK constraint — los válidos se determinan dinámicamente del workflow del proyecto padre.
- **`tasks.priority`**: 0=none, 1=low, 2=medium, 3=high.
- **`tasks.assignee`**: Texto libre, ej: "@juan".
- **`tasks.position`**: Para orden dentro de una columna/status en kanban.
- **`ON DELETE CASCADE`**: Eliminar proyecto elimina sus tareas.
- **`_meta`**: Tabla para versionado de migraciones.

---

## Workflows Configurables por Proyecto

Cada proyecto define su propio flujo de estados. El último elemento del array es el estado terminal "done".

### Ejemplos

```bash
# Proyecto simple
taskd project add cli-tool --path ~/dev/cli \
  --workflow "todo,in_progress,done"

# Proyecto con review y pre-prod
taskd project add web-app --path ~/dev/web-app \
  --workflow "todo,in_progress,review,pre,done"

# Proyecto completo
taskd project add api --path ~/dev/api \
  --workflow "backlog,todo,in_progress,review,done"
```

### Reglas de Workflow

1. **El último estado** del array es siempre el terminal "done" (equivalente a done/completado)
2. **`cancelled`** siempre está implícito — disponible desde cualquier estado, no necesita estar en el array
3. **No se pueden duplicar** nombres de estado dentro de un workflow
4. **No se puede eliminar** un estado que tenga tareas (sin `--force`)
5. **`--force`** reasigna tareas de estados eliminados al primer estado del nuevo workflow

### Validación al Cambiar Workflow

```bash
taskd project update api --workflow "todo,in_progress,done"
```

Respuesta si hay tareas afectadas:

```json
{
  "ok": false,
  "error": "Cannot remove status 'review' — 3 tasks still in this status",
  "affected_tasks": [
    {"id": 28, "title": "Update README", "status": "review"},
    {"id": 33, "title": "Fix CORS", "status": "review"},
    {"id": 41, "title": "Update deps", "status": "review"}
  ],
  "hint": "Move or cancel these tasks first, or use --force to reassign them to 'todo'"
}
```

### CLI para Gestión de Workflows

```bash
# Ver workflow de un proyecto
taskd project show api
# → {"project":{"name":"api","workflow":["backlog","todo","in_progress","review","done"]}}

# Listar todos los proyectos con workflows
taskd project list
# → {"projects":[
#     {"name":"api","workflow":["backlog","todo",...],"task_count":12},
#     {"name":"web-app","workflow":["todo","in_progress",...],"task_count":3}
#   ]}

# Modificar workflow
taskd project update api --workflow "backlog,todo,in_progress,review,pre_prod,done"
```

---

## CLI Completo para Integración con IA

Todos los comandos imprimen JSON a stdout. Errores a stderr como `{"error":"..."}`.

### Gestión de Proyectos

| Comando | Descripción | Ejemplo |
|---------|-------------|---------|
| `taskd project add <name> [--path] [--workflow]` | Registrar proyecto | `taskd project add api --path ~/dev/api --workflow "backlog,todo,in_progress,review,done"` |
| `taskd project list` | Listar proyectos | `taskd project list` |
| `taskd project show <name>` | Ver detalle + workflow | `taskd project show api` |
| `taskd project update <name> [--workflow] [--path]` | Actualizar proyecto | `taskd project update api --workflow "todo,in_progress,done"` |
| `taskd project remove <name>` | Eliminar proyecto + tareas | `taskd project remove api` |

### Gestión de Tareas

| Comando | Descripción | Ejemplo |
|---------|-------------|---------|
| `taskd add <title> --project X [--priority N] [--assignee @name] [--status S]` | Crear tarea | `taskd add "Fix auth" --project api --priority 3 --assignee @juan` |
| `taskd list [--project X] [--status S] [--assignee A] [--json]` | Listar tareas | `taskd list --project api --status in_progress --json` |
| `taskd show <id>` | Ver detalle completo | `taskd show 42` |
| `taskd update <id> [--title] [--description] [--priority N] [--assignee @name]` | Actualizar metadata | `taskd update 42 --title "Fix N+1" --priority 2` |
| `taskd move <id> <status>` | Mover a status específico | `taskd move 42 review` |
| `taskd start <id>` | Mover al 2do estado del workflow | `taskd start 42` |
| `taskd review <id>` | Mover a "review" (si existe en workflow) | `taskd review 42` |
| `taskd done <id>` | Mover al último estado (done terminal) | `taskd done 42` |
| `taskd cancel <id>` | Cancelar tarea | `taskd cancel 42` |
| `taskd reorder <id> <position>` | Reordenar dentro de columna | `taskd reorder 42 0` |
| `taskd stats [--project X]` | Estadísticas | `taskd stats --project api` |

### Migración

| Comando | Descripción |
|---------|-------------|
| `taskd migrate` | Ejecutar migraciones pendientes |

### Comportamiento de `start`, `review`, `done`

- **`taskd start <id>`**: Mueve al **segundo estado** del workflow (después de backlog si existe, si no al primero). Ej: si workflow es `["backlog","todo","in_progress",...]` → mueve a "todo".
- **`taskd review <id>`**: Mueve al estado que contenga "review" en su nombre (búsqueda parcial). Si no existe, error.
- **`taskd done <id>`**: Mueve al **último estado** del workflow (el terminal).

### Output JSON (ejemplo)

```bash
taskd add "Fix N+1 query" --project api --priority 3 --assignee @juan
```

```json
{
  "ok": true,
  "task": {
    "id": 17,
    "title": "Fix N+1 query",
    "project": "api",
    "status": "backlog",
    "priority": 3,
    "assignee": "@juan",
    "created_at": "2026-09-11T14:30:00Z"
  }
}
```

```bash
taskd list --project api --status in_progress --json
```

```json
{
  "tasks": [
    {"id": 17, "title": "Fix N+1 query", "status": "in_progress", "priority": 3, "assignee": "@juan"},
    {"id": 23, "title": "Caching layer", "status": "in_progress", "priority": 2, "assignee": "@maria"}
  ]
}
```

---

## TUI — 3 Vistas

### Navegación General

```
                          ┌─────────────────┐
                          │   taskd (no args)│
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐
                          │    DASHBOARD     │
                          │                  │
                          │  [1] [2] [3]  q  │
                          └──┬───────┬───┬───┘
                             │       │   │
              ┌──────────────┘       │   └──────────┐
              │                      │              │
         [2] / Tab             [3] / Tab            q
              │                      │              │
              ▼                      ▼              ▼
      ┌──────────────┐       ┌──────────────┐  ┌─────────┐
      │     LIST      │       │    KANBAN     │  │  EXIT   │
      │               │       │               │  └─────────┘
      │  j k s d x    │       │  h j k l s S  │
      │  Enter / F S A│       │  Enter d x    │
      └───────┬───────┘       └───────┬───────┘
              │                       │
              │        Enter          │
              └───────┬───────────────┘
                      ▼
             ┌──────────────┐
             │  TASK DETAIL  │
             │   (modal)     │
             │               │
             │  a s d x e    │
             │  Esc close    │
             └───────────────┘
```

---

### Vista 1: Dashboard

Muestra stats por proyecto (con sus workflows), team workload, y tareas activas.

```
┌─ taskd ──────────────────────────────────────────────────────────────────────┐
│                                                                              │
│  Projects:  api(12)  web-app(3)  cli-tool(1)                                │
│  ─────────────────────────────────────────────────────────────────────────── │
│                                                                              │
│  ┌─ Overview ──────────────────────┐  ┌─ Active ───────────────────────────┐ │
│  │                                  │  │                                    │ │
│  │  Total         16               │  │  IN PROGRESS                       │ │
│  │  ─────────────────────────────  │  │  ┌──────────────────────────────┐  │ │
│  │  Backlog        4  ░░░░░░░░░░   │  │  │ 17  Fix N+1 query     @juan │  │ │
│  │  Todo           3  ░░░░░░       │  │  │ 23  Caching layer     @maria │  │ │
│  │  In Progress    3  ░░░░░░░░░    │  │  │ 31  Refactor auth     @pedro │  │ │
│  │  Review         1  ░░           │  │  └──────────────────────────────┘  │ │
│  │  Done           4  ░░░░░░░░░░   │  │                                    │ │
│  │  Cancelled      1  ░░           │  │  REVIEW                           │ │
│  │                                  │  │  ┌──────────────────────────────┐  │ │
│  │  Team Workload                  │  │  │ 28  Update README     @juan  │  │ │
│  │  ─────────────────────────────  │  │  └──────────────────────────────┘  │ │
│  │  @juan   5 tasks (2 in prog)   │  │                                    │ │
│  │  @maria  4 tasks (1 in prog)   │  │                                    │ │
│  │  @pedro  3 tasks (1 in prog)   │  │                                    │ │
│  │  unassigned  4                 │  │                                    │ │
│  │                                  │  │                                    │ │
│  └──────────────────────────────────┘  └────────────────────────────────────┘ │
│                                                                              │
│  [1] Dashboard  [2] List  [3] Kanban    / filter    q quit                   │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Navegación:**
- `2` o `Tab` → List
- `3` o `Tab` → Kanban
- `/` → Filtrar por proyecto
- `q` → Salir

---

### Vista 2: List

Lista ordenada por prioridad, con filtros por proyecto/status/assignee. Soporte multi-proyecto.

```
┌─ taskd ── List ── api + web-app ─────────────────────────────────────────────┐
│                                                                              │
│  Filter: api, web-app    Priority: all    Status: all    Assignee: all       │
│  ─────────────────────────────────────────────────────────────────────────── │
│                                                                              │
│  ID   Priority  Status        Assignee  Title                    Project     │
│  ───  ────────  ────────────  ────────  ───────────────────────  ─────────  │
│  17   ███ HIGH  in_progress   @juan     Fix N+1 query            api        │
│  31   ███ HIGH  in_progress   @pedro    Refactor auth module      api        │
│  45   ███ HIGH  todo          @maria    Fix checkout bug          web-app    │
│  7    ██  MED   todo          @maria    API docs                  api        │
│  23   ██  MED   in_progress   @maria    Add caching layer         api        │
│  28   ██  MED   review        @juan     Update README             api        │
│  52   ██  MED   in_progress   @juan     Add search filters        web-app    │
│  9    █   LOW   backlog       unassigned Error handling            api        │
│  10   █   LOW   backlog       unassigned Rate limiting             api        │
│  61   █   LOW   backlog       unassigned Update dependencies       web-app    │
│                                                                              │
│  ─────────────────────────────────────────────────────────────────────────── │
│  Total: 11 tasks    Selected: #17 — Fix N+1 query                           │
│                                                                              │
│  [1] Dashboard  [2] List  [3] Kanban    / filter    q quit                   │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Filtros disponibles:**
- `/` → Filtrar por proyecto
- `F` → Filtrar por prioridad (none/low/med/high)
- `S` → Filtrar por status (muestra estados del workflow del proyecto)
- `A` → Filtrar por assignee

**Navegación:**
- `j`/`↓` y `k`/`↑` → Navegar entre tareas
- `Enter` → Ver detalle (modal)
- `s` → Start (mover a in_progress)
- `d` → Done
- `x` → Cancel
- `Tab` → Cambiar a siguiente vista

---

### Vista 3: Kanban

Columnas dinámicas según el workflow del proyecto. Multi-proyecto muestra unión de workflows.

**Proyecto "api"** con workflow `["backlog", "todo", "in_progress", "review", "done"]`:

```
┌─ taskd ── Kanban ── api ─────────────────────────────────────────────────────┐
│                                                                              │
│  ┌─ backlog (4) ──────┐ ┌─ todo (3) ───────┐ ┌─ in_progress (3) ────────┐  │
│  │                      │ │                    │ │                            │  │
│  │  9  Error handling   │ │  7  API docs       │ │ ▶ 17  Fix N+1 query  P3  │  │
│  │     @unassigned      │ │     @maria         │ │     @juan                │  │
│  │  10  Rate limiting   │ │  8  Logging        │ │  23  Caching layer   P2  │  │
│  │     @unassigned      │ │     @unassigned    │ │     @maria               │  │
│  │  11  Input valid.    │ │  12  Metrics       │ │  31  Refactor auth   P3  │  │
│  │     @pedro           │ │     @unassigned    │ │     @pedro               │  │
│  │  14  Graceful shutd. │ │                    │ │                            │  │
│  │     @unassigned      │ │                    │ │                            │  │
│  └──────────────────────┘ └────────────────────┘ └────────────────────────────┘  │
│  ┌─ review (1) ───────┐ ┌─ done (4) ────────┐ ┌─ cancelled (1) ─────────┐  │
│  │                      │ │                    │ │                            │  │
│  │  28  Update README   │ │  1  Setup repo     │ │  15  Old auth module      │  │
│  │     @juan            │ │     @pedro         │ │     @maria               │  │
│  │                      │ │  2  Init DB        │ │                            │  │
│  │                      │ │     @pedro         │ │                            │  │
│  │                      │ │  3  Config setup   │ │                            │  │
│  │                      │ │     @pedro         │ │                            │  │
│  │                      │ │  4  CI pipeline    │ │                            │  │
│  │                      │ │     @juan          │ │                            │  │
│  └──────────────────────┘ └────────────────────┘ └────────────────────────────┘  │
│                                                                              │
│  [1] Dashboard  [2] List  [3] Kanban    / filter    q quit                   │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Filtro multi-proyecto** (unión de workflows):

```
┌─ Kanban ── api + web-app ────────────────────────────────────────────────────┐
│                                                                              │
│ ┌─ backlog ──┐ ┌─ todo ─────┐ ┌─ in_progress ─┐ ┌─ review ─┐ ┌─ pre ────┐ │
│ │  api:9     │ │  api:7     │ │  api:17       │ │  api:28  │ │ web:—    │ │
│ │  api:10    │ │  api:8     │ │  web:52 ▶     │ │  web:55  │ │          │ │
│ │            │ │  web:45    │ │  api:23       │ │          │ │          │ │
│ └────────────┘ │  web:46    │ │  api:31       │ └──────────┘ └──────────┘ │
│                └────────────┘ └───────────────┘                            │
│                                                                             │
│ ┌─ done ─────────────────────────────────────────────────────────────────┐  │
│ │  api:1,2,3,4    web:47,49,50                                          │  │
│ └────────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Navegación en Kanban:**
- `h` / `←` → Columna izquierda
- `l` / `→` → Columna derecha
- `j` / `↓` → Siguiente tarea en columna
- `k` / `↑` → Tarea anterior en columna
- `s` → Mover tarea a la derecha (avanzar status)
- `S` → Mover tarea a la izquierda (retroceder status)
- `Enter` → Ver detalle de tarea
- `d` → Marcar done (último estado del workflow)
- `x` → Cancelar

---

### Vista 4: Detalle de Tarea (Modal)

```
  ╔══════════════════════════════════════════════════════════════════════════════╗
  ║  #17 — Fix N+1 query in UserList                             P3 ███        ║
  ║  ──────────────────────────────────────────────────────────────────────────  ║
  ║  Status:     in_progress                  Project:   api                   ║
  ║  Assignee:   @juan                        Created:   2026-09-10 14:30      ║
  ║  Updated:    2h ago                       Completed: —                     ║
  ║  ──────────────────────────────────────────────────────────────────────────  ║
  ║  Description (Markdown):                                                    ║
  ║  ┌───────────────────────────────────────────────────────────────────────┐  ║
  ║  │ The `getUserWithPosts` endpoint fires one query per user             │  ║
  ║  │ to fetch posts. With 500 users this means 500 queries.              │  ║
  ║  │                                                                      │  ║
  ║  │ ## Plan                                                              │  ║
  ║  │ - [x] Add eager loading for posts relationship                      │  ║
  ║  │ - [ ] Add batch query for comments                                   │  ║
  ║  │ - [ ] Run benchmarks before/after                                    │  ║
  ║  └───────────────────────────────────────────────────────────────────────┘  ║
  ║  ──────────────────────────────────────────────────────────────────────────  ║
  ║  Actions: [a] Assign  [s] Start  [d] Done  [x] Cancel  [e] Edit  Esc close║
  ╚══════════════════════════════════════════════════════════════════════════════╝
```

**Acciones del modal:**
- `a` → Asignar persona (abre textinput)
- `s` → Start task
- `d` → Mark done
- `x` → Cancel task
- `e` → Editar título/descripción
- `j`/`k` → Scroll en descripción markdown
- `Esc` → Cerrar modal

---

## Mapa de Teclado Completo

| Vista | Tecla | Acción |
|-------|-------|--------|
| **Global** | `1` | Dashboard |
| **Global** | `2` | List |
| **Global** | `3` | Kanban |
| **Global** | `Tab` | Siguiente vista |
| **Global** | `/` | Filtro por proyecto |
| **Global** | `Esc` | Cerrar modal / Limpiar filtro |
| **Global** | `q` | Salir |
| **List** | `j`/`↓` | Siguiente tarea |
| **List** | `k`/`↑` | Tarea anterior |
| **List** | `F` | Filtro por prioridad |
| **List** | `S` | Filtro por status |
| **List** | `A` | Filtro por assignee |
| **Kanban** | `h`/`←` | Columna izquierda |
| **Kanban** | `l`/`→` | Columna derecha |
| **Kanban** | `j`/`↓` | Siguiente tarea en columna |
| **Kanban** | `k`/`↑` | Tarea anterior en columna |
| **Kanban** | `s` | Mover derecha (avanzar status) |
| **Kanban** | `S` | Mover izquierda (retroceder status) |
| **Ambas** | `Enter` | Ver detalle |
| **Ambas** | `d` | Marcar done |
| **Ambas** | `x` | Cancelar |
| **Modal** | `a` | Asignar persona |
| **Modal** | `e` | Editar tarea |
| **Modal** | `j`/`k` | Scroll descripción |
| **Modal** | `Esc` | Cerrar |

---

## Convenciones de Código

Siguiendo vroom/gitdash:

- **Archivos**: `snake_case.go`
- **Tipos**: `PascalCase`
- **Comentarios**: español donde aplique
- **`internal/`** exclusivamente — no hay paquetes exportados
- **Config nunca falla**: defaults + warning pattern
- **Atomic writes**: tmp + rename para persistencia
- **Build**: `go build -o ~/.local/bin/taskd ./cmd/taskd`
- **Bubbletea v2**: imports con `charm.land/*` (NO `github.com/charmbracelet`)
- **Tests**: directamente sobre el model (como vroom/gitdash), white-box testing
- **Entrada CLI**: `internal/cli/cli.go` con `Run(args []string) bool`

---

## Dependencias

```go
require (
    charm.land/bubbletea/v2    v2.x.x
    charm.land/bubbles/v2      v2.x.x
    charm.land/lipgloss/v2     v2.x.x
    modernc.org/sqlite         v1.x.x
    github.com/BurntSushi/toml v1.x.x
)
```

---

## Flujo de Trabajo de la IA (ejemplo completo)

```bash
# 1. Setup: crear proyecto con workflow custom
taskd project add backend --path ~/dev/backend \
  --workflow "backlog,todo,in_progress,review,pre_prod,done"

# 2. IA descubre bug durante code review
taskd add "Fix N+1 query in UserList" --project backend --priority 3 --assignee @juan
# → {"ok":true,"task":{"id":17,...}}

# 3. IA empieza a trabajar
taskd start 17
# → {"ok":true,"task":{"id":17,"status":"todo"},"next":"in_progress"}

# 4. IA avanza a in_progress
taskd move 17 in_progress
# → {"ok":true,"task":{"id":17,"status":"in_progress"}}

# 5. IA termina, marca para review
taskd review 17
# → {"ok":true,"task":{"id":17,"status":"review"}}

# 6. Humano revisa, pasa a pre-prod
taskd move 17 pre_prod
# → {"ok":true,"task":{"id":17,"status":"pre_prod"}}

# 7. QA aprueba
taskd done 17
# → {"ok":true,"task":{"id":17,"status":"done","completed_at":"2026-09-11T..."}}

# 8. IA verifica qué hay pendiente
taskd list --status in_progress --json
# → {"tasks":[...]}

# 9. IA checkea carga del equipo
taskd stats --project backend
# → {"stats":{"total":15,"by_status":{...},"by_assignee":{...}}}
```

---

## Fases de Implementación

### Fase 1: Core (Storage + CLI)

**Entregable**: Binario `taskd` que gestiona tareas vía CLI. Sin TUI.

Archivos:
- `cmd/taskd/main.go` — entry point
- `internal/cli/cli.go` — dispatch de subcomandos
- `internal/db/db.go` — SQLite connection, WAL, auto-migrate
- `internal/db/migrations.go` — schema embebido
- `internal/db/projects.go` — CRUD proyectos + workflows
- `internal/db/tasks.go` — CRUD tareas + queries
- `internal/model/project.go` — Project struct + workflow helpers
- `internal/model/task.go` — Task struct + Status type
- `internal/config/config.go` — TOML config
- `go.mod` — dependencias
- Tests para db layer (`:memory:` SQLite)

### Fase 2: TUI

**Entregable**: TUI completa con Dashboard + List + Kanban.

Archivos adicionales:
- `internal/tui/app.go` — Model principal, tab switching
- `internal/tui/dashboard.go` — panel stats + team workload
- `internal/tui/list.go` — lista ordenada por prioridad
- `internal/tui/kanban.go` — board dinámico con columnas por workflow
- `internal/tui/styles.go` — estilos lipgloss
- `internal/tui/keybindings.go` — teclado configurable
- `internal/tui/filters.go` — barra de filtro

### Fase 3: Polish

**Entregable**: Tool production-ready.

- `--json` flag en todos los comandos list/show
- Tabwriter para output humano
- Stats por proyecto y por assignee
- Shell completions (bash/zsh/fish)
- Mouse support en kanban
- Tests TUI (model testing como vroom/gitdash)
- Soporte markdown renderizado en viewport

---

## Preguntas Abiertas (para futuras sesiones)

1. **Posición de tareas**: ¿Drag & drop con mouse en kanban o solo teclado?
2. **Tags/labels**: ¿Necesitas etiquetas para categorizar tareas más allá de prioridad?
3. **Subtareas**: ¿Necesitarás anidar tareas (parent/child)?
4. **Notificaciones**: ¿Notificar cuando una tarea cambia de estado (para workflow de equipo)?
5. **Export**: ¿Necesitar exportar a JSON/CSV para reports?
