# Protocolo de Descomposición de Tareas

## Objetivo

Este protocolo define cómo el orquestador debe analizar una tarea de usuario de alto nivel y descomponerla en subtareas manejables para subagentes especializados.

## Paso 1: Análisis Inicial de la Tarea

### 1.1 Clasificación de la tarea

Al recibir una tarea, clasifícala en una o más categorías:

| Categoría | Indicadores | Agente(s) implicado(s) |
|-----------|------------|----------------------|
| **Frontend UI** | "interfaz", "componente", "pantalla", "formulario", "vista" | `frontend-react` |
| **Backend API** | "endpoint", "API", "servicio", "microservicio" | `backend-python` o `backend-node` |
| **Base de datos** | "modelo", "schema", "migración", "tabla", "colección" | Depende del stack |
| **Autenticación** | "login", "auth", "JWT", "OAuth", "permisos" | Múltiples agentes |
| **Testing** | "tests", "pruebas", "cobertura", "QA" | `test-agent` |
| **Documentación** | "docs", "README", "docstrings", "API reference" | `docs-agent` |
| **Full-stack** | Combinación de los anteriores | Múltiples agentes |

### 1.2 Identificación del Stack Tecnológico

Determina el stack a partir de:
- Archivos de configuración del repositorio (`package.json`, `pyproject.toml`, `go.mod`)
- Instrucciones explícitas del usuario
- Código existente en el repositorio

```
SI existe package.json con "react" → frontend es React
SI existe package.json sin "react" → backend es Node.js
SI existe pyproject.toml o requirements.txt → backend es Python
SI existe ambos → full-stack, determinar qué tipo de backend
```

## Paso 2: Construcción del DAG

### 2.1 Identificar Subtareas Atómicas

Una subtarea atómica es aquella que:
- Puede ser ejecutada por un solo tipo de subagente
- Tiene un resultado claro y verificable
- No depende de código que aún no existe
- Puede completarse en una sola sesión del subagente

**Ejemplos de descomposición**:

Tarea usuario: *"Implementar sistema de autenticación con JWT"*

```
T1: Modelo de datos de usuario (backend-python)
    → Crea entidad User con campos: id, email, password_hash, role

T2: Endpoints de auth (backend-python) [DEPENDE: T1]
    → POST /auth/register
    → POST /auth/login (retorna JWT)
    → GET /auth/me (requiere JWT)

T3: Middleware de autenticación (backend-python) [DEPENDE: T2]
    → Valida JWT en headers
    → Inyecta user en request

T4: Formulario de login (frontend-react) [DEPENDE: T2 para saber la API]
    → LoginForm component
    → Hook useAuth
    → Manejo de token en localStorage

T5: Tests de endpoints auth (test-agent) [DEPENDE: T2, T3]
    → Tests integración de register/login
    → Tests con token válido/inválido

T6: Documentación (docs-agent) [DEPENDE: T1, T2, T3]
    → README section sobre auth
    → Docstrings en endpoints

T7: Code review (review-agent) [DEPENDE: T1, T2, T3, T4]
    → Revisar seguridad del JWT
    → Revisar validaciones
```

### 2.2 Identificar Dependencias

Para cada subtarea, pregunta:
- **¿Esta tarea necesita código que otra tarea crea?** → La primera depende de la segunda
- **¿Esta tarea puede empezar con la API spec sin la implementación?** → Puede ser paralela
- **¿El código de esta tarea será revisado por otro agente?** → El review va después

### 2.3 Construir el DAG

Representa el DAG visualmente:

```
T1 (modelo) ─────────────────────────────────────────────────┐
                                                               ▼
T2 (endpoints) ─[depende T1]──────────────────────────────→ T7 (review)
     │
     ├──────────────────────────────────────────────────────→ T4 (frontend)
     │                                                         │
     └──────────────────────────────────────────────────────→ T5 (tests)
                                                               │
T3 (middleware) ─[depende T2]────────────────────────────────-┘
     │
     └──────────────────────────────────────────────────────→ T6 (docs)
```

**Grupos de ejecución paralela**:
- **Grupo 1 (paralelo)**: T1
- **Grupo 2 (paralelo)**: T2 (si T1 completo)
- **Grupo 3 (paralelo)**: T3, T4 (si T2 completo)
- **Grupo 4 (paralelo)**: T5, T6 (si T3 completo)
- **Grupo 5**: T7 (si T4 completo)

## Paso 3: Estimación de Complejidad

Para cada subtarea, estima:

| Complejidad | Descripción | Tiempo estimado |
|-------------|-------------|-----------------|
| **Simple** | 1-3 archivos, lógica directa | 5-10 min |
| **Media** | 3-8 archivos, algo de lógica | 10-20 min |
| **Alta** | >8 archivos, lógica compleja | 20-40 min |

Si una subtarea es de complejidad Alta, considera si puede dividirse más.

## Paso 4: Asignación de Agentes

### Reglas de asignación

```
PARA CADA subtarea:

  SI categoría == "frontend-ui":
    agente = "frontend-react"

  SI categoría == "backend-api":
    SI stack == "python":
      agente = "backend-python"
    SINO SI stack == "node" O stack == "typescript":
      agente = "backend-node"
    SINO:
      pregunta al usuario el stack

  SI categoría == "testing":
    agente = "test-agent"

  SI categoría == "documentación":
    agente = "docs-agent"

  SI categoría == "revisión":
    agente = "review-agent"

  SI categoría == "full-stack":
    dividir en subtareas frontend + backend (ver Paso 2)
```

## Paso 5: Generación de Descripciones de Tareas

Cada descripción de tarea que pasas al subagente debe ser **autosuficiente**. El subagente no tiene acceso al contexto de tu conversación con el usuario. Incluye:

```
### Contexto del Proyecto
{Breve descripción del proyecto: qué hace, qué stack usa, estructura de directorios relevante}

### Tu Tarea
{Descripción precisa de qué debe implementar}

### Requisitos Técnicos
- {Requisito específico 1}
- {Requisito específico 2}

### Archivos a Crear/Modificar
- `path/al/archivo.ts`: {qué debe contener}
- `path/al/otro.ts`: {qué debe contener}

### Contexto de Tareas Previas
{Si hay código que otro agente ya creó y que este agente usa, descríbelo aquí}

### Criterios de Aceptación
- [ ] {Criterio verificable 1}
- [ ] {Criterio verificable 2}

### Variables de Entorno
- ENGRAM_URL: {url de engram}
- TASK_ID: {tu_task_id}

### Cómo Reportar Resultado
Al terminar, haz POST a {ENGRAM_URL}/observations con:
- topic_key: "agent/{TASK_ID}/result"
- content: JSON con status, summary, files_created, files_modified
```

## Paso 6: Validación del Plan

Antes de ejecutar, verifica:

- [ ] Todas las subtareas tienen un agente asignado
- [ ] Las dependencias están bien identificadas (no hay ciclos)
- [ ] Las descripciones son suficientemente detalladas
- [ ] El orden de ejecución es correcto
- [ ] El número de subtareas es razonable (< 10 por sesión recomendado)

Si hay > 10 subtareas, considera:
- ¿Se puede hacer en fases? (Fase 1: backend, Fase 2: frontend, Fase 3: tests)
- ¿Algunas subtareas se pueden combinar?

## Antipatrones a Evitar

| Antipatrón | Problema | Solución |
|-----------|---------|---------|
| Subtareas demasiado grandes | El agente no puede completarlas en una sesión | Dividir más |
| Dependencias circulares | Deadlock — ningún agente puede empezar | Revisar el DAG |
| Descripción vaga | El agente no sabe qué hacer exactamente | Ser específico con archivos y criterios |
| Demasiados agentes paralelos | Saturar el sistema | Respetar MAX_SUBAGENTS |
| Mezclar tipos de tareas en una subtarea | Confusión de responsabilidades | Un agente, una especialidad |
