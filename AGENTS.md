# AGENTS.md — tsk

Guía para agentes sin contexto previo sobre este proyecto.

## Qué es

TUI + CLI para gestión de tareas, diseñada para programadores que trabajan
en 1-3 proyectos simultáneamente. Integración total con IA vía CLI:
agentes IA ejecutan `tsk add`, `tsk start`, `tsk done` etc. por shell.

## Stack

- Go 1.26+, module `taskd`
- `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2` (import paths `charm.land`, NO github.com/charmbracelet)
- `modernc.org/sqlite` (pure Go, sin CGO)
- `github.com/BurntSushi/toml`
- Binary name: **`tsk`** (no `taskd`)

## Comandos

```bash
make build          # build → ~/.local/bin/tsk
make test           # go vet + go test
make all            # test + build
make install        # alias de build
```

```bash
./scripts/gen-fixtures.sh   # (si existe) regenera datos de test
```

## Paso crucial tras cualquier cambio de código

**Desplegar el binario** (los tests/smoke con `go run` no actualizan el
instalado; el usuario ejecuta el bin de `~/.local/bin`, no el repo):

```bash
make build
```

Sin este paso, cualquier verificación que haga el usuario sobre la TUI usa la
versión vieja. Ejecutarlo SIEMPRE al terminar una tarea de código, después de
la verificación (`make test`).

## Arquitectura

```
cmd/taskd/main.go          → entry point: CLI dispatch → TUI
internal/cli/cli.go        → Run(args) bool — todos los subcomandos CLI
internal/config/config.go  → TOML config, XDG paths, defaults
internal/db/               → SQLite connection, WAL, migrations, CRUD
internal/model/            → Project, Task, workflow helpers
internal/tui/              → Bubbletea v2: Dashboard, List, Kanban, Detail modal
```

## Convenciones

- **Archivos**: `snake_case.go`
- **Tipos**: `PascalCase`
- **Comentarios**: español donde aplique
- **`internal/`** exclusivamente — no hay paquetes exportados
- **Config nunca falla**: defaults + warning pattern
- **Atomic writes**: tmp + rename para persistencia
- **Bubbletea v2**: imports con `charm.land/*` (NO `github.com/charmbracelet`)
- **Tests**: directamente sobre el model (como vroom/gitdash), white-box testing
- **Entrada CLI**: `internal/cli/cli.go` con `Run(args []string) bool`

## CLI — output

Todos los comandos devuelven JSON a stdout por defecto para integración con IA.
Flags `--json` explícitos en `project list`/`project show`/`stats` para output JSON.
Sin `--json`, usan tabwriter para output humano alineado.

## Binary name

El binario se llama **`tsk`**, no `taskd`. El nombre `taskd` es solo
el nombre del módulo Go y del directorio del proyecto.
