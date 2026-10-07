# pf-svc-core

Raíz de composición del deployable **`core`** de AI27 (ADR 17, ADR 29, DEC-050 §2).

`core` compone en un solo proceso los módulos de backend que viven juntos en V1
—`bff`, `identidad`, `operacion`, `catalogos`, `reportes`— cableándolos con un
pool por rol, los adaptadores en proceso de sus aristas co-ubicadas, un solo
servidor HTTP y los workers que cada módulo declara. Es el **único** repo
autorizado a importar otros `pf-svc-*` (00-indice regla 1).

No contiene lógica de negocio: solo configuración, pools, adaptadores, arranque
de servidor y de workers. Si aparece lógica de dominio aquí, pertenece a
`pf-svc-<x>/internal/`.

## Módulos cableados hoy

| Módulo | Estado | Nota |
|---|---|---|
| identidad | ✅ cableado (`v0.1.1`) | hoja del grafo; un pool `svc_identidad`; destino de las aristas de revocación y de sesión; relay de outbox en `Workers()` |
| catalogos | ✅ cableado (`v0.1.1`) | hoja del grafo; un pool `svc_catalogos`; arista restringida `catalogos → identidad` (revocación) en proceso |
| operacion | ✅ cableado (`v0.1.3`) | cinco pools (`op_viajes/asignacion/bitacora/auditoria/roster`, ADR 33); aristas `operacion → catalogos` y la restringida `operacion → identidad` en proceso |
| reportes | ✅ cableado (`v0.1.1`) | un pool `svc_reportes` (ADR 18); aristas `reportes → catalogos` y la restringida `reportes → identidad` en proceso |
| bff | ✅ cableado (`v0.1.0`) | sin pool (no es dueño de datos); aristas `bff → operacion` y `bff → identidad` en proceso (fase a) |

`core` compone los **cinco** módulos de V1: el composition root está completo
(arch-test exige exactamente `[bff, identidad, operacion, catalogos, reportes]`).
Las aristas síncronas co-ubicadas se resuelven con adaptadores **en proceso** de
`internal/adapters/`, cada uno respetando el presupuesto de tiempo que su
consumidor exporta (`operacion.PresupuestoAptitud`, `reportes.PresupuestoCatalogos`,
`operacion/catalogos/reportes.PresupuestoRevocacion`, `bff.PresupuestoOperacion`,
`bff.PresupuestoIdentidad`), la misma taxonomía de error y la cancelación que su
adaptador HTTP (I3). Los adaptadores HTTP de esos puertos **no** se cablean aquí
porque los destinos están co-ubicados.

Adaptadores en proceso cableados:

| Arista | Adaptador | Puerto del consumidor |
|---|---|---|
| `operacion → catalogos` | `AptitudRecursosEnProceso` | `operacion.AptitudRecursos` |
| `reportes → catalogos` | `ReportesCatalogosEnProceso` | `reportes.CatalogosPort` |
| `operacion/catalogos/reportes → identidad` (restringida, DEC-051) | `RevocacionEnProceso` | `operacion.Revocacion`, `catalogos.RevocacionPort`, `reportes.RevocacionPort` |
| `bff → operacion` | `BFFOperacionEnProceso` | `bff.OperacionPort` |
| `bff → identidad` | `BFFIdentidadEnProceso` | `bff.IdentidadPort` |

**Scaffold / brechas conocidas:** los servicios de dominio de los módulos son
scaffold, así que las llamadas síncronas devuelven "no implementado" en runtime;
el cableado es correcto en forma y paridad (I3), pero esto **no es evidencia de
G2** (DEC-049). La proyección de tombstones de `bff` (`bff.Deps.Revocacion`) va
`nil`: su worker de `sesion.revocada` necesita el bus, en espera de ADR 32; el
middleware verifica firma pero aún no consulta tombstones, así que la revocación
no es efectiva end-to-end. `bff` solo cablea sus aristas de fase (a) (operacion,
identidad); catalogos y reportes son fase (b) y no están en su `Deps` todavía.

Este repo se completa de forma **incremental**: cada vez que un módulo cambia su
superficie se actualiza `go.mod`, el cableado en `construirModulos`, su adaptador
en proceso con pruebas, y esta tabla.

## Correr local

`core` abre **un pool por rol** (I1), cada uno con su propia cadena de conexión.
Las variables son obligatorias (la configuración falla listando todas las que
falten, no solo la primera):

| Variable | Rol |
|---|---|
| `DB_SVC_CATALOGOS_URL` | `svc_catalogos` |
| `DB_OP_VIAJES_URL` | `op_viajes` |
| `DB_OP_ASIGNACION_URL` | `op_asignacion` |
| `DB_OP_BITACORA_URL` | `op_bitacora` |
| `DB_OP_AUDITORIA_URL` | `op_auditoria` |
| `DB_OP_ROSTER_URL` | `op_roster` |
| `DB_SVC_REPORTES_URL` | `svc_reportes` |
| `DB_SVC_IDENTIDAD_URL` | `svc_identidad` |

En producción (sin tag), `core` usa el verificador JWT real y exige `JWKS_URL`:

```bash
SERVICE_NAME=core HTTP_ADDR=:8080 JWKS_URL=https://idp.interno/.well-known/jwks.json \
  DB_SVC_CATALOGOS_URL=... DB_OP_VIAJES_URL=... DB_OP_ASIGNACION_URL=... \
  DB_OP_BITACORA_URL=... DB_OP_AUDITORIA_URL=... DB_OP_ROSTER_URL=... \
  DB_SVC_REPORTES_URL=... DB_SVC_IDENTIDAD_URL=... \
  go run ./cmd/core
```

En desarrollo, con el stub de autenticación (`devauth`, DEC-049), no se necesita
un IdP (las cadenas de BD siguen siendo obligatorias):

```bash
SERVICE_NAME=core HTTP_ADDR=:8080 \
  DB_SVC_CATALOGOS_URL=... DB_OP_VIAJES_URL=... DB_OP_ASIGNACION_URL=... \
  DB_OP_BITACORA_URL=... DB_OP_AUDITORIA_URL=... DB_OP_ROSTER_URL=... \
  DB_SVC_REPORTES_URL=... DB_SVC_IDENTIDAD_URL=... \
  go run -tags devauth ./cmd/core

curl localhost:8080/health   # {"status":"ok"}
curl localhost:8080/ready    # {"status":"ok"} | 503
```

Trabajo local multi-repo: un `go.work` **no versionado** en `repos/` une los
módulos (00 regla 7). Este repo no lo crea ni lo referencia.

## Stub de autenticación de desarrollo (`devauth`)

`internal/devauth/` es el stub de DEC-049. Solo se compila con el build tag
`devauth`; **el binario de producción no lo contiene** (lo verifica la compuerta
anti-stub del CI). Como defensa adicional, el stub se niega a arrancar si el
entorno declarado es producción (`CORE_PRODUCCION=true`).

Modela los **ocho roles** de DEC-045 (identificadores de DEC-053, tomados del
enum de `pf-pkg-contracts`, no escritos a mano) en **dos tenants** fake. El token
de desarrollo es el Subject `"<rol>@<tenant>"`, p. ej.:

```bash
curl -H "Authorization: Bearer jefe_operativo@tenant-dev-alfa" localhost:8080/...
```

Una petición sin una identidad de desarrollo válida se rechaza igual que sin JWT
(fallo cerrado). El stub **no emite alcance** (unidad/cliente): el alcance se
resuelve en servidor (DEC-053).

> **Las pruebas que corren con `devauth` no son evidencia de `G2`** (DEC-049): ni
> revocación ≤30 s ni aislamiento real de tenant. Eso requiere `identidad`
> integrada de verdad.

## Ramas

`pf-core-m` (main) / `pf-core-d` (desarrollo) / `pf-core-q` (QA), según
`docs/arquitectura/repo-convention.md`. Se trabaja en `pf-core-d`.

## Pendientes y lo que espera a ADR 32

- **Composición real:** catalogos (`v0.1.0`), operacion (`v0.1.1`) y reportes
  (`v0.1.0`) ya están cableados con sus pools y las aristas en proceso a
  catalogos. Sus `Routes` y `Workers` siguen en scaffold (sin rutas de negocio ni
  workers), así que todavía no hay outbox que relevar ni consumidores que
  arrancar; se cablean cuando los módulos los implementen. Faltan por llegar
  `identidad` y `bff` (DEC-049).
- **Adaptador real del broker V1** (para la outbox y los consumidores de
  `sesion.revocada`) e **IdP on-premise**: en espera de que ADR 32 elija
  componente (DEC-062). En desarrollo, `core` usará `bus/memory` de
  `pf-pkg-platform-go`; no se elige producto aquí.
- **Manifiestos de despliegue** para el orquestador on-premise (ADR 32): fuera
  de alcance de este repo hasta ADR 32. No hay task definitions ni manifiestos
  de AWS.
- **`sesion.revocada`:** cada módulo protegido tendrá su propia suscripción
  durable, proyección y readiness (DEC-057/058); se cablea cuando los módulos
  existan y el evento esté en la versión fijada de `pf-pkg-contracts`.
