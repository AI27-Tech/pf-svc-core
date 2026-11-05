# CHANGELOG

## 2026-11-05 16:48 — Pins del demo: identidad v0.1.4, operacion v0.1.19, bff v0.1.18

**Para quién:** el binario del demo (DEC-088). Tres arreglos llegan a `core`:
- **identidad v0.1.4 (PR #28, DEC-138):** volver a entrar sin cerrar sesión ya no responde 500. El login nuevo reemplaza la sesión viva y el token viejo deja de servir. Un usuario de Keycloak sin sujeto recibe 401 en lugar de 500.
- **operacion v0.1.19 (PR #91, DEC-112/DEC-132):** un cliente solo lista y lee sus propios viajes. Antes veía los de todas las empresas. El viaje de otro cliente responde 404.
- **bff v0.1.18 (PR #76):** el filtro `estado` de la lista de viajes funciona en los dos portales.

**Cambios:**
- `go.mod`/`go.sum`: `pf-svc-identidad` v0.1.3 → v0.1.4, `pf-svc-operacion` v0.1.18 → v0.1.19, `pf-svc-bff` v0.1.17 → v0.1.18. Sin cambio de código en `core`.

**Verificación:** `gofmt` limpio, `go build ./...`, `go vet ./...`, `go test -count=1 ./...` y `arch-test` en verde. Recorrido del demo por el `bff` con login real de Keycloak 26.7.4: 92/92. Incluye un segundo login con la sesión viva, el aislamiento entre clientes (lista y detalle ajeno → 404) y el filtro por estado. Gate G2: 17/17.

## 2026-11-05 13:18 — El login de `core` corre contra Keycloak real

**Para quién:** los portales y quien opera `core`. `POST /api/v1/sesion` del `bff` ya inicia sesión de verdad: usuario, contraseña y TOTP contra Keycloak, y devuelve tokens firmados por Keycloak con `rol`, `audiencia`, `tenant_id` y `sesion_id`. El refresco (`POST /api/v1/sesion/refresh`) y el cierre (`DELETE /api/v1/sesion`, 204) también funcionan, y el cierre corta la sesión en Keycloak. Antes, el login y el logout entraban en panic por el IdP nil (B2). Quien opera `core` debe dar ahora las variables de Keycloak y del verificador (ver README); sin ellas, `core` no arranca y nombra las que faltan (DEC-095).

**Resumen:** sube `identidad` a v0.1.3 (E4-T1, E4-T3 a E4-T6), cablea el adaptador de Keycloak en `identidad.Deps.IDP` igual que el cmd de `identidad` y el `TokenParser` que activa el login. El verificador JWT de producción valida además `aud` e `iss` configurables.

**Cambios:**
- `go.mod`/`go.sum`: `pf-svc-identidad` v0.1.2 → v0.1.3. MVS no movió contratos (`tracer.37`) ni `platform-go` (v0.1.9); `go build` no pidió renombrar constantes de enum.
- `cmd/core/idp.go`: `construirIDP` arma el proveedor con `idpconfig.Cargar` (variables `IDENTIDAD_IDP` / `IDENTIDAD_KEYCLOAK_*`, secretos directos o `_FILE`) y `identidad.NewIDPKeycloak`. Exige `IDENTIDAD_IDP=keycloak` en los dos builds: `identidad.New` falla sin IdP y el doble en memoria vive en `internal/` de `identidad`.
- `cmd/core/auth_prod.go`: `construirAutenticador` exige `JWKS_URL`, `JWT_AUDIENCE` y `JWT_ISSUER` (todas las faltantes en un error). La audiencia ya no sale de `SERVICE_NAME`. `verificadorConEmisor` rechaza un `iss` distinto, porque `auth.Verifier` de `platform-go` v0.1.9 no lo valida. Devuelve también el `TokenParser` sobre el mismo verificador.
- `cmd/core/auth_devauth.go`: sin cambio para el stub; devuelve `TokenParser` nil, así que con `devauth` el login y el refresco responden 501.
- `internal/adapters/identidad_tokens.go`: `TokensIdentidad` implementa `identidad.TokenParser` (`sub` y `sesion_id` del access token, verificados).
- `cmd/core/main.go`: `construirModulos` recibe el proveedor y el `TokenParser` y los pasa a `identidad.Deps`. `IDPAdmin` no se cablea: las rutas de gobierno no son públicas en `core` (DEC-137).
- Pruebas: `idp_test.go` (con y sin Keycloak, faltantes nombradas, sin filtrar secretos), `auth_prod_test.go` (variables obligatorias y `iss`), `identidad_tokens_test.go` (lectura y fallo cerrado); `wiring_test.go` construye `identidad` con el adaptador real y configuración ficticia.
- `README.md`: variables de Keycloak y del verificador.

**Notas:**
- Con `devauth`, `core` ya no arranca sin Keycloak configurado: es consecuencia de `identidad` v0.1.3. Para arrancarlo sin Keycloak, `identidad` tendría que exportar su doble en memoria.
- Hallazgos de `identidad` v0.1.3 observados con Keycloak real, sin cambio aquí: un usuario de Keycloak sin `sujeto` en `identidad` recibe 500 en el login (el login no da de alta sujetos), y un segundo login del mismo usuario con su sesión viva también da 500 (`una_sesion_viva`). Expirar no libera esa sesión; solo la libera el logout.

**Verificación:** `gofmt` limpio, `go vet ./...` (con y sin `-tags devauth`), `go build ./...` y `-tags devauth`, `go test -race ./...` y `go test -tags devauth ./...`, `arch-test` v0.1.9 en verde; el binario de producción no contiene `internal/devauth`. Contra Keycloak 26.7.4 con el realm de `identidad` v0.1.3, solo por el `bff`: login de los 8 roles (201, claims correctos), refresh 200 con `sesion_id` estable, logout 204 sin panic, token previo rechazado en 1 s y la sesión de Keycloak cortada. Recorrido de la demo con tokens del login: 76 pasos, 0 fallas. Gate G2: 17/17.

**Archivos modificados:**
- `go.mod`, `go.sum`, `cmd/core/main.go`, `cmd/core/idp.go`, `cmd/core/idp_test.go`, `cmd/core/auth_prod.go`, `cmd/core/auth_prod_test.go`, `cmd/core/auth_devauth.go`, `cmd/core/wiring_test.go`, `internal/adapters/identidad_tokens.go`, `internal/adapters/identidad_tokens_test.go`, `README.md`, `CHANGELOG.md`

## 2026-11-04 18:16 — Grupo 2 del flujo interno compuesto en proceso y el bff como única puerta pública

**Para quién:** el panel interno, el portal cliente y quien opera `core`. En `core`, el panel ya lee el roster, completa la fecha/hora, lee el histórico de incidencias, lee y guarda la ficha técnica y lista los GPS, todo por el `bff`. Cambia también qué responde el servidor público: solo las rutas del `bff` (más `/health` y `/ready`) existen (DEC-137). Una ruta nativa de un módulo, por ejemplo `POST /api/v1/vehiculos` de `catalogos`, `POST /api/v1/viajes/{id}/descargas` de `operacion` o `DELETE /api/v1/sesiones/{id}` de `identidad`, ahora es 404. La sesión se inicia, refresca y cierra por `/api/v1/sesion` del `bff`. Quien opera `core` puede fijar con `MENSAJERIA_DISPONIBLE_MINUTOS` cuántos minutos sin actividad deja de estar disponible un Ejecutivo (DEC-128; vacío o inválido, 30).

**Resumen:** sube los pins a los tags publicados del grupo 2 y cablea los 6 métodos nuevos de los puertos del `bff` en los adaptadores en proceso. Cierra la caída de `montarRutas` al mux interno y deja de montar las rutas de `identidad` (DEC-137). Pasa a `mensajeria` la ventana de disponibilidad del Ejecutivo (DEC-128).

**Cambios:**
- `go.mod`/`go.sum`: `pf-svc-bff` v0.1.16 → v0.1.17; `pf-svc-operacion` v0.1.16 → v0.1.18; `pf-svc-catalogos` v0.1.3 → v0.1.5; `pf-svc-mensajeria` v0.1.7 → v0.1.8; `pf-svc-reportes` v0.1.4 → v0.1.5; `pf-pkg-platform-go` v0.1.8 → v0.1.9 (por MVS). Contratos siguen en `tracer.37` e `identidad` en v0.1.2. `go build` no pidió renombrar constantes de enum.
- `internal/adapters/bff_operacion.go`: `ListarUnidades`, `CompletarFechaHora`, `ListarIncidencias`, `ObtenerFicha`, `GuardarFicha` sobre `operacion.Consultas()`, con `bff.PresupuestoOperacion` y `traducirErrorOperacion`. `GuardarFicha` recibe el `&Tracto{}` que el handler del bff ya tradujo de `"tracto": null`.
- `internal/adapters/bff_catalogos.go`: `ListarGps` sobre `catalogos.Consultas().ListarGps(cursor)`.
- `cmd/core/main.go`: `montarRutas` solo monta las rutas del `bff`. Las `Routes` de `catalogos`, `operacion`, `reportes` y `mensajeria` ya no se llaman en `core`: solo arman handlers, sin efectos laterales, y sus `Workers` se recolectan aparte. Se quita `idn.Routes(srv.Mux(), ...)`: `identidad` conserva su relay de outbox y su fachada en proceso, que usa el `bff` para la sesión. `mensajeria.Deps.VentanaDisponible` sale de `MENSAJERIA_DISPONIBLE_MINUTOS` con `config.GetEnvInt`, la misma clave y default que el cmd de `mensajeria`.
- `cmd/core/wiring_test.go`: `TestMontarRutasSoloBFF` (los patrones del bff, incluidos los 6 nuevos y las 3 rutas de `/api/v1/sesion`) y `TestRutaNativaSinBFFEs404` (rutas nativas de `operacion`, `catalogos` e `identidad` dan 404).
- `internal/adapters/*_test.go`: los dobles de `ConsultasAPI` suman los métodos nuevos; la tabla de paridad del flujo interno suma 5 casos y `TestBFFCatalogosListarGps` cubre el cursor y la categoría del error.
- `.github/workflows/ci.yml`: `arch-test` pasa a v0.1.9, la versión de `platform-go` en `go.mod`, como pide el comentario del paso.

**Notas:**
- `pf-svc-realtime` llama a `identidad` por HTTP para la revocación. No está en el binario de la demo; queda como deuda para cuando se componga (DEC-137 #4, DT-17).
- Login y refresh por el `bff` llegan a `identidad` en proceso, pero `identidad` v0.1.2 no tiene IdP cableado ni `EmitirSesion`/`RefrescarSesion` implementados, así que hoy no emiten sesión. Era igual por la ruta propia de `identidad`. El logout (`DELETE /api/v1/sesion`) sí revoca: confirma la revocación y luego entra en panic por el IdP nil (B2), como antes.

**Verificación:** `gofmt` limpio, `go vet ./...` (con y sin `-tags devauth`), `go build ./...` y `-tags devauth`, `go test -race ./...` y `go test -tags devauth ./...`, `arch-test` v0.1.9 en verde; el binario de producción no contiene `internal/devauth`. Gate G2 contra el binario de esta rama, revocando por el `bff`: 17/17. Recorrido de la demo sobre el binario real: `/tmp/smoke-recorrido-demo.out`.

**Archivos modificados:**
- `go.mod`, `go.sum`, `cmd/core/main.go`, `cmd/core/wiring_test.go`, `internal/adapters/bff_operacion.go`, `internal/adapters/bff_operacion_test.go`, `internal/adapters/bff_catalogos.go`, `internal/adapters/bff_catalogos_test.go`, `internal/adapters/aptitud_recursos_test.go`, `internal/adapters/reportes_catalogos_test.go`, `.github/workflows/ci.yml`, `CHANGELOG.md`

## 2026-11-04 17:05 — Los 409 de mensajeria llegan con su código

**Para quién:** los frontends. Por `core`, un 409 de `mensajeria` deja de llegar como `"conflicto"` y trae el código del contrato (`emergencia_cerrada`, `conflicto_version_reclamo`, `conversacion_solo_lectura`, `stream_resync_required`, `transicion_invalida`).

**Resumen:** solo sube pins. `core` ya reenviaba el error de `mensajeria` tal cual; el código faltaba en origen y lo agrega `pf-svc-mensajeria` v0.1.7 (PR #57). `pf-svc-bff` v0.1.16 (PR #69) hace lo mismo en su carril HTTP, que `core` no usa, pero se sube para no quedar atrás.

**Cambios:**
- `go.mod`: `pf-svc-mensajeria` `v0.1.6` → `v0.1.7`; `pf-svc-bff` `v0.1.15` → `v0.1.16`.

**Archivos modificados:**
- `go.mod`, `go.sum`, `CHANGELOG.md`

## 2026-11-04 15:40 — Cablear en proceso las emergencias de E8-T18

**Para quién:** quien despliega `core`. Con este cambio, las rutas de emergencias del monitoreo y de la bandeja de Reacción del `bff` llegan a `mensajeria`, `operacion` y `catalogos` en proceso. Sin él, `core` no compilaría contra `bff v0.1.15`.

**Resumen:** los adaptadores en proceso implementan los métodos nuevos de los puertos del `bff` (E8-T18, DEC-135, DEC-121). Se suben los cuatro pins de la ola.

**Cambios:**
- `internal/adapters/bff_mensajeria.go`: `ActivarEmergencia` pasa la marca de reentrega que ya daba `mensajeria`, para que el `bff` responda 200 y no 201.
- `internal/adapters/bff_emergencias_por_id.go`: las 8 operaciones por id de la bandeja de Reacción en `BFFMensajeriaEnProceso`; `BFFOperacionEnProceso.ObtenerAsignacion`; `BFFCatalogosEnProceso.ObtenerCliente`, `ObtenerVehiculo` y `ObtenerCustodio` (versión vigente). Mismo presupuesto y traducción de error que el resto de cada adaptador (I3).
- `internal/adapters/bff_operacion_test.go`: el doble implementa `ObtenerAsignacion` y la prueba de paridad del flujo interno la incluye.
- `go.mod`: `api-client-go` `v0.1.0-tracer.36` → `v0.1.0-tracer.37`; `pf-svc-bff` `v0.1.14` → `v0.1.15`; `pf-svc-mensajeria` `v0.1.4` → `v0.1.6`; `pf-svc-operacion` `v0.1.15` → `v0.1.16`.

**Archivos modificados:**
- `internal/adapters/bff_mensajeria.go`, `internal/adapters/bff_emergencias_por_id.go`, `internal/adapters/bff_operacion_test.go`
- `go.mod`, `go.sum`, `CHANGELOG.md`

## 2026-11-04 — Cablear en el core compuesto los adjuntos y el dashboard del bff (E8-T17)

**Para quién:** el panel interno (E8-T17). El core compuesto es quien sirve el `bff` en producción; hasta ahora no compilaba contra el `bff` v0.1.14 (que amplió su `OperacionPort` con adjuntos y añadió un `ReportesPort`). Esto cierra el cableado de extremo a extremo: el dashboard y la evidencia responden en el core, no solo en el contrato.

**Resumen:** sube los pines y cablea las dos aristas nuevas en proceso.

- `BFFOperacionEnProceso` gana `ListarAdjuntos`, `CrearIntencionSubidaAdjunto` y `DescargarAdjunto`, reenviando a `operacion.Consultas()` con el presupuesto de operacion y la misma traducción de error (I3).
- Nuevo `BFFReportesEnProceso` (`internal/adapters/bff_reportes.go`): el adaptador en proceso de la arista bff → reportes, que antes no existía. `main.go` lo inyecta con `Reportes: adapters.NewBFFReportes(rep.Consultas())` en `bff.New`; `rep` ya se construía antes del `bff`. Conserva `proyeccion_al` y `stale` (DEC-033).

**Pines:** `pf-pkg-contracts` `tracer.33` → `tracer.36`; `pf-svc-bff` `v0.1.13` → `v0.1.14`; `pf-svc-operacion` `v0.1.14` → `v0.1.15`; `pf-svc-reportes` `v0.1.2` → `v0.1.4`. Sin renombres de enum; `go build` limpio.

**Cambios:**
- `internal/adapters/bff_operacion.go`: tres métodos de adjuntos.
- `internal/adapters/bff_reportes.go` (nuevo): adaptador en proceso bff → reportes.
- `cmd/core/main.go`: `Reportes:` en el `bff.New`.
- `internal/adapters/bff_operacion_test.go`: `operacionFake` gana los tres métodos de adjuntos.
- `internal/adapters/bff_reportes_test.go` (nuevo): paridad I3 del adaptador de reportes (presupuesto, timeout → Degradado, cancelación, propagación de taxonomía, conservación de frescura).
- `go.mod`/`go.sum`: los cuatro pines.

**Verificación:** `gofmt` limpio, `go build/vet ./...`, `go build ./cmd/core`, `go build -tags devauth ./...`, `go test ./...` y `arch-test` (v0.1.8) OK. La arista bff → reportes ya estaba declarada en las reglas; arch-test en verde.

**Siguiente (fuera de este PR):** el frontend sube su pin y construye el dashboard y la evidencia; el almacén real de objetos (DT-14) y el escaneo (DT-13) siguen pendientes.

**Archivos modificados:**
- `go.mod`, `go.sum`, `cmd/core/main.go`
- `internal/adapters/bff_operacion.go`, `internal/adapters/bff_reportes.go`
- `internal/adapters/bff_operacion_test.go`, `internal/adapters/bff_reportes_test.go`
- `CHANGELOG.md`

## 2026-11-03 11:54 CST — Resolver la conversación del viaje para el portal cliente (E9-T8)

**Resumen:** el adaptador en proceso bff → mensajeria traduce el viaje_id del portal cliente al conversacion_id con `ObtenerConversacionDeViaje` antes de leer o escribir. Antes el portal cliente recibía 404 en `GET/POST /api/v1/viajes/{id}/conversacion[/mensajes]` porque el viaje_id se buscaba como `conversacion.id`.

**Para quién:** para el cliente del binario de la demo (E9-T8) y para quien separe después los métodos de viaje en `bff.MensajeriaPort`: el despacho de este adaptador es provisional.

**Cambios:**
- `go.mod`/`go.sum`: mensajeria `v0.1.3 → v0.1.4` (`ConversacionesAPI.ObtenerConversacionDeViaje`, migración 0007); por MVS `api-client-go` `tracer.31 → tracer.33`. Sin renombres de enum (git-workflow §8).
- `internal/adapters/bff_mensajeria.go`: `ObtenerConversacion`, `ListarMensajes` y `EnviarMensaje` resuelven la conversación del viaje cuando la audiencia es Cliente y siguen por id; con audiencia interna el id pasa sin cambio. Un solo presupuesto `bff.PresupuestoMensajeria` y `traducirErrorMensajeria` para las dos llamadas. bff v0.1.13 usa esos tres métodos con viaje_id (rutas `/viajes/{id}/conversacion`, solo Cliente) y con conversacion_id (rutas `/conversaciones/{id}`, solo Interno); la audiencia es lo único que el adaptador ve para distinguirlos (comentario `shortcut`).
- `internal/adapters/bff_mensajeria_test.go`: `TestBFFMensajeriaConversacionDeViaje` comprueba, en los tres métodos, que el viaje_id llega como conversacion_id resuelto, que el id interno pasa sin cambio y que el 404 de la resolución se propaga sin llamar al método por id.

**Verificación:** `gofmt` limpio; `go build/vet/test ./...` con y sin `-tags devauth`; devauth ausente del build de producción; `arch-test` v0.1.8 OK. `cmd/migrate` aplica la 0007 de mensajeria (índice `reclamo_version_por_conversacion`). Smoke sobre el binario real con el arnés gate-g2, por rutas del bff: el cliente lee la conversación 200, envía 201 y lista 200; otro tenant y otro cliente del mismo tenant 404; el ejecutivo ve la conversación en la bandeja, reclama con la `version_reclamo` leída 201 y responde 201. El mismo smoke contra `pf-core-d` da 404 en las tres rutas del cliente (ver PR).

## 2026-11-03 10:40 CST — Componer el flujo interno del panel y la atención de la bandeja por el bff

**Resumen:** core cablea en proceso los ocho métodos nuevos de `bff.OperacionPort` (ETA, asignación, bitácora, incidencias, estadías) sobre `operacion.Consultas()`, y reenvía atendida y recibos de la bandeja a mensajeria. Con eso el panel interno hace el flujo operativo del viaje y la atención de la conversación por las rutas del bff, primer paso para que el bff sea la única puerta pública.

**Para quién:** para el binario de la demo y para quien cierre después el mux de respaldo de `montarRutas`: estas rutas ya no dependen de él.

**Cambios:**
- `go.mod`/`go.sum`: bff `v0.1.12 → v0.1.13`, operacion `v0.1.13 → v0.1.14` (roles de incidencia corregidos: `monitorista` ya abre incidencias) y mensajeria `v0.1.2 → v0.1.3` (`MarcarAtendida` y recibos en `ConversacionesAPI`, migración 0006). Todos tags publicados. `api-client-go` sigue en `tracer.31`. Sin renombres de enum (git-workflow §8).
- `internal/adapters/bff_operacion.go`: `FijarEta`, `AsignarRecursos`, `RegistrarEntradaBitacora`, `AbrirIncidencia`, `ResolverIncidencia`, `ListarEstadias`, `AbrirEstadia`, `CerrarEstadia`, con el mismo presupuesto y traducción de error que los existentes (I3).
- `internal/adapters/bff_mensajeria.go`: `MarcarAtendida`, `RegistrarReciboEntregado` y `RegistrarReciboLeido` reenvían a `mensajeria.ConversacionesAPI` con `bff.PresupuestoMensajeria` y `traducirErrorMensajeria`. Se borra `sinRutaEnMensajeria` (el `NoEncontrado` provisional).
- `internal/adapters/bff_operacion_test.go`: el fake implementa los ocho métodos; `TestBFFOperacionFlujoInterno` comprueba deadline y propagación del `codigo` del conflicto en cada uno.
- `internal/adapters/bff_mensajeria_test.go`: `TestBFFMensajeriaAtencionYRecibos` sustituye a la prueba del `NoEncontrado`: comprueba que los tres métodos llegan a mensajeria con id y cuerpo sin cambio y que el error categorizado se propaga.
- `cmd/core/wiring_test.go`: `POST .../asignacion` y `GET .../detalle` ya los sirve bff en el mux público; el ejemplo de ruta que cae al mux interno pasa a `PATCH .../asignacion` (reasignar, solo operacion). `montarRutas` no cambia.

**Verificación:** `gofmt` limpio; `go build/vet/test ./...` con y sin `-tags devauth`; devauth ausente del build de producción; `arch-test` v0.1.8 OK. `cmd/migrate` aplica la 0006 de mensajeria (`conversacion.leido_ai27_hasta`, `auditoria_mensajeria.registro_id` y sus índices). Smoke sobre el binario real con el arnés gate-g2, solo por rutas del bff: flujo interno completo (incidencia abierta por `monitorista` 201), bandeja (listar, reclamar 201, enviar 201, atendida 204, recibos 204, reclamo con versión obsoleta 409) y sujeto no-UUID en mensajeria 401 sin texto SQL (ver PR).

## 2026-11-02 17:10 CST — Componer bff v0.1.12 (tracer.31) y arrancar sin choque de rutas

**Resumen:** core sube bff a v0.1.12 y cablea en proceso los puertos nuevos: transición de viaje, tipos de servicio y la conversación/bandeja de mensajeria. Además corrige un panic de arranque que ya existía en `pf-core-d`: el binario compuesto no llegaba a `/ready`.

**Para quién:** para el binario de la demo (DEC-088). Con esto corren en core la transición interna (E8-T14) y del cliente (E9-T7), el selector de tipos de servicio (E9-T4), la conversación del cliente (E9-T8) y la bandeja interna (E8-T15).

**Cambios:**
- `go.mod`/`go.sum`: bff `v0.1.4→v0.1.12`; operacion `v0.1.8→v0.1.13` (fachada `TransicionarViaje`, PR #71); mensajeria `v0.1.0→v0.1.2` (fachada `ConversacionesAPI`, PR #33); por MVS `api-client-go` `tracer.21→tracer.31` y platform-go `v0.1.6→v0.1.8`. catalogos sigue en v0.1.3. Sin renombres de enum (§8).
- `internal/adapters/bff_operacion.go`: `TransicionarViaje` sobre `Consultas().TransicionarViaje`.
- `internal/adapters/bff_catalogos.go`: `ListarTiposServicio` sobre `ListarValoresCatalogo` con `catclient.TiposServicio` fijo, mismo presupuesto y traducción que `ListarClientes`.
- `internal/adapters/bff_mensajeria.go`: el adaptador también recibe `mensajeria.ConversacionesAPI` (cambia `NewBFFMensajeria`). Agrega los 10 métodos de conversación y bandeja. `MarcarAtendida`, `RegistrarReciboEntregado` y `RegistrarReciboLeido` devuelven el mismo `NoEncontrado` que el adaptador HTTP de bff traduce del 404 de mensajeria (mensajeria no tiene esos casos de uso). Los ids pasan sin cambio, igual que por HTTP.
- `cmd/core/main.go`: `montarRutas` monta bff en el mux público y el resto de los módulos en un mux interno al que el público delega. Antes todo iba a un solo mux y los patrones repetidos (bff vs catalogos `GET /api/v1/clientes`; vs operacion `POST .../transiciones`; vs mensajeria conversación/bandeja/emergencias) hacían panic al arrancar.
- `.github/workflows/ci.yml`: arch-test `v0.1.7→v0.1.8`, la misma versión de platform-go que ahora fija `go.mod`.
- Tests: `bff_catalogos_test.go` y `bff_mensajeria_test.go` (nuevos), `TestBFFOperacionTransicionarViaje`, `TestMontarRutasBFFGanaSinPanic`.

**Verificación:** `gofmt` limpio; `go build/vet/test ./...` con y sin `-tags devauth`; devauth ausente del build de producción; `arch-test` v0.1.7 y v0.1.8 OK. Smoke con el arnés gate-g2 (Postgres + `cmd/migrate` + NATS, auth real con jwtmint): `/ready` 200, crear viaje 201, transición interna 200 (a CANCELADO) y 409 (arista inválida), tipos de servicio 200, bandeja 404 de mensajeria (sujeto sin pool), atendida/recibo 404.

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
