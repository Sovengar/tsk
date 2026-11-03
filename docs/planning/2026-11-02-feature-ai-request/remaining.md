# Pendiente — handoff AI, mux y `github.com/sovengar/ai`

Task: #92
Estado: piloto tsk funcionando, integración mux verificada, módulo sin publicar.

## Requisito nuevo: handoff único

El `config.toml` de tsk **no debe tener un `command` por harness**. Eso repite en la
configuración del usuario el conocimiento de argv de cada agente (`pi -p`, `opencode
--prompt`, `codex exec`, etc.). Es exactamente lo que `ai` debe resolver.

Contrato objetivo:

```
entrada:  harness + model + effort + prompt
salida:   proceso del agente ejecutándose
```

- `ai` (binario) traduce la entrada a argv correcto.
- `mux` solo presenta: pane, tab, popup u overlay. No sabe de agentes.
- tsk solo expande placeholders y lanza **un** comando.
- El usuario no escribe flags por agente.
- Un agente desconocido puede aportar `binary`, pero no un `command` completo.

Config objetivo, una sola vez:

```toml
[handoff]
command = "mux --container {{container}} --layout tab --working-dir {{working_dir}} --session-id {{session_id}} -- ai exec --harness {{harness}} --model {{model}} --effort {{effort}} --prompt-file {{prompt_file}}"
```

No hay `[[harness]]` en tsk. Catálogo, binarios, defaults y overrides viven en
`ai`, no en la config de cada app. tsk solo necesita la lista para el picker; la
pide al binario `ai` o a `ai catalog`. Si un agente es custom, se registra en `ai`.
No hay `--label`: mux decide la etiqueta. Recibe `--session-id {{session_id}}` y,
solo si el layout es `tab`/`popup`, usa `session_id` tal cual (o reutiliza
`--label` si el usuario lo pasa). En `pane` no hay etiqueta. tsk genera el
`session_id` como `tsk-<project>-<task_number>` (ej. `tsk-projects-92`); no se usa `$$` de
shell. Si un task puede tener varios handoffs simultáneos, añadir un sufijo de
lanzamiento para evitar etiquetas repetidas.

### `ai exec`

- Recibe `--harness`, `--model`, `--effort`, `--prompt-file`.
- Resuelve capacidades y construye argv.
- Omite `--model`/`--effort` si están vacíos.
- Omite effort si el agente no lo soporta; nunca inventa flags.
- Lee el prompt y lo entrega como argv o stdin según el agente.
- Usa `exec` para conservar TTY, señales y exit code.
- No pasa el prompt por shell sin citar.
- `--dry-run` imprime argv; sirve para tests.

### Por qué `--prompt-file` y no `--prompt`

- El prompt es multilínea y puede contener comillas, `$`, backticks o saltos.
- Un archivo evita quoting de shell y límites de longitud de argv.
- tsk ya crea un archivo 0600 y pasa la ruta.
- `ai` lee los bytes exactos; no interpreta el prompt.
- `--prompt` literal puede existir para tests o prompts cortos, pero el handoff
  usa `--prompt-file`.

### Por qué un binario `ai` y no solo el módulo

- Las apps Go importan `github.com/sovengar/ai`.
- La config de tsk necesita un ejecutable: el shell no puede importar Go.
- `ai` es un wrapper fino del módulo: catálogo, modelos y `exec`.
- `mux` sigue siendo solo presentación.

Pendiente de decidir: formato de override de binario, y si `ai` se instala con
`make install` o con `go install`.

## Los 6 puntos

### 1. Cerrar piloto tsk y aplicar handoff único

- Quitar los tres `[harness.handoff].command` y los `[[harness]]` de `~/.config/tsk/config.toml`.
- Implementar el contrato de arriba en tsk + `ai`.
- Mantener e2e: `tsk ask` crea **pane**, no tab.
- Commitear cambios de tsk: `go.mod`, `internal/harness`, TUI, tests, plan.
- Actualizar task #92.

### 2. Publicar `github.com/sovengar/ai`

- `/home/buble/dev/projects/ai` no es repo git.
- Crear repo, remoto `sovengar/ai`, tag `v0.1.0`.
- Quitar `replace github.com/sovengar/ai => ../ai` de tsk.
- Verificar build limpio con require versionado.

### 3. Adoptar el módulo en las apps

- `vroom`: modo `run` con `ProcessInvoker`.
- `gitdash`: overlay de conflicto, run en background, diff y confirmar.
- `dbx`: `ModelSource` `/models` + selector modelo/effort.
- `prdash`: modo attach.
- `wsdash`, `review-tui`, `cdx`: sin handoff hoy; decidir si entran.

### 4. Gaps del core

- Invoker Anthropic Messages; hoy solo OpenAI-compatible.
- `ModelSource` como interfaz; hoy es `OptionsFor`.
- Cobertura de agentes: `pi`, `codex`, `opencode`, `hermes`, `jcode`.
- Tests de `ai exec` para cada formato de argv.

### 5. Documentación

- ADR/README de cada app.
- Ejemplos de config para vroom, gitdash, dbx y prdash.
- Documentar el handoff único como contrato, no como ejemplo de tsk.
- Herdr popup real: solo config (`[[keys.command]] type = "popup"`), no plugin.

### 6. Git y operación

- Push del commit chezmoi `c8002a1` (rama `feat/bundles`).
- Resolver duplicado de `mux`:
  - `ai/scripts/mux`
  - chezmoi `home/dot_local/bin/executable_mux`
- Elegir fuente única o añadir check de sincronización.
- Instalar `ai` en PATH junto a `mux`.
- Cerrar task #92 solo cuando las apps dejen de tener lógica de argv propia.

## Criterio de terminado

- Un solo `command` de handoff en tsk, sin `[[harness]]`.
- Añadir un agente nuevo no toca la config de tsk.
- `mux` no conoce agentes.
- `tsk`, `vroom`, `gitdash`, `dbx` y `prdash` consumen `ai`.
- `ai` está publicado y tsk no usa `replace`.
