// Package pgtest crea bases de datos efímeras para las pruebas de integración de
// core. Solo lo importan archivos _test.go. Sin CORE_TEST_PG_ADMIN_URL las
// pruebas se saltan; el CI del repo lo define. Con la URL pero sin la declaración
// de cluster desechable (EnvDesechable) o con bases ajenas en el cluster, falla.
package pgtest

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// EnvAdmin es la URL (postgres://usuario@host/base?...) de un rol administrador
// del cluster de pruebas: crea bases, esquemas y roles. Los roles de app se
// conectan con la misma URL cambiando solo el usuario, así que el cluster debe
// confiar en ellos (auth trust) porque sus roles se crean sin contraseña.
const EnvAdmin = "CORE_TEST_PG_ADMIN_URL"

// EnvDesechable (=1) declara que el cluster de EnvAdmin es solo para pruebas.
// Nueva resetea la configuración de los roles op_*/svc_* de TODO el cluster, así
// que sin esta declaración se niega a tocar nada.
const EnvDesechable = "CORE_TEST_PG_DESECHABLE"

// prefijoBase es el de las bases efímeras que crea Nueva; son las únicas, además
// de las del sistema, que puede haber en un cluster desechable.
const prefijoBase = "core_it_"

// exigirDesechable falla si el autor de la prueba no declaró el cluster desechable.
func exigirDesechable(env func(string) string) error {
	if env(EnvDesechable) != "1" {
		return fmt.Errorf("%s no es 1: Nueva resetea la configuración de los roles op_*/svc_* de todo el cluster; "+
			"solo se corre contra un Postgres dedicado a pruebas (ver README)", EnvDesechable)
	}
	return nil
}

// revisarBases falla si el cluster tiene alguna base que no sea del sistema ni
// de pgtest: señal de un Postgres de desarrollo cuyos roles no se deben resetear.
func revisarBases(ctx context.Context, conn *pgx.Conn) error {
	filas, err := conn.Query(ctx, `SELECT datname FROM pg_database
		WHERE datname NOT IN ('postgres','template0','template1') AND datname NOT LIKE $1`, prefijoBase+"%")
	if err != nil {
		return fmt.Errorf("consultar pg_database: %w", err)
	}
	ajenas, err := pgx.CollectRows(filas, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("leer pg_database: %w", err)
	}
	if len(ajenas) > 0 {
		return fmt.Errorf("el cluster no es dedicado a pruebas: tiene bases ajenas %v; "+
			"apunta %s a un Postgres desechable", ajenas, EnvAdmin)
	}
	return nil
}

// Base es una base efímera con su credencial administrativa.
type Base struct {
	Nombre   string
	adminURL *url.URL
}

// Nueva crea una base vacía y la elimina al terminar la prueba. Salta la prueba si
// no hay cluster configurado.
func Nueva(t *testing.T, env func(string) string) *Base {
	t.Helper()
	raw := env(EnvAdmin)
	if raw == "" {
		t.Skipf("%s no definido: prueba de integración contra Postgres omitida", EnvAdmin)
	}
	if err := exigirDesechable(env); err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("pgtest: %s inválida: %v", EnvAdmin, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatalf("pgtest: conectar como admin: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if err := revisarBases(ctx, conn); err != nil {
		t.Fatalf("pgtest: %v", err)
	}

	// Los roles son del cluster, no de la base: su search_path sobrevive a las bases
	// borradas. Se limpia el de corridas previas para que la prueba demuestre lo
	// que fija cmd/migrate y no lo que quedó de otra corrida.
	if _, err := conn.Exec(ctx, `DO $$ DECLARE r text; BEGIN
		FOR r IN SELECT rolname FROM pg_roles WHERE rolname ~ '^(op|svc)_' AND rolconfig IS NOT NULL LOOP
			EXECUTE format('ALTER ROLE %I RESET ALL', r);
		END LOOP; END $$`); err != nil {
		t.Fatalf("pgtest: limpiar la configuración de roles: %v", err)
	}

	nombre := fmt.Sprintf("%s%d", prefijoBase, time.Now().UnixNano())
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+nombre); err != nil {
		t.Fatalf("pgtest: crear base: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		k, err := pgx.Connect(c, raw)
		if err != nil {
			return
		}
		defer func() { _ = k.Close(c) }()
		_, _ = k.Exec(c, "DROP DATABASE IF EXISTS "+nombre+" WITH (FORCE)")
	})
	return &Base{Nombre: nombre, adminURL: u}
}

// URL devuelve la URL de la base con el usuario dado; "" es el administrador.
func (b *Base) URL(rol string) string {
	u := *b.adminURL
	u.Path = "/" + b.Nombre
	if rol != "" {
		u.User = url.User(rol)
	}
	return u.String()
}

// Conn abre una conexión administrativa a la base; se cierra sola al terminar.
func (b *Base) Conn(t *testing.T) *pgx.Conn {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, b.URL(""))
	if err != nil {
		t.Fatalf("pgtest: conectar a %s: %v", b.Nombre, err)
	}
	t.Cleanup(func() { _ = c.Close(ctx) })
	return c
}
