//go:build devauth

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	busmem "github.com/AI27-Tech/pf-pkg-platform-go/bus/memory"
	"github.com/AI27-Tech/pf-pkg-platform-go/server"
	"github.com/AI27-Tech/pf-svc-core/internal/busregistry"
	"github.com/AI27-Tech/pf-svc-core/internal/devauth"
	"github.com/AI27-Tech/pf-svc-core/internal/pgtest"
	"github.com/google/uuid"
)

// El humo de punta a punta corre las costuras de la ola de solicitudes juntas por
// primera vez: bff → adaptadores en proceso → operacion (módulo solicitudes) →
// outbox → bus en memoria → mensajeria, sobre Postgres real, con los pools por
// rol de abrirPools y las migraciones de cmd/migrate. Necesita CORE_TEST_PG_ADMIN_URL
// (ver internal/pgtest) y el tag devauth (usa los tokens del stub).

// tenantAlfa es el tenant UUID del cliente de la prueba. Los tenants del stub
// ("tenant-dev-alfa") no son UUID y operacion los rechaza (403 "organización
// válida") y su sujeto ("rol@tenant") tampoco es UUID, que mensajeria exige
// (boundary.Claims: "sin claims"). El humo delega en devauth y sustituye ambos.
var tenantAlfa = uuid.MustParse("00000000-0000-4000-8000-0000000000a1")

type devauthConTenantUUID struct{ *devauth.Stub }

func (d devauthConTenantUUID) VerifyToken(ctx context.Context, token string) (*auth.Claims, error) {
	c, err := d.Stub.VerifyToken(ctx, token)
	if err != nil {
		return nil, err
	}
	c.TenantID = tenantAlfa.String()
	c.Subject = uuid.NewSHA1(tenantAlfa, []byte(c.Subject)).String()
	return c, nil
}

// humo es el entorno armado: el mux público, la base y el bus.
type humo struct {
	mux     *http.ServeMux
	base    *pgtest.Base
	workers []string // Name() de los workers arrancados
}

// migrarBase aplica cmd/migrate sobre la base, como el paso de despliegue.
func migrarBase(t *testing.T, base *pgtest.Base) {
	t.Helper()
	cmd := exec.Command("go", "run", "../migrate")
	cmd.Env = append(os.Environ(), "DB_ADMIN_URL="+base.URL(""))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cmd/migrate: %v\n%s", err, out)
	}
}

// armarHumo migra la base, abre un pool por rol con la credencial del propio rol
// (así rige el search_path que fijó el bootstrap) y construye los módulos como run().
func armarHumo(t *testing.T) *humo {
	t.Helper()
	base := pgtest.Nueva(t, os.Getenv)
	migrarBase(t, base)
	for _, r := range rolesDeBD {
		t.Setenv(r.env, base.URL(r.rol))
	}

	ctx, cancelar := context.WithCancel(context.Background())
	pls, cerrar, err := abrirPools(ctx, server.New(":0"))
	if err != nil {
		cancelar()
		t.Fatalf("abrirPools: %v", err)
	}
	b := busmem.Nuevo(busregistry.Nuevo(), busmem.Opciones{})
	stub, err := devauth.Nuevo(devauth.EntornoDesarrollo)
	if err != nil {
		t.Fatalf("devauth: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	modules, idn, err := construirModulos(pls, b, b, b, devauthConTenantUUID{stub}, idpDePrueba(t), nil, logger)
	if err != nil {
		t.Fatalf("construirModulos: %v", err)
	}
	mux := http.NewServeMux()
	montarRutas(mux, modules)

	done := make(chan struct{}, 64)
	arrancados := 0
	var nombres []string
	for _, m := range modules {
		for _, w := range m.Workers() {
			arrancados++
			nombres = append(nombres, w.Name())
			go func() { _ = w.Run(ctx); done <- struct{}{} }()
		}
	}
	for _, w := range idn.Workers() {
		arrancados++
		go func() { _ = w.Run(ctx); done <- struct{}{} }()
	}
	t.Cleanup(func() {
		cancelar()
		for i := 0; i < arrancados; i++ {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Errorf("un worker no drenó en 10 s")
				i = arrancados
			}
		}
		cerrar()
	})
	return &humo{mux: mux, base: base, workers: nombres}
}

// llamar hace una petición al mux público con el token de desarrollo del rol, en
// el tenant alfa. El cuerpo, si lo hay, va como JSON.
func (h *humo) llamar(t *testing.T, metodo, ruta, rol string, cuerpo any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if cuerpo != nil {
		b, _ := json.Marshal(cuerpo)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(metodo, ruta, r)
	req.Header.Set("Authorization", "Bearer "+rol+"@tenant-dev-alfa")
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// TestHumoSolicitudHastaViajeAprobadoYConversacion recorre el camino feliz de la
// ola: el cliente crea la solicitud, `cotizacion` cotiza y confirma el pago, el
// jefe operativo aprueba; el viaje existe con el MISMO id en APROBADO y
// mensajeria tiene la conversación de la solicitud (DEC-148, ADR 38 y 39).
func TestHumoSolicitudHastaViajeAprobadoYConversacion(t *testing.T) {
	h := armarHumo(t)

	// Con el pool op_solicitudes operacion arranca el relay del outbox de
	// solicitudes y el worker de vencimiento; sin pool no hay ni uno ni otro.
	for _, w := range []string{"outbox-relay:solicitudes", "vencimiento:solicitudes"} {
		if !contieneSubcadena(h.workers, w) {
			t.Fatalf("falta el worker %q entre %v: operacion no recibió el pool op_solicitudes", w, h.workers)
		}
	}

	// 1. El cliente crea la solicitud.
	codigo, cuerpo := h.llamar(t, http.MethodPost, "/api/v1/solicitudes", "cliente", map[string]any{
		"origen":   "CDMX",
		"destinos": []map[string]any{{"nombre": "Guadalajara"}},
	})
	if codigo != http.StatusCreated {
		t.Fatalf("crear solicitud: %d %s", codigo, cuerpo)
	}
	var sol struct {
		Id     uuid.UUID `json:"id"`
		Estado string    `json:"estado"`
	}
	if err := json.Unmarshal(cuerpo, &sol); err != nil || sol.Id == uuid.Nil {
		t.Fatalf("respuesta de crear: %v %s", err, cuerpo)
	}
	ruta := "/api/v1/solicitudes/" + sol.Id.String()

	// 2. `cotizacion` cotiza (la fecha del servicio entra con la cotización).
	servicio := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	paso := func(nombre, metodo, sufijo, rol string, body any, esperado string) {
		t.Helper()
		c, b := h.llamar(t, metodo, ruta+sufijo, rol, body)
		if c < 200 || c > 299 {
			t.Fatalf("%s: %d %s", nombre, c, b)
		}
		var s struct {
			Estado string `json:"estado"`
		}
		_ = json.Unmarshal(b, &s)
		if s.Estado != esperado {
			t.Fatalf("%s: estado %q, se esperaba %q (%s)", nombre, s.Estado, esperado, b)
		}
	}
	paso("cotizar", http.MethodPut, "/cotizacion", "cotizacion", map[string]any{
		"fecha_hora_servicio": servicio, "moneda": "MXN", "monto": "1500.00", "notas": nil,
	}, "COTIZADO")
	// 3. `cotizacion` confirma el pago.
	paso("confirmar pago", http.MethodPut, "/pago", "cotizacion", map[string]any{
		"fecha_pago": time.Now().UTC().Format("2006-01-02"), "referencia": "REF-1",
	}, "PAGADO")
	// 4. El jefe operativo aprueba.
	paso("aprobar", http.MethodPost, "/aprobacion", "jefe_operativo", nil, "APROBADO")

	// 5. El viaje existe con el mismo id, en APROBADO, nacido de la solicitud.
	ctx := context.Background()
	c := h.base.Conn(t)
	var estado string
	var nacio bool
	if err := c.QueryRow(ctx, `SELECT estado, nacio_de_solicitud FROM viajes.viaje WHERE id = $1`, sol.Id).Scan(&estado, &nacio); err != nil {
		t.Fatalf("el viaje con el id de la solicitud no existe: %v", err)
	}
	if estado != "APROBADO" || !nacio {
		t.Fatalf("viaje: estado %q nacio_de_solicitud %v; se esperaba APROBADO/true", estado, nacio)
	}
	var destinos int
	if err := c.QueryRow(ctx, `SELECT count(*) FROM viajes.destino WHERE viaje_id = $1`, sol.Id).Scan(&destinos); err != nil || destinos != 1 {
		t.Fatalf("destinos del viaje = %d (%v); se esperaba 1", destinos, err)
	}

	// 5b. Los eventos van al outbox que corresponde (ADR 35, ADR 39): solicitud.*
	// en solicitudes.outbox y viaje.* en viajes.outbox. Con un search_path
	// equivocado del rol todo caería en un solo outbox y la prueba lo delata.
	cuenta := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := c.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return n
	}
	if n := cuenta(`SELECT count(*) FROM solicitudes.outbox WHERE tipo = 'solicitud.creada' AND agregado_id = $1`, sol.Id.String()); n != 1 {
		t.Fatalf("solicitud.creada en solicitudes.outbox = %d; se esperaba 1", n)
	}
	if n := cuenta(`SELECT count(*) FROM viajes.outbox WHERE tipo LIKE 'solicitud.%'`); n != 0 {
		t.Fatalf("hay %d eventos solicitud.* en viajes.outbox; deben ir en solicitudes.outbox", n)
	}
	if n := cuenta(`SELECT count(*) FROM viajes.outbox WHERE tipo = 'viaje.creado' AND agregado_id = $1`, sol.Id.String()); n != 1 {
		t.Fatalf("viaje.creado en viajes.outbox = %d; se esperaba 1", n)
	}
	if n := cuenta(`SELECT count(*) FROM solicitudes.outbox WHERE tipo LIKE 'viaje.%'`); n != 0 {
		t.Fatalf("hay %d eventos viaje.* en solicitudes.outbox; deben ir en viajes.outbox", n)
	}

	// 6. mensajeria tiene la conversación de la solicitud: llega por el bus
	// (solicitud.creada → mensajeria-conversacion-solicitud), así que se espera.
	var conv uuid.UUID
	esperarHasta(t, 15*time.Second, "la conversación tipo=solicitud de "+sol.Id.String(), func() bool {
		err := c.QueryRow(ctx, `SELECT id FROM mensajeria.conversacion WHERE viaje_id = $1 AND tipo = 'solicitud'`, sol.Id).Scan(&conv)
		return err == nil
	})

	// 7. Y se lee por la superficie pública (bff → adaptador → mensajeria). Esa
	// ruta del bff es solo del portal del cliente.
	codigo, cuerpo = h.llamar(t, http.MethodGet, ruta+"/conversacion", "cliente", nil)
	if codigo != http.StatusOK {
		t.Fatalf("leer conversación de la solicitud: %d %s", codigo, cuerpo)
	}
	// La vista del cliente no expone el id de la conversación: lleva el de la
	// solicitud (el mismo del viaje) y su estado.
	var vista struct {
		ViajeId uuid.UUID `json:"viaje_id"`
		Estado  string    `json:"estado"`
	}
	if err := json.Unmarshal(cuerpo, &vista); err != nil || vista.ViajeId != sol.Id || vista.Estado != "abierta" {
		t.Fatalf("conversación leída = %s; se esperaba abierta con viaje_id %s (conversación %s)", cuerpo, sol.Id, conv)
	}
}

func esperarHasta(t *testing.T, plazo time.Duration, que string, ok func() bool) {
	t.Helper()
	for fin := time.Now().Add(plazo); time.Now().Before(fin); time.Sleep(50 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("no ocurrió en %s: %s", plazo, que)
}
