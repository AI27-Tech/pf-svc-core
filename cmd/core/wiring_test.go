package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/bus"
	busmem "github.com/AI27-Tech/pf-pkg-platform-go/bus/memory"
	"github.com/AI27-Tech/pf-svc-core/internal/busregistry"
)

// autenticadorNulo satisface auth.Authenticator sin verificar nada. Basta para
// construir los módulos en un test de cableado: no se ejercita ninguna ruta.
type autenticadorNulo struct{}

func (autenticadorNulo) VerifyToken(context.Context, string) (*auth.Claims, error) {
	return &auth.Claims{}, nil
}

// nombresDeWorkers recolecta los Name() de los workers de todos los módulos,
// igual que run() antes de pasarlos a RunGroup. En RunGroup cada uno se registra
// en readiness como "worker:"+Name().
//
// El memory.Bus hace de Publisher, Subscriber y Broadcaster a la vez, así que el
// mismo objeto se pasa por los tres puertos. Pools vacíos: ningún New toca su
// pool al construir (se verifica en los repos de cada módulo). Con conSubscriber
// en false se pasa un Subscriber nil, que representa el arranque sin consumo de
// bus en reportes.
func nombresDeWorkers(t *testing.T, b *busmem.Bus, conSubscriber bool) []string {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var sub bus.Subscriber
	if conSubscriber {
		sub = b
	}
	modules, idn, err := construirModulos(pools{}, b, sub, b, autenticadorNulo{}, logger)
	if err != nil {
		t.Fatalf("construirModulos: %v", err)
	}
	var nombres []string
	for _, m := range modules {
		for _, w := range m.Workers() {
			nombres = append(nombres, w.Name())
		}
	}
	for _, w := range idn.Workers() {
		nombres = append(nombres, w.Name())
	}
	return nombres
}

// TestReportesConsumeConSubscriber verifica el cableado del Paso 3 (E6-T1): con
// un Subscriber inyectado, core construye reportes de modo que su consumidor
// durable de proyecciones entra en la recolección de Workers; sin Subscriber,
// reportes no aporta ese worker. Se afirma por diferencia para no acoplarse al
// nombre que cada transporte da al worker (NATS: bus-durable:reportes; memoria:
// reportes).
func TestReportesConsumeConSubscriber(t *testing.T) {
	busCon := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})
	busSin := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})

	con := nombresDeWorkers(t, busCon, true)
	sin := nombresDeWorkers(t, busSin, false)

	if len(con)-len(sin) != 1 {
		t.Fatalf("con Subscriber se esperaba 1 worker más (el consumidor de reportes); con=%v sin=%v", con, sin)
	}
	// El worker nuevo es el consumidor durable de reportes, suscrito con
	// durableName "reportes" (Paso 2). El memory.Bus lo nombra "reportes".
	if !contiene(con, "reportes") {
		t.Fatalf("no se encontró el consumidor de reportes entre %v", con)
	}
	if contiene(sin, "reportes") {
		t.Fatalf("sin Subscriber, reportes no debía aportar un worker; sin=%v", sin)
	}
}

func contiene(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
