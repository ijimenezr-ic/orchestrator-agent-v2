# orchestrator-agent-v2

**Orquestador multi-agente** para GitHub Copilot (VS Code, Agent Mode) y OpenCode, con aislamiento por git worktrees y memoria persistente vía Engram.

## Arquitectura

```
┌──────────────────────────────────────────────────────┐
│         USUARIO en VS Code o OpenCode                │
│                    Chat prompt                        │
└───────────────────────┬──────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────┐
│        COPILOT CHAT (Agent Mode) / OPENCODE          │
│              Modelo: Claude Opus 4.6                  │
│                                                      │
│  Lee instrucciones de:                               │
│  ├── .github/copilot-instructions.md (VS Code)       │
│  ├── AGENTS.md (OpenCode)                            │
│  └── instructions/ (protocolo completo)              │
│                                                      │
│  ORQUESTADOR = Copilot/OpenCode guiado por markdown  │
└──┬────────────┬────────────────────────────────────-─┘
   │ MCP        │ MCP
   ▼            ▼
┌──────────┐  ┌────────────────────────────────────────┐
│  Engram  │  │       orchestrator-mcp (Go)            │
│  Memory  │  │  ├── worktree_create/list/merge/remove │
│  Server  │  │  ├── agent_spawn/status/list/cancel    │
└──────────┘  └───────────────┬────────────────────────┘
                              │ OS processes
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌──────────┐   ┌──────────┐   ┌──────────┐
        │ Subagente│   │ Subagente│   │ Subagente│
        │ React    │   │ Python   │   │ Tests    │
        │ worktree1│   │ worktree2│   │ worktree3│
        └──────────┘   └──────────┘   └──────────┘
              │               │               │
              └───────────────┴───────────────┘
                   Reportan via Engram HTTP API
```

## Prerequisitos

- **Go 1.22+** — para compilar el MCP server
- **Git 2.30+** — para soporte de `git worktree`
- **Engram** — servidor de memoria persistente ([instalar Engram](https://github.com/your-org/engram))
- **GitHub Copilot** (VS Code con Agent Mode) **o** **OpenCode**
- **OpenCode CLI** — para lanzar subagentes como procesos independientes

## Instalación

### 1. Compilar el MCP server

```bash
cd mcp-server
go build -o orchestrator-mcp ./cmd/orchestrator-mcp/
```

Esto genera el binario `mcp-server/orchestrator-mcp`.

### 2. Configurar VS Code (GitHub Copilot)

El archivo `.vscode/mcp.json` ya está configurado. Solo ajusta las rutas si es necesario.

Asegúrate de que Copilot está en **Agent Mode** (no Chat normal).

### 3. Configurar OpenCode

El archivo `opencode.json` en la raíz del proyecto ya está configurado. OpenCode lo detecta automáticamente.

### 4. Configurar variables de entorno (opcional)

Copia `config.example.json` como referencia. Las variables de entorno del MCP server son:

| Variable | Default | Descripción |
|----------|---------|-------------|
| `WORKTREE_BASE_DIR` | `../.worktrees` | Directorio base para worktrees de subagentes |
| `ENGRAM_URL` | `http://localhost:7437` | URL del servidor Engram |
| `SUBAGENT_MODEL` | `claude-sonnet-4-5-20250514` | Modelo para subagentes |
| `MAX_SUBAGENTS` | `0` (sin límite) | Máximo de subagentes concurrentes |
| `GITHUB_TOKEN` | — | Token para GitHub Models API (subagentes) |

### 5. Iniciar Engram

```bash
engram serve
```

Engram escucha en `http://localhost:7437` por defecto.

## Uso

### En VS Code (GitHub Copilot Agent Mode)

1. Abre VS Code en este repositorio
2. Activa **Agent Mode** en GitHub Copilot Chat
3. Habla con el orquestador:

```
"Implementa autenticación JWT. El backend es Python con FastAPI y 
el frontend es React + TypeScript."
```

El orquestador:
- Analiza la tarea y la descompone en subtareas
- Crea git worktrees para cada subagente
- Lanza subagentes en paralelo
- Monitorea el progreso
- Mergea los resultados
- Te informa del resultado final

### En OpenCode

```bash
opencode
```

Inicia OpenCode en el directorio del proyecto. El archivo `opencode.json` configura los MCP servers automáticamente. Pide la tarea igual que en VS Code.

### Observabilidad

Para ver qué hace cada subagente en tiempo real:

```bash
# Ver el TUI de Engram
engram tui

# O preguntar al orquestador:
"¿Cómo van los subagentes? ¿Cuál es el estado de cada uno?"
```

Los subagentes reportan en Engram con topic_keys:
- `agent/{task_id}/status` — Progreso actual
- `agent/{task_id}/result` — Resultado final

## Estructura del Proyecto

```
orchestrator-agent-v2/
├── .github/
│   └── copilot-instructions.md       # Instrucciones para VS Code Copilot Agent Mode
├── .vscode/
│   └── mcp.json                       # Config MCP servers para VS Code
├── instructions/
│   ├── orchestrator.md                # Protocolo completo del orquestador
│   ├── agents/
│   │   ├── frontend-react.md          # System prompt: subagente React
│   │   ├── backend-python.md          # System prompt: subagente Python hexagonal
│   │   ├── backend-node.md            # System prompt: subagente Node hexagonal
│   │   ├── test-agent.md              # System prompt: subagente tests
│   │   ├── docs-agent.md              # System prompt: subagente documentación
│   │   └── review-agent.md            # System prompt: subagente code review
│   └── protocols/
│       ├── task-decomposition.md      # Cómo descomponer tareas en DAG
│       ├── worktree-workflow.md        # Gestión de git worktrees
│       ├── memory-protocol.md         # Uso de Engram
│       └── merge-protocol.md          # Merge de ramas al finalizar
├── mcp-server/
│   ├── cmd/orchestrator-mcp/main.go   # Entrypoint del MCP server
│   ├── internal/
│   │   ├── worktree/manager.go        # Herramientas de git worktree
│   │   ├── spawner/spawner.go         # Lanzador de subagentes
│   │   └── status/tracker.go          # Tracker de estado vía Engram
│   ├── go.mod
│   └── go.sum
├── opencode.json                      # Config MCP servers para OpenCode
├── AGENTS.md                          # Instrucciones para OpenCode
├── config.example.json                # Ejemplo de configuración
└── README.md                          # Este archivo
```

## Tipos de Subagentes

| Tipo | Especialización | Modelo |
|------|----------------|--------|
| `frontend-react` | React 18+, TypeScript, hooks, TanStack Query, Tailwind | Claude Sonnet 4.6 |
| `backend-python` | Python, arquitectura hexagonal, FastAPI, SQLAlchemy | Claude Sonnet 4.6 |
| `backend-node` | Node.js/TypeScript, arquitectura hexagonal, NestJS, Prisma | Claude Sonnet 4.6 |
| `test-agent` | Jest, Vitest, pytest, Playwright, React Testing Library | Claude Sonnet 4.6 |
| `docs-agent` | README, docstrings, JSDoc, OpenAPI, diagramas ASCII | Claude Sonnet 4.6 |
| `review-agent` | Code review: CRITICAL / WARNING / SUGGESTION | Claude Sonnet 4.6 |

## Patrones de Memoria Engram

| Contexto | topic_key | Ejemplo |
|---------|-----------|---------|
| Estado de subagente | `agent/{task_id}/status` | `agent/2026-03-18-jwt-backend/status` |
| Resultado de subagente | `agent/{task_id}/result` | `agent/2026-03-18-jwt-backend/result` |
| Plan del orquestador | `orchestrator/plan/{session_id}` | `orchestrator/plan/2026-03-18-jwt` |
| Contexto de sesión | `orchestrator/session/{session_id}` | `orchestrator/session/2026-03-18-jwt` |

## Troubleshooting

### El MCP server no aparece en VS Code
- Verifica que el binario `mcp-server/orchestrator-mcp` existe y tiene permisos de ejecución
- Revisa la ruta en `.vscode/mcp.json`
- Reinicia VS Code

### Los subagentes no se lanzan
- Verifica que `opencode` está instalado y en el PATH
- Verifica que `GITHUB_TOKEN` tiene acceso a GitHub Models API
- Revisa los logs del MCP server en la consola de VS Code

### Engram no está disponible
- Asegúrate de que Engram está corriendo: `engram serve`
- Verifica que `ENGRAM_URL` apunta a la URL correcta
- Por defecto: `http://localhost:7437`

### Conflictos de merge
- El orquestador pausa el merge y te pide instrucciones
- Revisa el protocolo en `instructions/protocols/merge-protocol.md`

### Límite de subagentes
- Si tienes muchas tareas paralelas, ajusta `MAX_SUBAGENTS` en `.vscode/mcp.json`
- `0` = sin límite, `4` = máximo 4 subagentes concurrentes
