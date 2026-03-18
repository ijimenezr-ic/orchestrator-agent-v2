# Protocolo de Merge

## Objetivo

Este protocolo define cómo el orquestador debe ejecutar el merge de las ramas de los subagentes hacia la rama principal del repositorio, respetando el orden correcto y gestionando los conflictos de forma segura.

## Cuándo Ejecutar el Merge

El merge se ejecuta cuando:
1. **Todas las subtareas están en estado `completed`** — verificado via `agent_list()` o Engram
2. **El usuario solicita el merge explícitamente** — en flujos interactivos
3. **Fase de review completada** — si hay un `review-agent`, el merge solo ocurre si el veredicto es `APROBADO` o `APROBADO CON CONDICIONES` (con las condiciones resueltas)

## Preparación Previa al Merge

### 1. Verificar estado de todos los agentes

```
agent_list()
→ Verificar que todos los agentes tienen status: "completed"
→ Si alguno tiene status: "failed" o "running", NO ejecutar merge todavía
```

### 2. Leer resultados de todos los agentes

```
PARA CADA task_id en el plan:
  agent_result(task_id="{id}")
  → Verificar que el resultado indica éxito
  → Notar archivos creados/modificados para anticipar conflictos
```

### 3. Identificar el orden de merge

Basarse en el DAG construido en la fase de descomposición:
- Mergear primero las tareas base (sin dependencias)
- Mergear después las tareas que dependen de ellas
- Los tests y la documentación pueden ir al final o en paralelo (no tienen código que otros necesiten)

**Ejemplo de orden para JWT auth:**
```
1. T1: model (base)
2. T2: endpoints (depende de T1)
3. T3: middleware (depende de T2)
4. T4: frontend (depende de T2, independiente de T3)
5. T5: tests (depende de T2 y T3)
6. T6: docs (independiente)
```

## Ejecución del Merge

### Merge exitoso (caso normal)

```
PARA CADA tarea en orden topológico:

  1. Llamar worktree_merge:
     worktree_merge(
       source_branch="agent/{session_id}/{task_slug}",
       target_branch="main"
     )

  2. SI resultado == "success":
     - Actualizar estado de la tarea en el plan (Engram)
     - Continuar con la siguiente tarea
     - (Opcional) worktree_remove(task_id="{id}", force=false)

  3. SI resultado == "conflict":
     → Seguir el Protocolo de Resolución de Conflictos
```

### Merge con conflictos

Cuando el MCP server detecta conflictos en el merge, retorna:
```json
{
  "status": "conflict",
  "source_branch": "agent/2026-03-18-jwt/endpoints",
  "target_branch": "main",
  "conflicting_files": ["src/auth/service.ts", "src/users/types.ts"]
}
```

**El orquestador DEBE:**
1. Pausar el proceso de merge
2. Informar al usuario con detalle
3. Esperar instrucción antes de continuar

```
⚠️ Conflicto de merge detectado

Al intentar mergear agent/2026-03-18-jwt/endpoints → main:
Archivos en conflicto:
- src/auth/service.ts
- src/users/types.ts

Estos archivos fueron modificados por múltiples agentes simultáneamente.

¿Cómo quieres proceder?
A) Resolver manualmente: Abre los archivos con los markers <<<<<<< y resuelve
B) Priorizar los cambios del agente de endpoints (sobrescribir main)
C) Priorizar los cambios de main (descartar cambios del agente)
D) Revisar cambio por cambio juntos

Esperando tu decisión...
```

## Protocolo de Resolución de Conflictos

### Opción A: Resolución Manual

```
1. Informar al usuario:
   "El merge está pausado. Los archivos en conflicto están en el worktree 
   en {path}. Abre los archivos, busca los markers <<<<<<< y resuelve 
   los conflictos. Cuando termines, dime 'merge listo' y continuamos."

2. El usuario resuelve manualmente en su editor

3. Cuando el usuario confirma:
   - Hacer commit de la resolución
   - Continuar con el merge
```

### Opción B: Priorizar cambios del agente (ours)

```
El orquestador informa que tomará los cambios del agente:
"Aplicando los cambios del agente, descartando los conflictos de main."

Internamente: git checkout --theirs {archivo} → add → commit
```

### Opción C: Priorizar cambios de main (theirs)

```
El orquestador descarta los cambios del agente para los archivos en conflicto:
"Descartando los cambios del agente para los archivos en conflicto, 
manteniendo la versión actual de main."

Internamente: git checkout --ours {archivo} → add → commit
```

## Estrategias para Minimizar Conflictos

### En la fase de diseño (prevención)

1. **Asignar módulos completos**: En lugar de que dos agentes editen el mismo archivo, asignar módulos enteros a cada agente
   - T1 (backend-python): Todo el módulo `src/users/`
   - T2 (backend-python): Todo el módulo `src/auth/`
   - NO: ambos editan `src/api/routes.py`

2. **Interfaces compartidas primero**: Si dos agentes comparten una interface/tipo, crearla como primera subtarea y mergear antes de lanzar los demás

3. **Tests en worktree del código**: El agente de tests debería trabajar en un worktree creado DESPUÉS de mergear el código que va a testear

### Ejemplo de diseño anti-conflictos

```
❌ PROPENSO A CONFLICTOS:
   T1 modifica: src/main.py, src/models.py, src/routes.py
   T2 modifica: src/main.py, src/auth.py, src/routes.py
   → Conflicto seguro en src/main.py y src/routes.py

✅ DISEÑO SIN CONFLICTOS:
   T1 modifica: src/users/ (módulo completo)
   T2 modifica: src/auth/ (módulo completo)
   T3 modifica: src/main.py (solo registra los módulos de T1 y T2 — se hace DESPUÉS de T1 y T2)
```

## Post-Merge: Verificación

Después de todos los merges, ejecutar verificación básica:

### Para proyectos Node.js/TypeScript
```
1. Verificar que el proyecto compila:
   npm run build → ¿sale 0?

2. Verificar que los tests pasan:
   npm test → ¿salen todos en verde?
```

### Para proyectos Python
```
1. Verificar que no hay errores de importación:
   python -c "from src.main import app" → ¿sin error?

2. Verificar tests:
   pytest → ¿todos pasan?
```

Si la verificación falla, es señal de que algún merge introdujo un problema. Reportar al usuario con el error.

## Limpieza Final

Una vez todo está mergeado y verificado:

```
worktree_cleanup()
→ Elimina todos los worktrees creados durante la sesión
→ Ejecuta git worktree prune para limpiar referencias huérfanas
```

**Qué hace worktree_cleanup:**
- Lista todos los worktrees creados por el orquestador (en el directorio WORKTREE_BASE_DIR)
- Para cada uno: `git worktree remove --force {path}`
- Ejecuta: `git worktree prune`
- Las **ramas** (branches) se conservan — solo se eliminan los directorios de trabajo

## Reporte al Usuario

Al completar el merge:

```
## ✅ Merge Completado

**Sesión**: 2026-03-18-jwt
**Ramas mergeadas**: 6
**Rama destino**: main

### Ramas mergeadas:
✅ agent/2026-03-18-jwt/model → main (sin conflictos)
✅ agent/2026-03-18-jwt/endpoints → main (sin conflictos)
✅ agent/2026-03-18-jwt/middleware → main (sin conflictos)
✅ agent/2026-03-18-jwt/frontend → main (sin conflictos)
✅ agent/2026-03-18-jwt/tests → main (sin conflictos)
✅ agent/2026-03-18-jwt/docs → main (sin conflictos)

### Archivos modificados en main:
- src/domain/entities/user.py (nuevo)
- src/api/routers/auth.py (nuevo)
- src/api/main.py (modificado)
- [... N archivos más]

### Verificación:
✅ Build exitoso
✅ 23/23 tests pasan

### Limpieza:
✅ Worktrees eliminados
```

## Gestión de Errores

| Situación | Acción |
|-----------|--------|
| Merge falla por conflicto | Pausar, informar usuario, esperar instrucción |
| Build falla post-merge | Informar qué falló, sugerir revisar el último merge |
| Tests fallan post-merge | Informar qué tests fallan, posiblemente lanzar `test-agent` para diagnosticar |
| Worktree ya no existe | Continuar con el siguiente (el worktree pudo haber sido limpiado manualmente) |
| Rama del agente ya está mergeada | Skip, continuar con siguiente |
