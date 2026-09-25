# tsk

[![CI](https://github.com/Sovengar/tsk/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Sovengar/tsk/actions/workflows/ci.yml)

Gestor de tareas en TUI + CLI pensado para programadores que alternan entre
**1-3 proyectos**. Dashboard, lista y Kanban en la terminal, con workflow
configurable por proyecto y un Gantt que proyecta la cola de cada persona.
Todos los comandos hablan **JSON**, así que los agentes de IA pueden crear,
arrancar y cerrar tareas por shell (`tsk add`, `tsk start`, `tsk done`, …).

## Características

- **Cuatro vistas** con conmutación por tecla: Dashboard, List, Kanban y Gantt.
- **Workflow por proyecto**: la secuencia de estados (p. ej. `backlog → todo →
  doing → review → done`) se define al registrar el proyecto y debe incluir
  `done`; el orden de presentación de la lista es configurable e independiente.
- **Kanban por estado**: una columna por estado del workflow del proyecto.
- **Gantt de capacidad**: proyecta la cola de cada persona en días laborables,
  respeta los *off-days* y muestra las tareas sin responsable. `--project` y
  `--assignee` filtran la vista, no recalculan fechas: la capacidad es una sola
  y se reparte entre proyectos.
- **Prioridad, asignado, estimación y tags** por tarea, con filtrado por
  proyecto, estado, persona y tag.
- **Comentarios** por tarea y **off-days** por persona (rango de fechas + nota).
- **Archivado reversible de proyectos** (oculta él, sus tareas y comentarios sin
  borrarlos).
- **CLI JSON-first** para integración con agentes, con `completion` para
  bash/zsh/fish y `migrate` para las migraciones de la base de datos.
- **SQLite embebido** (pure Go, sin CGO) y **config TOML** opcional; sin fichero
  de config funciona con defaults.

## Instalación

Requiere **Go 1.26+**.

```bash
make build      # compila e instala en ~/.local/bin/tsk
make install    # alias de build
```

O directamente:

```bash
go build -o ~/.local/bin/tsk ./cmd/tsk
```

El binario se llama **`tsk`** (`taskd` es solo el nombre del módulo Go).

## Uso

```bash
tsk            # lanza la TUI (sin argumentos)
tsk --help     # lista de comandos
```

### CLI

Ejemplo de ciclo completo:

```bash
# Registrar un proyecto con su workflow
tsk project add web --workflow backlog,todo,doing,review,done --list-order todo,doing,review,done

# Crear una tarea
tsk add "Arreglar redirect del login" --project web \
  --priority 3 --assignee @ana --estimate 0.5 --tag bug,auth

tsk list --project web --status todo   # filtrar
tsk start 1                            # mover a "doing"
tsk review 1                           # mover a "review"
tsk done 1                             # cerrar
tsk gantt --assignee @ana --weeks 4    # proyectar la cola de @ana
```

Comandos disponibles:

| Comando | Qué hace |
|---|---|
| `tsk project add\|list\|show\|update\|remove\|archive\|unarchive` | Gestión de proyectos y su workflow |
| `tsk add <title> --project X [--priority N] [--assignee @name] [--estimate N] [--tag T]` | Crear tarea |
| `tsk list [--project X] [--status S] [--assignee A] [--tag T]` | Listar tareas |
| `tsk show <id>` | Detalle de una tarea (con comentarios) |
| `tsk update <id> [--title] [--description] [--priority] [--assignee] [--estimate] [--tag] [--untag] [--tags]` | Editar metadatos |
| `tsk move <id> <status>` | Mover a un estado concreto |
| `tsk start\|review\|done\|cancel <id>` | Atajos de workflow |
| `tsk comment add\|list\|remove` | Comentarios de tarea |
| `tsk offday add\|list\|remove` | Días no laborables por persona |
| `tsk gantt [--project] [--assignee] [--from] [--weeks] [--json]` | Proyección por persona |
| `tsk stats [--project X]` | Estadísticas por estado y persona |
| `tsk migrate` | Aplicar migraciones pendientes |
| `tsk completion bash\|zsh\|fish` | Autocompletado del shell |

Los comandos de acción (`add`, `update`, `move`, `start`, `done`, `cancel`,
`show`, `comment`, …) devuelven **JSON a stdout**. Los de listado
(`list`, `project list`, `project show`, `offday list`, `gantt`, `stats`)
aceptan `--json`; sin él imprimen una tabla alineada.

### TUI

| Tecla | Acción |
|---|---|
| `1`/`2`/`3`/`4` | Cambiar a List / Kanban / Gantt / Dashboard |
| `hjkl`, flechas | Navegar |
| `Tab` | Ciclar entre proyectos |
| `Enter` | Abrir el detalle de la tarea bajo el cursor |
| `i` | Insertar (proyecto en el Dashboard, tarea en List/Kanban) |
| `s` | Iniciar tarea (List) / cambiar estado (Kanban) |
| `d` / `x` | Marcar como `done` / cancelar |
| `e` / `E` | Editar / abrir en el editor externo |
| `/` | Filtros |
| `Ctrl+p` | Prioridad |
| `n`/`p`, `N`/`P` | Paginación (List) |
| `m` | Asignados y off-days (Dashboard) |
| `?` | Ayuda |
| `q` | Salir |

## Configuración

`~/.config/tsk/config.toml` (respeta `$XDG_CONFIG_HOME`; override con
`$TSK_CONFIG`). Todos los campos son opcionales:

```toml
[database]
path = ""                 # default: ~/.local/share/tsk/tsk.db (respeta $XDG_DATA_HOME)

[editor]
command = "nvim"          # editor externo (tecla E)

list_page_size        = 10   # tareas por página en la vista List
default_estimate_days = 1.0  # estimación de tareas sin estimate (Gantt)
gantt_weeks           = 6    # horizonte por defecto del Gantt
```

Un config malformado no rompe nada: se aplican los defaults.

## Desarrollo

`make check` es el equivalente local del gate de CI (jobs Build/Lint/Test):

```bash
make check   # golangci-lint + go vet + go test -race + go build ./...
make test    # go vet + go test -race -count=1 ./...
make lint    # golangci-lint v2.13.2 (pineado, vía go run)
make build   # compila e instala en ~/.local/bin/tsk
```

Arquitectura: `cmd/tsk` (entry point), `internal/cli` (subcomandos y salida
JSON), `internal/config` (TOML + paths XDG), `internal/db` (SQLite, WAL,
migraciones, CRUD), `internal/model` (Project, Task, workflow, Gantt) e
`internal/tui` (dashboard Bubbletea v2).

## Para agentes de IA

`AGENTS.md` en la raíz documenta stack, comandos, arquitectura, convenciones y
el flujo de trabajo (incluido el despliegue del binario tras cada cambio de
código). Léelo antes de tocar el repo.
