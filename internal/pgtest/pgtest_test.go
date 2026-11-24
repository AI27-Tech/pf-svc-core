package pgtest

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestExigirDesechable(t *testing.T) {
	sin := func(string) string { return "" }
	if err := exigirDesechable(sin); err == nil || !strings.Contains(err.Error(), EnvDesechable) {
		t.Fatalf("sin %s debía fallar nombrando la variable; err = %v", EnvDesechable, err)
	}
	if err := exigirDesechable(func(string) string { return "1" }); err != nil {
		t.Fatalf("con %s=1 no debía fallar: %v", EnvDesechable, err)
	}
}

// TestRevisarBasesRechazaBaseAjena crea una base ajena en el cluster de pruebas y
// comprueba que la guarda la ve; con solo bases core_it_ y del sistema, pasa.
func TestRevisarBasesRechazaBaseAjena(t *testing.T) {
	raw := os.Getenv(EnvAdmin)
	if raw == "" {
		t.Skipf("%s no definido", EnvAdmin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	if err := revisarBases(ctx, conn); err != nil {
		t.Fatalf("cluster limpio: la guarda no debía fallar: %v", err)
	}
	const ajena = "base_ajena_pgtest"
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+ajena); err != nil {
		t.Fatalf("crear base ajena: %v", err)
	}
	// Cleanup corre en orden inverso: esta baja la base antes de cerrar conn.
	t.Cleanup(func() { _, _ = conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+ajena+" WITH (FORCE)") })

	if err := revisarBases(ctx, conn); err == nil || !strings.Contains(err.Error(), ajena) {
		t.Fatalf("con una base ajena la guarda debía fallar nombrándola; err = %v", err)
	}
}
