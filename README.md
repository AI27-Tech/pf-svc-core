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
| catalogos | ❌ no cableado | `pf-svc-catalogos` aún vacío, sin versión publicada |
| operacion | ❌ no cableado | `pf-svc-operacion` aún vacío, sin versión publicada |
| identidad | ❌ no cableado | en paralelo (DEC-049); mientras tanto, `devauth` |
| bff | ❌ no cableado | — |
| reportes | ❌ no cableado | — |

**Hoy `core` arranca sin ningún módulo** y solo sirve `/health` y `/ready`
(DEC-049): no hay ningún `pf-svc-*` con versión publicada que componer. Este
repo se completa de forma **incremental**: cada vez que un módulo de `core`
publica versión, se agrega a `go.mod`, se cablea en `construirModulos` de
`cmd/core/main.go`, se crea su adaptador en proceso en `internal/adapters/` con
sus pruebas de paridad (I3), y se actualiza esta tabla.

Orden de llegada esperado (DEC-049): `catalogos` + `operacion` → `identidad` →
`bff` → `reportes`.

## Correr local

En producción (sin tag), `core` usa el verificador JWT real y exige `JWKS_URL`:

```bash
SERVICE_NAME=core HTTP_ADDR=:8080 JWKS_URL=https://idp.interno/.well-known/jwks.json \
  go run ./cmd/core
```

En desarrollo, con el stub de autenticación (`devauth`, DEC-049), no se necesita
un IdP:

```bash
SERVICE_NAME=core HTTP_ADDR=:8080 go run -tags devauth ./cmd/core

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

- **Composición real:** bloqueada hasta que `catalogos` y `operacion` publiquen
  versión con su `New()`/`Consultas()` (DEC-049). Hasta entonces no hay
  adaptadores en proceso ni workers que arrancar.
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
