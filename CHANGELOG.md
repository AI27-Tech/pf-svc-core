# CHANGELOG

## 2026-10-28 — tracer.21: subir el pin de contrato

**Resumen:** se sube el pin de `pf-pkg-contracts` de `v0.1.0-tracer.20` a `v0.1.0-tracer.21`. Aditivo (ficha técnica E8-T12 / histórico E8-T13); los módulos que `core` compone no cambian su superficie. Solo `go.mod`/`go.sum`.

**Para quién:** alineación de versión de contrato de la raíz de composición con los módulos que cablea.

**Cambios:**
- `go.mod`/`go.sum`: `api-client-go` `v0.1.0-tracer.20` → `v0.1.0-tracer.21`.

**Verificación:** `go build/vet ./...`, `go test ./...` (3 paquetes ok), `gofmt` limpio, `arch-test` (v0.1.7, la del CI) OK. Sin renombres de enum (§8).

## 2026-10-27 — Componer mensajeria y cablear bff→catalogos, bff→operacion (E8-T11 + E6-T3 + E7)

**Resumen:** core sube los pines a bff v0.1.4, catalogos v0.1.3, operacion v0.1.8 y pinea mensajeria v0.1.0, y cablea en proceso las aristas que esos tags habilitan. Con esto el selector de cliente (E8-T11), la descarga del expediente (E6-T3) y el panel de emergencia (E7) quedan resueltos en el binario compuesto.

**Para quién:** para el deployment compuesto. El frontend interno obtiene `GET /api/v1/clientes` (selector), la descarga de UN viaje compone bitácora/auditoría desde operacion, y el panel de Reacción opera el carril de emergencia, todo a través de bff con las aristas resueltas en proceso.

**Cambios:**
- `go.mod`: bff `v0.1.3→v0.1.4`, catalogos `v0.1.2→v0.1.3`, operacion `v0.1.7→v0.1.8`, mensajeria nuevo `v0.1.0`. `api-client-go` sube a `tracer.20` por las dependencias.
- `internal/adapters/bff_catalogos.go` (nuevo): adaptador en proceso `bff→catalogos` para `listarClientes` (E8-T11).
- `internal/adapters/bff_mensajeria.go` (nuevo): adaptador en proceso `bff→mensajeria` sobre `mensajeria.Emergencias()` (los 8 métodos del carril de emergencia, E7). El `bearer` del puerto se ignora en proceso (claims en el ctx).
- `internal/adapters/bff_operacion.go`: suma `ListarBitacora`, `ConsultarAuditoria`, `RegistrarDescargaViaje` (E6-T3), que el `OperacionPort` de bff v0.1.4 exige.
- `cmd/core/main.go`: `construirModulos` arma `mensajeria.New` (dos pools del rol `svc_mensajeria`, Chat y Despacho con el MISMO pool en V1: no hay worker de despacho, DEC-023; upgrade a un segundo pool cuando exista), inyecta `bff.Deps.Catalogos` y `bff.Deps.Mensajeria`, y suma mensajeria a la lista de módulos. Nuevo env `DB_SVC_MENSAJERIA_URL` y pool en `rolesDeBD`/`pools`.
- `cmd/migrate/main.go`: bootstrap del esquema `mensajeria` (rol `svc_mensajeria`) y su `search_path` (su relay de outbox consulta sin calificar el esquema).
- Tests: los fakes de `internal/adapters` (operacionFake, catalogosFake, catalogosDimFake) se ponen al día con las interfaces ampliadas.

**Dependencia de tooling:** arch-test reconoce `core compone mensajeria` desde `pf-pkg-platform-go` (regla `requires_exactly` con mensajeria). El CI de core debe subir el pin del tool a la versión tagueada con esa regla (ver abajo). Verificado local: `arch-test` con la regla nueva da OK contra core.

**Verificación:** `go build ./...` + `go build ./cmd/core ./cmd/migrate`, `go vet ./...`, `go test ./...` (verde), `arch-test` con la regla nueva (OK). El pin del tool en el CI se sube en el mismo PR una vez exista el tag de platform-go.



**Resumen:** Nuevo binario `cmd/migrate`: el bootstrap de esquemas, roles y grants del deployable `core`. Corre una vez en el despliegue, con credencial administrativa (`DB_ADMIN_URL`), ANTES de `cmd/core`. Arma los `migrate.Modulo` de identidad, catalogos, operacion (sus 5 esquemas vía `migrations.Orden`/`RolPorEsquema`) y reportes, los aplica en dos fases con el paquete `migrate` de platform-go (v0.1.6), y fija el `search_path` de los roles cuyo relay/outbox/inbox consulta sin calificar el esquema.

**Por qué un binario aparte (DEC-096):** los `0001` de cada módulo hacen `CREATE SCHEMA`/`CREATE ROLE`, privilegio que los pools de app de `core` (privilegio mínimo, I1) no tienen ni deben tener. Meter esa credencial en el proceso del servicio la dejaría viva en cada arranque; `cmd/migrate` la confina al paso de despliegue y termina. `cmd/core` arranca después con sus roles de app y asume la base migrada.

**Cambios:**
- `cmd/migrate/main.go` (nuevo): bootstrap. Una sola conexión admin aplica todos los módulos (el `ResolverEjecutor` la devuelve para cualquier rol; la pertenencia de cada esquema a su rol la fijan los `.sql`). `search_path` por rol tras aplicar (DEC-096 nota 3).
- `cmd/core/main.go`: se retira el TODO de migraciones; se documenta que el bootstrap es externo (`cmd/migrate`) y que `core` asume la base migrada.
- `go.mod`/`go.sum`: `pf-pkg-platform-go` a `v0.1.6` (trae el paquete `migrate`); se importan los subpaquetes `migrations` de los cuatro módulos con esquema.

**Verificado:** `go build`/`vet` en verde, `gofmt` limpio, `arch-test@v0.1.3 OK`. Bootstrap probado end-to-end contra Postgres efímero: crea los 8 esquemas y 8 roles, la excepción de GRANT de `op_asignacion` sobre `viajes.outbox` queda efectiva (prueba que las dos fases corrieron en orden), los 4 `search_path` fijados, y reaplicar es idempotente.

**Fuera de este cambio:** cablear el mismo bootstrap en `pf-svc-mensajeria` (su `cmd/migrate`), y que el gate G2 reuse estos binarios en vez de su lógica bash inline.

## 2026:10:15 (hora local) — operacion consume catalogo.actualizado (E5-T5)

**Resumen:** core inyecta el Subscriber durable del bus a operacion, para que su consumidor de `catalogo.actualizado` (E5-T5) arranque como worker e invalide la caché en proceso de aptitud. Antes el camino quedaba cableado en operacion pero muerto: core lo construía sin Subscriber.

**Cambios:**
- `cmd/core/main.go`: el `operacion.Deps` recibe `Subscriber: subscriber` (el mismo `bc.durable` que ya se pasa a reportes). Con él, operacion envuelve su puerto Aptitud con la caché y abre el consumidor durable `operacion-catalogos`; aparece en readiness vía `RunGroup` como `worker:bus-durable:operacion-catalogos`. mensajeria NO se cablea aquí: vive en su propio deployable y su arista entrante (bff → mensajeria) es siempre HTTP.
- `cmd/core/wiring_test.go`: nuevo `TestOperacionConsumeCatalogosConSubscriber` (con Subscriber, operacion aporta el worker de catálogos; sin él, no). Se ajusta `TestReportesConsumeConSubscriber`: la aserción pasa de "delta total de workers = 1" a presencia/ausencia por consumidor, porque ahora operacion también aporta un consumidor cuando hay Subscriber.

**Nota de verificación:** build conjunto de core y wiring_test verdes con el bus en memoria. No se cambió `go.mod`: durante el desarrollo core se enlaza a la versión local de operacion (rama feature) vía `go.work` (de trabajo, no versionado, no se commitea). El bump de `pf-svc-operacion` a su versión publicada queda para cuando se libere.

## 2026:10:14 (hora local) — reportes consume el bus (E6-T1, Paso 3)

**Resumen:** core inyecta el Subscriber durable del bus a reportes, para que su consumidor de proyecciones (E6-T1, Pasos 1-2) arranque como worker y reportes consuma `viaje.*` end-to-end. Antes el camino estaba cableado en reportes pero muerto: core lo construía sin bus. Con esto, un core desplegado contra NATS puebla `hecho_viaje`/`hecho_transicion` de reportes a partir de lo que publica operacion.

**Cambios:**
- `go.mod`: `pf-svc-reportes` de `v0.1.1` a `v0.1.2` (la versión que expone `Deps.Subscriber`).
- `cmd/core/main.go`: `construirModulos` recibe un parámetro nuevo `subscriber bus.Subscriber` (sibling de `busPublisher` y `broadcaster`, cada capacidad del bus tipada por separado). En la raíz se pasa `bc.durable`, el mismo objeto que ya actúa de Publisher durable, que también implementa `bus.Subscriber`. reportes se construye con `Subscriber: subscriber`; su `Workers()` ya devuelve el consumidor cuando el Subscriber no es nil (reportes v0.1.2). El worker aparece en readiness vía `RunGroup` como `worker:bus-durable:reportes`.
- `cmd/core/wiring_test.go` (nuevo): test de cableado del Paso 3 con el bus en memoria (sin NATS ni Postgres). Afirma por diferencia que, con Subscriber, reportes aporta exactamente un worker más (su consumidor durable), y que sin Subscriber no aporta ninguno. La aserción fuerte es la diferencia de conteo; el nombre del worker se comprueba por substring para no acoplarse al prefijo del transporte (NATS y el bus en memoria usan `bus-durable:reportes`).

**Nota de verificación end-to-end:** la prueba contra NATS real (que el worker aparezca en `/ready` y que un `viaje.creado` publicado llegue a la proyección) sigue la frontera del `bus_test.go` existente (build tag `nats`, fuera de CI). Este cambio la deja lista; su ejercicio con broker + Postgres vivos es el Paso 4 (gate G2 #4/#8).

**Verificado:** gofmt limpio, `go vet ./...`, `go build ./...`, `go test ./...` (incluye el test de cableado nuevo), `go build -tags devauth ./...`, `go test -tags devauth ./...`, compuerta anti-stub (`go list -deps ./cmd/core` sin `internal/devauth`), build `-mod=readonly`, `go build ./cmd/core`, y arch-test@v0.1.3 OK.

**Archivos modificados:**
- `go.mod`, `go.sum`
- `cmd/core/main.go`, `cmd/core/wiring_test.go`

## 2026:10:07 17:27 — identidad + bff (composition root completo)

**Resumen:** Se cablean identidad y bff en core, completando el composition root de los cinco módulos de V1 (arch-test exige exactamente `[bff, identidad, operacion, catalogos, reportes]`). Para el deployable `core`: ya compone, autentica y monta las rutas de los cinco; las aristas síncronas co-ubicadas se resuelven con adaptadores en proceso con paridad I3.

**Cambios:**
- `go.mod`: se fijan las versiones nuevas (`catalogos v0.1.1`, `operacion v0.1.3`, `reportes v0.1.1`) y se agregan `identidad v0.1.1` y `bff v0.1.0`, todas exactas.
- `cmd/core/main.go`: pool `svc_identidad` (`DB_SVC_IDENTIDAD_URL`) en `rolesDeBD`, `pools` y `abrirPools`. `construirModulos` construye los cinco módulos y conecta las aristas: `Revocacion` en operacion/catalogos/reportes y `bff.Deps{Operacion, Identidad, Verificador}`. identidad se devuelve aparte porque su `Routes` pide verificador y revocación (firma distinta de la interfaz `module`); sus `Workers()` (relay de outbox) entran en la recolección. El autenticador (real o `devauth`) se inyecta en bff y en las rutas protegidas de identidad.
- `internal/adapters/revocacion.go` (nuevo): `RevocacionEnProceso`, un solo tipo que satisface los tres `RevocacionPort` (firma idéntica, DEC-051), sobre `identidad.Consultas().Revocacion()`. Tres constructores, cada uno con el `PresupuestoRevocacion` de su consumidor (I3).
- `internal/adapters/bff_identidad.go` (nuevo): `BFFIdentidadEnProceso`, satisface `bff.IdentidadPort` sobre `identidad.Consultas().Sesion()`, con `bff.PresupuestoIdentidad`.
- `internal/adapters/bff_operacion.go` (+ test, nuevos): `BFFOperacionEnProceso`, satisface `bff.OperacionPort` sobre `operacion.Consultas()`, con `bff.PresupuestoOperacion`. Pruebas de paridad I3 (presupuesto, timeout→Degradado, cancelación, propagación de error por clase).
- `.github/workflows/ci.yml`: pin de arch-test subido de `@v0.1.0` a `@v0.1.1` (la versión que valida `requires_exactly` y honra `shared_packages`/`roles`).
- `README.md`: tabla de los cinco módulos, adaptadores en proceso, env var de identidad, ejemplos de arranque y brechas conocidas.

**Verificado:** gofmt limpio, `go vet ./...`, `go build ./...`, `go test ./...` (nuevas pruebas de paridad bff→operacion), build y test `-tags devauth`, compuerta anti-stub (`go list -deps ./cmd/core` sin `internal/devauth`), build `-mod=readonly`, `go build ./cmd/core`, y **arch-test@v0.1.1 OK con los cinco módulos**. En runtime: core falla listando las ocho variables de BD faltantes incluida `DB_SVC_IDENTIDAD_URL`; con `-tags devauth` y `CORE_PRODUCCION=true` el stub se niega a arrancar.

**Brechas / alcance (no es evidencia de G2, DEC-049):** los servicios de dominio de los módulos son scaffold; las llamadas síncronas devuelven "no implementado" en runtime. La proyección de tombstones de bff (`bff.Deps.Revocacion`) va `nil`: su worker de `sesion.revocada` espera el bus (ADR 32), así que la revocación no es efectiva end-to-end todavía. bff cablea solo sus aristas de fase (a) (operacion, identidad); catalogos y reportes son fase (b). Paridad I3 de los adaptadores de identidad (bff→identidad, revocación): probada por conformación de tipos en el build; la paridad de runtime no se prueba en core porque `identidad.Consultas()` no expone un seam fakeable (el servicio vive en su `internal/`).

**Archivos modificados:**
- `go.mod`, `go.sum`
- `cmd/core/main.go`
- `internal/adapters/revocacion.go` (nuevo)
- `internal/adapters/bff_identidad.go` (nuevo)
- `internal/adapters/bff_operacion.go` (nuevo), `internal/adapters/bff_operacion_test.go` (nuevo)
- `.github/workflows/ci.yml`
- `README.md`

## 2026:10:05 (hora local) — reportes

**Resumen:** Segundo incremento de composición (prompt 09): `core` suma `reportes` (`v0.1.0`) a catalogos y operacion, resolviendo la arista `reportes → catalogos` con un adaptador EN PROCESO y sus pruebas de paridad (I3).

**Cambios:**
- `go.mod`: se fija `pf-svc-reportes v0.1.0` (versión exacta).
- `internal/adapters/reportes_catalogos.go` (nuevo): `ReportesCatalogosEnProceso`, adaptador en proceso del puerto `reportes.CatalogosPort` sobre `catalogos.Consultas()` (`ObtenerContrato`/`ObtenerSla`). Respeta `reportes.PresupuestoCatalogos` (leído del consumidor), la misma taxonomía de error (vencimiento/cancelación → Degradado; el resto se propaga sin recategorizar) y la cancelación.
- `internal/adapters/reportes_catalogos_test.go` (nuevo): pruebas de paridad I3 (deadline impuesto, timeout → Degradado, cancelación, propagación de cada clase de error del contrato).
- `cmd/core/main.go`: nuevo rol/pool `svc_reportes` en `rolesDeBD` y `abrirPools`; `construirModulos` construye `reportes.New(...)` con el adaptador en proceso. `config.Load` exige ahora siete cadenas de conexión por rol.
- `internal/adapters/doc.go`, `README.md`: tabla de módulos (reportes ✅), variable `DB_SVC_REPORTES_URL`, pendientes (faltan identidad y bff).

**Verificado:** gofmt limpio, `go vet ./...`, `go build ./...`, `go test ./...`, build+test `-tags devauth`, compuerta anti-stub, build `-mod=readonly`, arch-test OK (reportes ya requerido por la raíz de composición).

**Archivos modificados:**
- `go.mod`, `go.sum`
- `cmd/core/main.go`
- `internal/adapters/reportes_catalogos.go` (nuevo)
- `internal/adapters/reportes_catalogos_test.go` (nuevo)
- `internal/adapters/doc.go`
- `README.md`

## 2026:10:05 (hora local)

**Resumen:** Primer incremento de composición real (prompt 09, DEC-049): `core` ya cablea `catalogos` (`v0.1.0`) y `operacion` (`v0.1.1`), resolviendo la arista `operacion → catalogos` con el adaptador EN PROCESO de `internal/adapters/` y sus pruebas de paridad (I3). Antes `core` arrancaba sin módulos.

**Cambios:**
- `go.mod`: se fijan `pf-svc-catalogos v0.1.0` y `pf-svc-operacion v0.1.1` (versiones exactas, sin `replace` ni `go.work`).
- `internal/adapters/aptitud_recursos.go` (nuevo): `AptitudRecursosEnProceso`, el adaptador en proceso del puerto `operacion.AptitudRecursos` sobre `catalogos.Consultas()`. Respeta el mismo presupuesto de tiempo (`operacion.PresupuestoAptitud`, leído del consumidor, no duplicado), la misma taxonomía de error (vencimiento/cancelación → Degradado, igual que el HTTP; el resto se propaga sin recategorizar) y la cancelación del contexto.
- `internal/adapters/aptitud_recursos_test.go` (nuevo): pruebas de paridad I3 — impone el deadline del presupuesto, timeout → Degradado, cancelación propagada, y cada clase de error del contrato se propaga sin recategorizar.
- `cmd/core/main.go`: `construirModulos` ahora construye catalogos (hoja) y operacion (cinco pools/rol, ADR 33) con el adaptador en proceso; nuevo `abrirPools` abre un pool por rol (I1), los registra en readiness y los drena al apagar; `config.Load` exige las seis cadenas de conexión por rol (falla con todas las faltantes).
- `internal/adapters/doc.go`, `README.md`: tabla de módulos cableados, variables de entorno por rol y pendientes actualizados.

**Verificado:** gofmt limpio, `go vet ./...`, `go build ./...`, `go test ./...` (pruebas de paridad del adaptador), build+test `-tags devauth`, compuerta anti-stub (devauth ausente del build de producción), build `-mod=readonly`, arch-test OK.

**Nota:** los `Routes`/`Workers` de ambos módulos siguen en scaffold (sin rutas de negocio ni workers), así que todavía no hay outbox que relevar ni consumidores de `sesion.revocada` que arrancar; se cablean cuando los módulos los implementen. La arista restringida a identidad (`Revocacion`) se cablea cuando exista `identidad` (DEC-051).

**Archivos modificados:**
- `go.mod`, `go.sum`
- `cmd/core/main.go`
- `internal/adapters/aptitud_recursos.go` (nuevo)
- `internal/adapters/aptitud_recursos_test.go` (nuevo)
- `internal/adapters/doc.go`
- `README.md`

## 2026:10:06 12:55

**Resumen:** Se agregó la plantilla de Pull Request del backend, adaptada al stack Go de AI27.

**Cambios:**
- Se agregó `.github/pull_request_template.md` (core no tenía): invariantes de arquitectura (I1-I5, contrato primero), base de datos, eventos/bus, verificación (go build/vet/test, arch-test, gofmt), CHANGELOG y checklist de convenciones.

**Archivos modificados:**
- `.github/pull_request_template.md`

## 2026:10:06 12:00

**Resumen:** Se agregó la regla de steering de CHANGELOG y este archivo, para alinear `pf-svc-core` con la convención del resto de repos.

**Cambios:**
- Se agregó `.kiro/steering/changelog.md` (`inclusion: always`): toda modificación del repo debe registrarse aquí, en español, con marca de tiempo, resumen, cambios y archivos modificados.
- Se creó `CHANGELOG.md` con las entradas de los cambios ya realizados en el repo.

**Archivos modificados:**
- `.kiro/steering/changelog.md`
- `CHANGELOG.md`

## 2026:10:06 10:22

**Resumen:** Se corrigió el workflow de CI para autenticar siempre contra los repos privados de la organización.

**Cambios:**
- El paso que configuraba las credenciales de git tenía la guarda `if: ${{ env.GH_PAT != '' }}`, pero `GH_PAT` solo estaba definido en el `env` de ese mismo paso; el `if` se evalúa antes de aplicar ese `env`, así que la condición era siempre falsa y el paso se saltaba, dejando a `go mod download` sin credenciales (error 403 / "could not read Username").
- Se promovió `GH_PAT` al `env` del job y se configuró el acceso sin guarda, con un paso previo que falla claro si el secreto no está.
- El secreto `GH_PAT` es a nivel de repo (requerido en el plan Free: los secretos de organización no llegan a repos privados).

**Archivos modificados:**
- `.github/workflows/ci.yml`

## 2026:10:06 10:04

**Resumen:** Se completó `go.sum` y se fijaron las dependencias del tool `arch-test`.

**Cambios:**
- El `go.sum` del commit inicial se generó con un `go.work` temporal y le faltaban los hashes de `go.mod` de los dos deps privados (`pf-pkg-platform-go` v0.1.0 y `pf-pkg-contracts/.../api-client-go` v0.1.0-tracer.3), así que un build `-mod=readonly` (el de CI) fallaba con "missing go.sum entry". Con las credenciales resueltas, `go mod tidy` completó los hashes desde el remoto.
- Se agregó `tools.go` (build tag `tools`, no entra en ningún binario) que importa `github.com/AI27-Tech/pf-pkg-platform-go/tools/arch-test` para que `go mod tidy` conserve sus dependencias (p. ej. `gopkg.in/yaml.v3`) en `go.mod`/`go.sum`; el CI corre `go run .../tools/arch-test@<version> .` y las necesita.

**Archivos modificados:**
- `go.mod`
- `go.sum`
- `tools.go`

## 2026:10:05 17:04

**Resumen:** Esqueleto de la raíz de composición `pf-svc-core` (prompt 09, DEC-050). Arranque incremental (DEC-049): hoy no hay ningún `pf-svc-*` publicado, así que `core` arranca sin módulos y solo sirve `/health` y `/ready`.

**Cambios:**
- `go.mod`: requiere solo `pf-pkg-contracts` (api-client-go v0.1.0-tracer.3) y `pf-pkg-platform-go` v0.1.0; ningún `pf-svc-*` todavía. Sin `replace` ni `go.work` versionado.
- `cmd/core/main.go`: cableado puro — config, observabilidad (trazas+métricas), autenticador, servidor con `/health` y `/ready`, y arranque/drenado de workers vía `worker.RunGroup`. `construirModulos` devuelve lista vacía hoy; es el único punto que cambia por incremento.
- Autenticación por build tag: `auth_prod.go` (!devauth) usa el verificador JWT real y exige `JWKS_URL`; `auth_devauth.go` (devauth) usa el stub.
- `internal/devauth`: stub de desarrollo (DEC-049), solo con `//go:build devauth`. Modela los ocho roles de DEC-045 (del enum del contrato) en dos tenants fake; falla cerrado; se niega a arrancar en producción. No emite alcance (DEC-053). No es evidencia de G2.
- `internal/adapters/doc.go`: documenta las aristas co-ubicadas a cablear cuando los módulos existan; hoy vacío de adaptadores.
- CI: build/vet/test de producción, build+test con `-tags devauth`, compuerta anti-stub (devauth ausente del build de producción), `arch-test`, e imagen con la plantilla de `pf-pkg-platform-go` (`CMD_PATH=./cmd/core`, nunca con devauth).

**Archivos modificados:**
- `go.mod`
- `go.sum`
- `.gitignore`
- `README.md`
- `cmd/core/main.go`
- `cmd/core/auth_prod.go`
- `cmd/core/auth_devauth.go`
- `internal/devauth/devauth.go`
- `internal/devauth/devauth_test.go`
- `internal/adapters/doc.go`
- `.github/workflows/ci.yml`
