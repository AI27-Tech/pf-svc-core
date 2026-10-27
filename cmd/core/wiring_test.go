package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
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
// reportes no aporta ese worker. El nombre se comprueba por substring para no
// acoplarse al prefijo del transporte.
//
// NOTA: con la llegada de E5-T5, operacion también abre un consumidor durable
// cuando hay Subscriber, así que el delta TOTAL de workers con/sin Subscriber ya
// no es 1. Por eso la aserción es por consumidor (presencia/ausencia del de
// reportes), no sobre el conteo total; el de operacion se cubre en
// TestOperacionConsumeCatalogosConSubscriber.
func TestReportesConsumeConSubscriber(t *testing.T) {
	busCon := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})
	busSin := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})

	con := nombresDeWorkers(t, busCon, true)
	sin := nombresDeWorkers(t, busSin, false)

	// El worker del consumidor durable de reportes se suscribe con durableName
	// "reportes" (Paso 2). Se busca por substring para no acoplarse al prefijo que
	// cada transporte da al nombre (NATS y el bus en memoria usan
	// "bus-durable:reportes"; una variante vieja usaba solo "reportes").
	if !contieneSubcadena(con, "reportes") {
		t.Fatalf("no se encontró el consumidor de reportes entre %v", con)
	}
	if contieneSubcadena(sin, "reportes") {
		t.Fatalf("sin Subscriber, reportes no debía aportar un worker; sin=%v", sin)
	}
}

// TestOperacionConsumeCatalogosConSubscriber verifica el cableado de E5-T5: con
// un Subscriber inyectado, core construye operacion de modo que su consumidor
// durable de catalogo.actualizado (que invalida la caché de aptitud) entra en la
// recolección de Workers; sin Subscriber, operacion no aporta ese worker. Se
// busca por substring ("operacion-catalogos") para no acoplarse al prefijo del
// transporte ("bus-durable:operacion-catalogos").
func TestOperacionConsumeCatalogosConSubscriber(t *testing.T) {
	busCon := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})
	busSin := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})

	con := nombresDeWorkers(t, busCon, true)
	sin := nombresDeWorkers(t, busSin, false)

	if !contieneSubcadena(con, "operacion-catalogos") {
		t.Fatalf("no se encontró el consumidor de catálogos de operacion entre %v", con)
	}
	if contieneSubcadena(sin, "operacion-catalogos") {
		t.Fatalf("sin Subscriber, operacion no debía aportar el consumidor de catálogos; sin=%v", sin)
	}
}

func contieneSubcadena(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}
