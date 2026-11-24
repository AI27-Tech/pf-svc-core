// Package pgtest crea bases de datos efímeras para las pruebas de integración de
// core. Solo lo importan archivos _test.go. Sin CORE_TEST_PG_ADMIN_URL las
// pruebas se saltan (CI sin Postgres), igual que las de los servicios.
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

	// Los roles son del cluster, no de la base: su search_path sobrevive a las bases
	// borradas. Se limpia el de corridas previas para que la prueba demuestre lo
	// que fija cmd/migrate y no lo que quedó de otra corrida.
	if _, err := conn.Exec(ctx, `DO $$ DECLARE r text; BEGIN
		FOR r IN SELECT rolname FROM pg_roles WHERE rolname ~ '^(op|svc)_' AND rolconfig IS NOT NULL LOOP
			EXECUTE format('ALTER ROLE %I RESET ALL', r);
		END LOOP; END $$`); err != nil {
		t.Fatalf("pgtest: limpiar la configuración de roles: %v", err)
	}

	nombre := fmt.Sprintf("core_it_%d", time.Now().UnixNano())
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
