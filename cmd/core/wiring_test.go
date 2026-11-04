package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

// TestMontarRutasSoloBFF: con los módulos reales, montar las rutas no hace panic
// y el mux público sirve solo los patrones de bff (única puerta pública). bff y
// los módulos publican patrones repetidos (GET /api/v1/clientes, POST
// .../transiciones, la bandeja); como solo bff monta, no chocan.
func TestMontarRutasSoloBFF(t *testing.T) {
	publico := muxPublico(t)

	casos := []struct{ metodo, ruta, patron string }{
		{"GET", "/api/v1/clientes", "GET /api/v1/clientes"},
		{"POST", "/api/v1/viajes/x/transiciones", "POST /api/v1/viajes/{id}/transiciones"},
		{"GET", "/api/v1/catalogos/tipos-servicio", "GET /api/v1/catalogos/tipos-servicio"},
		{"GET", "/api/v1/bandeja/trabajo", "GET /api/v1/bandeja/trabajo"},
		{"POST", "/api/v1/conversaciones/x/atendida", "POST /api/v1/conversaciones/{id}/atendida"},
		{"POST", "/api/v1/viajes/x/asignacion", "POST /api/v1/viajes/{id}/asignacion"},
		{"GET", "/api/v1/viajes/x/detalle", "GET /api/v1/viajes/{id}/detalle"},
		// Flujo interno, grupo 2: ya lo sirve bff.
		{"GET", "/api/v1/unidades", "GET /api/v1/unidades"},
		{"PATCH", "/api/v1/viajes/x/fecha-hora", "PATCH /api/v1/viajes/{id}/fecha-hora"},
		{"GET", "/api/v1/viajes/x/incidencias", "GET /api/v1/viajes/{id}/incidencias"},
		{"GET", "/api/v1/viajes/x/ficha", "GET /api/v1/viajes/{id}/ficha"},
		{"PATCH", "/api/v1/viajes/x/ficha", "PATCH /api/v1/viajes/{id}/ficha"},
		{"GET", "/api/v1/gps", "GET /api/v1/gps"},
	}
	for _, c := range casos {
		_, patron := publico.Handler(httptest.NewRequest(c.metodo, c.ruta, nil))
		if patron != c.patron {
			t.Errorf("%s %s: patrón = %q; se esperaba %q", c.metodo, c.ruta, patron, c.patron)
		}
	}
}

// TestRutaNativaSinBFFEs404: una ruta nativa de un módulo que bff no sirve ya no
// es pública. Antes caía al mux interno y llegaba al módulo; ahora el mux público
// responde 404 sin tocarlo.
func TestRutaNativaSinBFFEs404(t *testing.T) {
	publico := muxPublico(t)

	nativas := []struct{ metodo, ruta string }{
		{"POST", "/api/v1/viajes/00000000-0000-0000-0000-000000000000/descargas"}, // operacion: registrarDescargaViaje
		{"POST", "/api/v1/vehiculos"},                    // catalogos: alta de vehículo
		{"GET", "/api/v1/custodios"},                     // catalogos: listado de custodios
		{"GET", "/api/v1/catalogos-tipo/tipos_servicio"}, // catalogos: valores por tipo
	}
	for _, n := range nativas {
		rec := httptest.NewRecorder()
		publico.ServeHTTP(rec, httptest.NewRequest(n.metodo, n.ruta, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d; se esperaba 404 (no es ruta de bff)", n.metodo, n.ruta, rec.Code)
		}
	}
}

// muxPublico construye los módulos reales y monta sus rutas como run().
func muxPublico(t *testing.T) *http.ServeMux {
	t.Helper()
	b := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	modules, _, err := construirModulos(pools{}, b, b, b, autenticadorNulo{}, logger)
	if err != nil {
		t.Fatalf("construirModulos: %v", err)
	}
	publico := http.NewServeMux()
	montarRutas(publico, modules)
	return publico
}
