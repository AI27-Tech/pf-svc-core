// Command migrate es el BOOTSTRAP de base de datos del deployable `core`
// (DEC-096, E0-T7). Corre UNA vez en el despliegue, con credencial
// administrativa, ANTES de que arranque `cmd/core`. Crea los esquemas, los
// roles y los grants de cada módulo aplicando sus migraciones embebidas, y fija
// el search_path de los roles que el relay/outbox/inbox necesitan.
//
// Por qué un binario aparte y no dentro de cmd/core (DEC-096): los 0001 de cada
// módulo hacen CREATE SCHEMA / CREATE ROLE, privilegio administrativo que los
// pools de app de core (privilegio mínimo, I1) no tienen y no deben tener. Meter
// esa credencial en el proceso del servicio la dejaría viva en cada arranque;
// este binario la confina al paso de despliegue y termina. `cmd/core` arranca
// después con sus roles de app y encuentra todo hecho.
//
// Es el mismo trabajo que hoy hace scripts/gate-g2/entorno.sh en bash, con el
// aplicador reutilizable migrate de pf-pkg-platform-go (dos fases, ADR 33).
package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/AI27-Tech/pf-pkg-platform-go/migrate"
	"github.com/jackc/pgx/v5"

	catalogosmig "github.com/AI27-Tech/pf-svc-catalogos/migrations"
	identidadmig "github.com/AI27-Tech/pf-svc-identidad/migrations"
	mensajeriamig "github.com/AI27-Tech/pf-svc-mensajeria/migrations"
	operacionmig "github.com/AI27-Tech/pf-svc-operacion/migrations"
	reportesmig "github.com/AI27-Tech/pf-svc-reportes/migrations"
)

// envDBAdmin es la credencial ADMINISTRATIVA del bootstrap: un rol con permiso
// de CREATE SCHEMA y CREATE ROLE (p. ej. el dueño de la base del despliegue). No
// es ninguno de los roles de app que core abre como pools (DEC-096).
const envDBAdmin = "DB_ADMIN_URL"

// searchPathPorRol fija el search_path de los roles cuyo relay/outbox/inbox de
// platform-go consulta tablas sin calificar el esquema (DEC-096 nota 3; hoy lo
// hace sql/roles.sql del gate). Sin esto, esos INSERT/SELECT no resuelven en
// runtime aunque las tablas existan.
//   - svc_identidad: su relay drena identidad.outbox.
//   - op_viajes / op_asignacion: el outbox único de operacion vive en viajes
//     (ADR 35); ambos lo consultan sin calificar (op_asignacion por la excepción
//     de GRANT).
//   - svc_reportes: su consumidor registra el inbox con inbox.Registrar, que
//     inserta en inbox_evento sin calificar el esquema.
var searchPathPorRol = []struct{ rol, esquema string }{
	{"svc_identidad", "identidad"},
	{"op_viajes", "viajes"},
	{"op_asignacion", "viajes"},
	{"svc_reportes", "reportes"},
	{"svc_mensajeria", "mensajeria"},
}

func main() {
	if err := run(); err != nil {
		slog.Error("migrate (bootstrap de core) terminó con error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dsn := os.Getenv(envDBAdmin)
	if dsn == "" {
		return fmt.Errorf("migrate: falta %s (credencial administrativa del bootstrap)", envDBAdmin)
	}

	// Una sola conexión administrativa aplica TODO. El bootstrap no usa pools por
	// rol: el ResolverEjecutor devuelve esta misma conexión para cualquier rol,
	// porque es la credencial admin la que crea esquemas y roles (DEC-096). La
	// pertenencia de cada esquema a su rol la fijan los propios .sql (cada 0001
	// hace CREATE ROLE ... y los grants), no la conexión con que se aplican.
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("migrate: conectar con la credencial admin: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	resolver := func(rol string) (migrate.Ejecutor, error) {
		return conn, nil
	}

	// Orden entre módulos: las hojas del grafo (identidad, catalogos) antes que
	// operacion y reportes. No hay grants que crucen de un módulo a otro por SQL
	// (las aristas entre módulos son por evento o HTTP, no por GRANT cruzado;
	// la única excepción de GRANT de operacion es interna, entre sus 5 esquemas),
	// así que el orden entre módulos no es estricto; se aplica de hojas a
	// terminal por claridad.
	modulos := []migrate.Modulo{
		unEsquema("identidad", "svc_identidad", identidadmig.FS),
		unEsquema("catalogos", "svc_catalogos", catalogosmig.FS),
		moduloOperacion(),
		unEsquema("reportes", "svc_reportes", reportesmig.FS),
		unEsquema("mensajeria", "svc_mensajeria", mensajeriamig.FS),
	}

	for _, m := range modulos {
		logger.Info("aplicando migraciones", slog.String("modulo", m.Nombre))
		if err := migrate.Aplicar(ctx, m, resolver); err != nil {
			return err
		}
	}

	// search_path por rol: tras crear todos los roles (los 0001 ya corrieron).
	for _, sp := range searchPathPorRol {
		if _, err := conn.Exec(ctx, fmt.Sprintf("ALTER ROLE %s SET search_path TO %s", sp.rol, sp.esquema)); err != nil {
			return fmt.Errorf("migrate: fijar search_path de %s: %w", sp.rol, err)
		}
	}

	logger.Info("bootstrap de base de datos completo", slog.Int("modulos", len(modulos)))
	return nil
}

// unEsquema arma un migrate.Modulo de un solo esquema (catalogos, identidad,
// reportes): su FS embebido, su rol dueño, y el Dir que coincide con el nombre.
func unEsquema(nombre, rol string, fsys embed.FS) migrate.Modulo {
	return migrate.Modulo{
		Nombre: nombre,
		Esquemas: []migrate.Esquema{
			{Nombre: nombre, Rol: rol, FS: fsys, Dir: nombre},
		},
	}
}

// moduloOperacion arma el migrate.Modulo de operacion: sus cinco esquemas en el
// Orden que exporta el módulo, cada uno con su rol dueño de RolPorEsquema. Las
// dos fases de migrate.Aplicar reproducen la excepción de GRANT de op_asignacion
// (ADR 33): primero los 0001 de los cinco, luego los grants.
func moduloOperacion() migrate.Modulo {
	esquemas := make([]migrate.Esquema, 0, len(operacionmig.Orden))
	for _, nombre := range operacionmig.Orden {
		esquemas = append(esquemas, migrate.Esquema{
			Nombre: nombre,
			Rol:    operacionmig.RolPorEsquema[nombre],
			FS:     operacionmig.FS,
			Dir:    nombre,
		})
	}
	return migrate.Modulo{Nombre: "operacion", Esquemas: esquemas}
}
