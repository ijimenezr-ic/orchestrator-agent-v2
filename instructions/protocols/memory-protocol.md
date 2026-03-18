# Protocolo de Uso de Engram (Memoria Persistente)

## Qué es Engram

Engram es el sistema de memoria persistente del orquestador multi-agente. Permite:
- Al orquestador **guardar su plan** sin saturar el contexto conversacional
- A los subagentes **reportar progreso y resultados** de forma estructurada
- Al orquestador **leer resultados compactos** sin procesar archivos completos
- **Persistir conocimiento** entre sesiones de trabajo

## API HTTP de Engram

El orquestador y los subagentes interactúan con Engram via HTTP REST.

### Base URL
`{ENGRAM_URL}` — Por defecto: `http://localhost:7437`

### Endpoints

#### POST /sessions
Inicia una sesión de memoria.
```json
// Request
{
  "id": "2026-03-18-jwt-auth",
  "project": "mi-proyecto",
  "directory": "/path/to/project"
}
// Response: { "id": "...", "started_at": "..." }
```

#### POST /sessions/{id}/end
Finaliza una sesión.
```json
// Request
{ "summary": "Implementado sistema JWT. 5 archivos creados, todos los tests pasan." }
```

#### POST /observations
Guarda una observación/memoria. **El endpoint más usado**.
```json
// Request
{
  "session_id": "2026-03-18-jwt-auth",
  "type": "observation",         // observation | decision | error | result
  "title": "Título descriptivo",
  "content": "Contenido detallado (puede ser JSON string)",
  "tool_name": "nombre-herramienta",   // opcional
  "project": "mi-proyecto",            // opcional
  "scope": "local",                    // opcional: local | global
  "topic_key": "agent/task-1/status"   // CRÍTICO: organiza las memorias por tema
}
```

#### GET /search
Busca en la memoria.
```
GET /search?q=QUERY&type=TYPE&project=PROJECT&limit=N
```
- `q`: término de búsqueda (texto libre o topic_key)
- `type`: filtrar por tipo (`observation`, `decision`, etc.)
- `project`: filtrar por proyecto
- `limit`: máximo de resultados (default: 10)

#### GET /context
Obtiene el contexto más relevante para el proyecto actual.
```
GET /context?project=MI_PROYECTO
```

#### GET /stats
Estadísticas de la memoria.

## Patrones de topic_key

El `topic_key` es el campo más importante para organizar la memoria. Sigue estos patrones:

| Contexto | topic_key pattern | Ejemplo |
|---------|------------------|---------|
| Estado de subagente | `agent/{task_id}/status` | `agent/2026-03-18-jwt-backend/status` |
| Resultado de subagente | `agent/{task_id}/result` | `agent/2026-03-18-jwt-backend/result` |
| Informe de review | `agent/{task_id}/review-report` | `agent/2026-03-18-jwt-review/review-report` |
| Plan del orquestador | `orchestrator/plan/{session_id}` | `orchestrator/plan/2026-03-18-jwt` |
| Contexto de sesión | `orchestrator/session/{session_id}` | `orchestrator/session/2026-03-18-jwt` |
| Decisión técnica | `orchestrator/decision/{session_id}` | `orchestrator/decision/2026-03-18-jwt` |

## Protocolo del Orquestador

### Al iniciar una nueva tarea

```
1. Buscar contexto previo:
   GET /search?q=orchestrator/session&project={nombre_proyecto}&limit=5

2. Iniciar sesión:
   POST /sessions
   { "id": "{session_id}", "project": "{nombre_proyecto}", "directory": "{repo_path}" }

3. Guardar el plan:
   POST /observations
   {
     "session_id": "{session_id}",
     "type": "observation",
     "title": "Plan de ejecución: {tarea_breve}",
     "content": "{DAG en JSON: [{id, tipo, descripcion, dependencias, estado}]}",
     "topic_key": "orchestrator/plan/{session_id}"
   }
```

### Guardar el plan como JSON

```json
// Contenido del topic_key: orchestrator/plan/{session_id}
{
  "tarea_original": "Implementar autenticación JWT",
  "session_id": "2026-03-18-jwt",
  "created_at": "2026-03-18T10:00:00Z",
  "tasks": [
    {
      "id": "T1",
      "slug": "model",
      "task_id": "2026-03-18-jwt-model",
      "agent_type": "backend-python",
      "description": "Crear entidad User con campos auth",
      "dependencies": [],
      "status": "completed",
      "pid": 12345,
      "worktree_path": "../.worktrees/2026-03-18-jwt-model"
    },
    {
      "id": "T2",
      "slug": "endpoints",
      "task_id": "2026-03-18-jwt-endpoints",
      "agent_type": "backend-python",
      "description": "Endpoints POST /auth/register y POST /auth/login",
      "dependencies": ["T1"],
      "status": "in_progress",
      "pid": 12346,
      "worktree_path": "../.worktrees/2026-03-18-jwt-endpoints"
    }
  ]
}
```

### Actualizar el plan tras completar una tarea

```
POST /observations
{
  "session_id": "{session_id}",
  "type": "observation",
  "title": "Plan actualizado: T1 completada",
  "content": "{plan JSON actualizado con status: completed}",
  "topic_key": "orchestrator/plan/{session_id}"
}
```

### Consultar el estado de los agentes

```
# Estado de un agente específico:
GET /search?q=agent/{task_id}/status&limit=1

# Resultado de un agente completado:
GET /search?q=agent/{task_id}/result&limit=1

# Todos los estados de la sesión:
GET /search?q=agent/2026-03-18-jwt&type=observation&limit=20
```

### Al finalizar la sesión

```
1. Guardar decisiones técnicas importantes:
   POST /observations
   {
     "session_id": "{session_id}",
     "type": "decision",
     "title": "Decisiones técnicas de {tarea}",
     "content": "Usamos JWT con HS256. Token expira en 24h. Refresh tokens no implementados.",
     "topic_key": "orchestrator/decision/{session_id}"
   }

2. Cerrar la sesión:
   POST /sessions/{session_id}/end
   { "summary": "Tarea completada. Implementadas 6 subtareas. 23 tests pasan." }
```

## Protocolo de los Subagentes

Los subagentes reportan vía HTTP directo (sin MCP) a Engram. Usan las variables de entorno `ENGRAM_URL` y `TASK_ID`.

### Al iniciar la tarea

```bash
# Python
import httpx, os

engram_url = os.getenv("ENGRAM_URL", "http://localhost:7437")
task_id = os.getenv("TASK_ID")

httpx.post(f"{engram_url}/observations", json={
    "session_id": "session-default",
    "type": "observation",
    "title": f"Iniciando tarea: {task_id}",
    "content": "Analizando repositorio y planificando implementación",
    "topic_key": f"agent/{task_id}/status"
})
```

### Durante el trabajo (actualizaciones de progreso)

```bash
# Actualización intermedia
httpx.post(f"{engram_url}/observations", json={
    "session_id": "session-default",
    "type": "observation",
    "title": f"Progreso: {task_id}",
    "content": "Entidad User creada. Implementando repositorio SQLAlchemy...",
    "topic_key": f"agent/{task_id}/status"
})
```

### Al completar la tarea

```bash
resultado = {
    "status": "completed",
    "task_id": task_id,
    "agent_type": "backend-python",
    "summary": "Entidad User implementada con campos: id, email, password_hash, role, created_at",
    "files_created": ["src/domain/entities/user.py", "src/infrastructure/database/user_repository.py"],
    "files_modified": [],
    "files_deleted": [],
    "decisions": ["Usé bcrypt para hash de contraseñas", "UUID v4 para IDs"],
    "warnings": [],
    "tests_written": ["tests/unit/test_user_entity.py"]
}

httpx.post(f"{engram_url}/observations", json={
    "session_id": "session-default",
    "type": "observation",
    "title": f"Completado: {task_id}",
    "content": json.dumps(resultado),
    "topic_key": f"agent/{task_id}/result"
})
```

### En caso de error

```bash
httpx.post(f"{engram_url}/observations", json={
    "session_id": "session-default",
    "type": "error",
    "title": f"Error en tarea: {task_id}",
    "content": f"Error: {str(exception)}. Traceback: {traceback_str}",
    "topic_key": f"agent/{task_id}/status"
})
```

## Uso de Engram para Evitar Saturar el Contexto

**El principio central**: el orquestador NUNCA debe leer archivos de código directamente. Solo lee resúmenes de Engram.

```
❌ INCORRECTO:
   cat src/auth/jwt.service.ts | read_file → saturar contexto

✅ CORRECTO:
   GET /search?q=agent/{task_id}/result → leer resumen compacto
```

El resultado de cada subagente (unos pocos kilobytes de JSON) es todo lo que el orquestador necesita para:
- Saber si la tarea se completó exitosamente
- Conocer qué archivos fueron creados/modificados
- Entender las decisiones técnicas tomadas
- Pasar el contexto relevante a los siguientes agentes

## Búsqueda Efectiva

```
# Buscar por topic_key exacto
GET /search?q=agent/2026-03-18-jwt-backend/result

# Buscar por proyecto y tipo
GET /search?q=jwt&project=mi-proyecto&type=observation&limit=10

# Contexto relevante del proyecto actual
GET /context?project=mi-proyecto

# Historial de sesiones anteriores
GET /search?q=orchestrator/session&project=mi-proyecto&limit=5
```
