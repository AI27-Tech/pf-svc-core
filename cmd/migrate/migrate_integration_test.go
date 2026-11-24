package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/AI27-Tech/pf-svc-core/internal/pgtest"
)

// TestMigrateDosVecesYLlegaLoNuevo: el bootstrap corre DOS veces sobre una base
// vacía y sale sin error (se reaplica en cada despliegue; DT-25), y deja las
// migraciones de la ola de solicitudes, el rol op_solicitudes con su search_path
// y los grants de la excepción de ADR 39.
func TestMigrateDosVecesYLlegaLoNuevo(t *testing.T) {
	base := pgtest.Nueva(t, os.Getenv)
	t.Setenv(envDBAdmin, base.URL(""))
	for i := 1; i <= 2; i++ {
		if err := run(); err != nil {
			t.Fatalf("corrida %d de migrate: %v", i, err)
		}
	}

	ctx := context.Background()
	c := base.Conn(t)
	// existe evalúa una expresión booleana (EXISTS(...) o has_*_privilege(...)).
	existe := func(expr string, args ...any) bool {
		t.Helper()
		var ok bool
		if err := c.QueryRow(ctx, "SELECT "+expr, args...).Scan(&ok); err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		return ok
	}

	// Lo nuevo de la ola: tablas de solicitudes, columna de viajes/0008 ya
	// existente en V2 y la columna de mensajeria/0011-0013.
	for _, tabla := range []string{
		"solicitudes.solicitud", "solicitudes.transicion_solicitud", "solicitudes.outbox",
		"solicitudes.cotizacion", "solicitudes.pago_solicitud", "viajes.viaje", "viajes.destino", "viajes.outbox",
		"mensajeria.conversacion",
	} {
		if !existe("EXISTS (SELECT 1 FROM pg_tables WHERE schemaname||'.'||tablename = $1)", tabla) {
			t.Errorf("falta la tabla %s tras migrate", tabla)
		}
	}

	// Los grants de la excepción (ADR 39, viajes/0008) y de auditoria/0003.
	for _, g := range []struct{ tabla, priv string }{
		{"viajes.viaje", "INSERT"}, {"viajes.destino", "INSERT"}, {"viajes.outbox", "INSERT"},
	} {
		if !existe("has_table_privilege('op_solicitudes', $1, $2)", g.tabla, g.priv) {
			t.Errorf("op_solicitudes sin %s sobre %s", g.priv, g.tabla)
		}
	}
	if existe("has_table_privilege('op_solicitudes', 'viajes.viaje', 'UPDATE')") {
		t.Error("op_solicitudes tiene UPDATE sobre viajes.viaje: la excepción de ADR 39 es solo INSERT")
	}
	if !existe("has_schema_privilege('op_solicitudes', 'auditoria', 'USAGE')") {
		t.Error("op_solicitudes sin USAGE sobre auditoria (auditoria/0003)")
	}

	// search_path del rol: lo fija el bootstrap, no la sesión.
	var cfg []string
	if err := c.QueryRow(ctx, `SELECT coalesce(rolconfig, '{}') FROM pg_roles WHERE rolname = 'op_solicitudes'`).Scan(&cfg); err != nil {
		t.Fatalf("leer rolconfig: %v", err)
	}
	if len(cfg) != 1 || cfg[0] != "search_path=solicitudes" {
		t.Errorf("rolconfig de op_solicitudes = %v; se esperaba [search_path=solicitudes]", cfg)
	}

	// Y se comprueba con el rol de verdad: al conectar como op_solicitudes, la
	// tabla `outbox` SIN calificar (la que usan el relay y el publicador de la
	// plataforma) resuelve a solicitudes.outbox. Sin el search_path del rol falla
	// con `relation "outbox" does not exist`.
	rc, err := pgx.Connect(ctx, base.URL("op_solicitudes"))
	if err != nil {
		t.Fatalf("conectar como op_solicitudes: %v", err)
	}
	defer func() { _ = rc.Close(ctx) }()
	var schema string
	if err := rc.QueryRow(ctx, `SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE c.oid = to_regclass('outbox')`).Scan(&schema); err != nil || schema != "solicitudes" {
		t.Errorf("`outbox` sin calificar resuelve a %q (%v); se esperaba solicitudes", schema, err)
	}
}
