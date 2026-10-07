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
| catalogos | ✅ cableado (`v0.1.0`) | hoja del grafo; un pool `svc_catalogos` |
| operacion | ✅ cableado (`v0.1.1`) | cinco pools (`op_viajes/asignacion/bitacora/auditoria/roster`, ADR 33); arista `operacion → catalogos` en proceso |
| identidad | ❌ no cableado | en paralelo (DEC-049); mientras tanto, `devauth` |
| bff | ❌ no cableado | — |
| reportes | ❌ no cableado | — |

`core` compone hoy **catalogos** (hoja) y **operacion**. La arista síncrona
`operacion → catalogos` (puerto `AptitudRecursos`) se resuelve con el adaptador
**en proceso** de `internal/adapters/`, que respeta el mismo presupuesto de
tiempo (`operacion.PresupuestoAptitud`), la misma taxonomía de error y la
cancelación que el adaptador HTTP de `pf-svc-operacion` (I3); sus pruebas de
paridad viven junto al adaptador. El adaptador HTTP del mismo puerto **no** se
cablea aquí porque catalogos está co-ubicado.

Los módulos aún no montan rutas de negocio (sus `Routes` son scaffold) ni
declaran workers; `core` sirve `/health` y `/ready`, abre un pool por rol y los
registra en readiness. Este repo se completa de forma **incremental**: cada vez
que un módulo publica versión, se agrega a `go.mod`, se cablea en
`construirModulos` de `cmd/core/main.go`, se crea su adaptador en proceso con sus
pruebas, y se actualiza esta tabla.

Orden de llegada esperado (DEC-049): `catalogos` + `operacion` → `identidad` →
`bff` → `reportes`.

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

En producción (sin tag), `core` usa el verificador JWT real y exige `JWKS_URL`:

```bash
SERVICE_NAME=core HTTP_ADDR=:8080 JWKS_URL=https://idp.interno/.well-known/jwks.json \
  DB_SVC_CATALOGOS_URL=... DB_OP_VIAJES_URL=... DB_OP_ASIGNACION_URL=... \
  DB_OP_BITACORA_URL=... DB_OP_AUDITORIA_URL=... DB_OP_ROSTER_URL=... \
  go run ./cmd/core
```

En desarrollo, con el stub de autenticación (`devauth`, DEC-049), no se necesita
un IdP (las cadenas de BD siguen siendo obligatorias):

```bash
SERVICE_NAME=core HTTP_ADDR=:8080 \
  DB_SVC_CATALOGOS_URL=... DB_OP_VIAJES_URL=... DB_OP_ASIGNACION_URL=... \
  DB_OP_BITACORA_URL=... DB_OP_AUDITORIA_URL=... DB_OP_ROSTER_URL=... \
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

- **Composición real:** catalogos (`v0.1.0`) y operacion (`v0.1.1`) ya están
  cableados con sus pools y la arista en proceso `operacion → catalogos`. Sus
  `Routes` y `Workers` siguen en scaffold (sin rutas de negocio ni workers), así
  que todavía no hay outbox que relevar ni consumidores que arrancar; se cablean
  cuando los módulos los implementen. Faltan por llegar `identidad`, `bff` y
  `reportes` (DEC-049).
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
