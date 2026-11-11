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
	identidad "github.com/AI27-Tech/pf-svc-identidad"
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
	modules, idn, err := construirModulos(pools{}, b, sub, b, autenticadorNulo{}, idpDePrueba(t), nil, logger)
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

// TestCatalogosAportaRelay (DT-22): core le pasa el bus a catalogos, así que su
// relay de outbox entra en la recolección de Workers y catalogo.actualizado
// llega a los consumidores de operacion y mensajeria.
func TestCatalogosAportaRelay(t *testing.T) {
	nombres := nombresDeWorkers(t, busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{}), true)
	if !contieneSubcadena(nombres, "outbox-relay:catalogos") {
		t.Fatalf("no se encontró el relay de outbox de catalogos entre %v", nombres)
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
		// Roster y flota (E8-T21, tracer.39): ya los sirve bff.
		{"POST", "/api/v1/unidades", "POST /api/v1/unidades"},
		{"GET", "/api/v1/unidades/x", "GET /api/v1/unidades/{id}"},
		{"PATCH", "/api/v1/unidades/x", "PATCH /api/v1/unidades/{id}"},
		{"POST", "/api/v1/vehiculos", "POST /api/v1/vehiculos"},
		{"PATCH", "/api/v1/vehiculos/x", "PATCH /api/v1/vehiculos/{id}"},
		{"GET", "/api/v1/custodios", "GET /api/v1/custodios"},
		{"POST", "/api/v1/custodios", "POST /api/v1/custodios"},
		// Sesión: la sirve el bff sobre la fachada en proceso de identidad (DEC-137).
		{"POST", "/api/v1/sesion", "POST /api/v1/sesion"},
		{"POST", "/api/v1/sesion/refresh", "POST /api/v1/sesion/refresh"},
		{"DELETE", "/api/v1/sesion", "DELETE /api/v1/sesion"},
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
		{"GET", "/api/v1/vehiculos/00000000-0000-0000-0000-000000000000/aptitud"}, // catalogos: aptitud de vehículo
		{"POST", "/api/v1/contratos"},                    // catalogos: alta de contrato
		{"GET", "/api/v1/catalogos-tipo/tipos_servicio"}, // catalogos: valores por tipo
		// identidad tampoco es pública (DEC-137).
		{"POST", "/api/v1/sesiones"},
		{"POST", "/api/v1/sesiones/refresh"},
		{"DELETE", "/api/v1/sesiones/00000000-0000-0000-0000-000000000000"},
		{"GET", "/api/v1/sesiones/revocadas"},
		{"GET", "/api/v1/sesiones/00000000-0000-0000-0000-000000000000"},
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
	publico, _ := muxPublicoEIdentidad(t)
	return publico
}

// muxPublicoEIdentidad es muxPublico y además devuelve el módulo identidad.
func muxPublicoEIdentidad(t *testing.T) (*http.ServeMux, *identidad.Module) {
	t.Helper()
	b := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	modules, idn, err := construirModulos(pools{}, b, b, b, autenticadorNulo{}, idpDePrueba(t), nil, logger)
	if err != nil {
		t.Fatalf("construirModulos: %v", err)
	}
	publico := http.NewServeMux()
	montarRutas(publico, modules)
	return publico, idn
}

// TestAdministracionConIDPAdmin (tracer.40, E8-T19): core le pasa a identidad la
// administración del adaptador de Keycloak, y el bff sirve la administración de
// usuarios y organizaciones sobre la fachada en proceso.
//
// Que identidad recibió IDPAdmin se observa en su Routes, que solo monta el
// gobierno con IDPAdmin; ese mux es de la prueba, en core no se monta (DEC-137).
// Las rutas del bff deben responder algo distinto de 404 (no montada) y de 501
// (sin cablear); con el autenticador nulo no hay sesión, así que no llegan a
// tocar la base.
func TestAdministracionConIDPAdmin(t *testing.T) {
	publico, idn := muxPublicoEIdentidad(t)

	nativo := http.NewServeMux()
	idn.Routes(nativo, autenticadorNulo{}, nil)
	if _, patron := nativo.Handler(httptest.NewRequest("POST", "/api/v1/sujetos", nil)); patron != "POST /api/v1/sujetos" {
		t.Fatalf("identidad no montó el gobierno (patrón %q): IDPAdmin no llegó a identidad.Deps", patron)
	}

	const id = "00000000-0000-0000-0000-000000000001"
	rutas := []struct{ metodo, ruta string }{
		{"GET", "/api/v1/sujetos"},
		{"POST", "/api/v1/sujetos"},
		{"GET", "/api/v1/sujetos/" + id},
		{"PATCH", "/api/v1/sujetos/" + id},
		{"DELETE", "/api/v1/sujetos/" + id},
		{"POST", "/api/v1/sujetos/" + id + "/reactivacion"},
		{"PUT", "/api/v1/sujetos/" + id + "/rol"},
		{"POST", "/api/v1/sujetos/" + id + "/revocaciones"},
		{"GET", "/api/v1/sujetos/" + id + "/alcance"},
		{"PUT", "/api/v1/sujetos/" + id + "/alcance/unidades"},
		{"PUT", "/api/v1/sujetos/" + id + "/alcance/subordinados"},
		{"GET", "/api/v1/organizaciones"},
		{"POST", "/api/v1/organizaciones"},
		{"PATCH", "/api/v1/organizaciones/" + id},
		{"DELETE", "/api/v1/organizaciones/" + id},
	}
	for _, r := range rutas {
		rec := httptest.NewRecorder()
		publico.ServeHTTP(rec, httptest.NewRequest(r.metodo, r.ruta, strings.NewReader("{}")))
		if rec.Code == http.StatusNotFound || rec.Code == http.StatusNotImplemented {
			t.Errorf("%s %s = %d; se esperaba una ruta montada y cableada", r.metodo, r.ruta, rec.Code)
		}
	}
}
