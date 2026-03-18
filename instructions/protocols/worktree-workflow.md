# Protocolo de Uso de Git Worktrees

## Objetivo

Este protocolo define cómo el orquestador debe gestionar los git worktrees para aislar el trabajo de cada subagente y permitir desarrollo paralelo sin conflictos.

## Concepto de Git Worktrees

Un **git worktree** es una copia de trabajo separada del repositorio vinculada a la misma base de datos de objetos Git. Permite tener múltiples ramas activas simultáneamente en directorios diferentes.

```
repo-principal/          # Worktree principal (branch: main)
../.worktrees/
├── session-xyz-t1/      # Worktree del subagente T1 (branch: agent/session-xyz/t1)
├── session-xyz-t2/      # Worktree del subagente T2 (branch: agent/session-xyz/t2)
└── session-xyz-t3/      # Worktree del subagente T3 (branch: agent/session-xyz/t3)
```

## Nomenclatura

### IDs y nombres de branch

**Task ID**: `{session_id}-{tarea_slug}`
- Ejemplo: `2026-03-18-jwt-auth-backend`

**Branch name**: `agent/{session_id}/{tarea_slug}`
- Ejemplo: `agent/2026-03-18-jwt/auth-backend`

**Worktree path**: `{WORKTREE_BASE_DIR}/{task_id}`
- Ejemplo: `../.worktrees/2026-03-18-jwt-auth-backend`

### Sesión ID

Formato: `{YYYY-MM-DD}-{slug}` donde slug es una descripción corta de la tarea general:
- `2026-03-18-jwt-auth`
- `2026-03-18-user-crud`
- `2026-03-18-dashboard-ui`

## Flujo Completo de Worktree

### Fase 1: Creación (antes de lanzar el subagente)

```
1. Identificar el punto de partida del worktree:
   - Normalmente: rama main o develop
   - A veces: rama de otro agente si este depende de él

2. Llamar worktree_create:
   worktree_create(
     task_id="{session_id}-{task_slug}",
     branch_name="agent/{session_id}/{task_slug}"
   )

3. El tool retorna el path del worktree creado.
   Guardar este path para pasárselo al agente.

4. Pasar el worktree_path al agente en agent_spawn:
   agent_spawn(
     task_id=...,
     agent_type=...,
     task_description=...,
     worktree_path="{path retornado por worktree_create}"
   )
```

### Fase 2: Trabajo del Subagente

El subagente trabaja **exclusivamente** dentro de su worktree:
- Crea y modifica archivos en el directorio del worktree
- Hace git commits en su rama
- No toca otros worktrees ni la rama main

### Fase 3: Verificación

Antes de mergear, verifica opcionalmente:
```
worktree_list()  → Ver todos los worktrees activos

Para cada worktree completado:
  - Ver los commits: git log agent/{session}/{task} --oneline
  - Ver los archivos cambiados: git diff main...agent/{session}/{task} --name-only
```

### Fase 4: Merge

Una vez completadas las subtareas, mergea en el orden correcto respecto al DAG:

```
PARA CADA tarea en orden topológico:
  1. worktree_merge(
       source_branch="agent/{session_id}/{task_slug}",
       target_branch="main"
     )

  2. SI merge exitoso:
     - Continuar con la siguiente tarea
     - worktree_remove(task_id="{session_id}-{task_slug}", force=false)

  3. SI hay conflictos:
     - Reportar al usuario: "Conflicto en merge de {branch}. Archivos: {lista}"
     - NO forzar el merge
     - Esperar instrucción del usuario
```

**Orden de merge recomendado**:
- Mergear primero las tareas base (modelo, entidades)
- Luego las tareas que dependen de ellas (endpoints, servicios)
- Tests y documentación pueden ir al final

### Fase 5: Limpieza

Una vez completado todo el proceso:
```
worktree_cleanup()  → Elimina todos los worktrees del orquestador
```

O de forma selectiva:
```
worktree_remove(task_id="{id}", force=false)  → Elimina un worktree específico
```

## Gestión de Conflictos de Merge

### Tipos de conflictos

| Tipo | Causa | Solución |
|------|-------|---------|
| **Edición concurrente** | Dos agentes editaron el mismo archivo/línea | Revisión manual |
| **Rename conflicto** | Un agente renombró un archivo que otro editó | Revisión manual |
| **Delete conflicto** | Un agente eliminó un archivo que otro editó | Decidir si eliminar o conservar |

### Protocolo de conflicto

```
1. El merge falla por conflictos
2. Reportar al usuario:
   "⚠️ Conflicto de merge detectado
    Rama: agent/{session}/{task} → main
    Archivos en conflicto:
    - src/users/service.ts (edición concurrente)
    - src/auth/middleware.ts (edición concurrente)

    Opciones:
    A) Resolver manualmente (te guío paso a paso)
    B) Aplicar los cambios de {task} sobre los de {otra_task} (merge ours)
    C) Descartar los cambios de {task} (merge theirs)"

3. Esperar decisión del usuario antes de continuar
```

### Prevención de conflictos

El orquestador debe minimizar conflictos en la fase de diseño:
- **Archivos distintos por agente**: Cada agente debería tocar archivos diferentes
- **Interfaces antes de implementación**: Si dos agentes comparten una interface, crearla primero
- **Módulos aislados**: Asignar módulos completos a agentes (ej: todo el módulo `users/` a un agente)

## Comandos Git de Referencia

Los siguientes comandos son ejecutados internamente por el MCP server:

```bash
# Crear worktree
git worktree add -b {branch_name} {worktree_path} {base_branch}

# Listar worktrees
git worktree list --porcelain

# Merge
git checkout {target_branch}
git merge --no-ff {source_branch} -m "Merge agent task {task_id}"

# Eliminar worktree
git worktree remove {worktree_path} [--force]

# Limpiar referencias huérfanas
git worktree prune
```

## Casos Especiales

### Subagente que depende del código de otro

Si T3 depende del código de T1 (no solo de T1 estar completo, sino de su código):

```
1. Esperar a que T1 complete
2. Mergear T1 a main ANTES de crear el worktree de T3
3. Crear worktree de T3 desde main (que ya tiene el código de T1)
4. Lanzar T3
```

### Cancelación de un subagente

Si un subagente debe ser cancelado y su worktree descartado:

```
1. agent_cancel(task_id="{id}")
2. worktree_remove(task_id="{id}", force=true)  # force=true para ignorar cambios no commiteados
3. El código de ese agente se descarta completamente
```

### Relanzar un subagente fallido

Si un agente falla y necesita relanzarse:

```
1. Evaluar si el worktree existente es reutilizable
2. Opción A: Reutilizar el mismo worktree (el agente continúa donde lo dejó)
3. Opción B: worktree_remove + worktree_create (empezar de cero)
4. agent_spawn con la misma configuración (o con instrucciones mejoradas)
```

## Ejemplo Completo

Para la tarea "Implementar sistema de autenticación JWT":

```
Session ID: 2026-03-18-jwt

Worktrees a crear:
├── 2026-03-18-jwt-model       → branch: agent/2026-03-18-jwt/model
├── 2026-03-18-jwt-endpoints   → branch: agent/2026-03-18-jwt/endpoints
├── 2026-03-18-jwt-middleware  → branch: agent/2026-03-18-jwt/middleware
├── 2026-03-18-jwt-frontend    → branch: agent/2026-03-18-jwt/frontend
├── 2026-03-18-jwt-tests       → branch: agent/2026-03-18-jwt/tests
└── 2026-03-18-jwt-docs        → branch: agent/2026-03-18-jwt/docs

Orden de merge:
1. agent/2026-03-18-jwt/model       → main
2. agent/2026-03-18-jwt/endpoints   → main
3. agent/2026-03-18-jwt/middleware  → main
4. agent/2026-03-18-jwt/frontend    → main
5. agent/2026-03-18-jwt/tests       → main
6. agent/2026-03-18-jwt/docs        → main

Cleanup:
worktree_cleanup()
```
