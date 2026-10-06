# CHANGELOG

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
