package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	opclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/operacion"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	bff "github.com/AI27-Tech/pf-svc-bff"
	operacion "github.com/AI27-Tech/pf-svc-operacion"
)

// operacionFake implementa operacion.ConsultasAPI para probar la paridad I3 del
// adaptador en proceso bff → operacion. Captura el contexto recibido (para
// verificar el deadline) y simula retardo o error.
type operacionFake struct {
	err      error
	retardo  time.Duration
	ctxVisto context.Context
}

var _ operacion.ConsultasAPI = (*operacionFake)(nil)

func (f *operacionFake) esperar(ctx context.Context) error {
	f.ctxVisto = ctx
	if f.retardo > 0 {
		select {
		case <-time.After(f.retardo):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.err
}

func (f *operacionFake) CrearViaje(ctx context.Context, _ opclient.CrearViajeEntrada) (*opclient.Viaje, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Viaje{}, nil
}

func (f *operacionFake) ListarViajes(ctx context.Context, _ opclient.ListarViajesParams) (*opclient.PaginaViajes, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.PaginaViajes{}, nil
}

func (f *operacionFake) ObtenerViaje(ctx context.Context, _ opclient.Id) (*opclient.ViajeDetalle, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.ViajeDetalle{}, nil
}

func (f *operacionFake) ListarBitacora(ctx context.Context, _ opclient.Id, _ opclient.ListarBitacoraParams) (*opclient.PaginaBitacora, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.PaginaBitacora{}, nil
}

func (f *operacionFake) ConsultarAuditoria(ctx context.Context, _ opclient.Id, _ opclient.ConsultarAuditoriaParams) (*opclient.PaginaAuditoria, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.PaginaAuditoria{}, nil
}

func (f *operacionFake) RegistrarDescargaViaje(ctx context.Context, _ opclient.Id) (*opclient.Viaje, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Viaje{}, nil
}

func (f *operacionFake) TransicionarViaje(ctx context.Context, _ opclient.Id, body opclient.TransicionEntrada) (*opclient.Viaje, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Viaje{Estado: body.AEstado}, nil
}

// TestBFFOperacionTransicionarViaje: la transición llega a operacion y su
// resultado (o su conflicto de máquina de estados) vuelve tal cual al bff.
func TestBFFOperacionTransicionarViaje(t *testing.T) {
	ad := NewBFFOperacion(&operacionFake{})
	out, err := ad.TransicionarViaje(context.Background(), opclient.Id{}, opclient.TransicionEntrada{AEstado: opclient.APROBADO})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if out.Estado != opclient.APROBADO {
		t.Fatalf("estado = %v; se esperaba APROBADO", out.Estado)
	}

	conflicto := platformerrors.NewConflictoCodigo("transicion_invalida", "operacion: arista no permitida")
	ad = NewBFFOperacion(&operacionFake{err: conflicto})
	_, err = ad.TransicionarViaje(context.Background(), opclient.Id{}, opclient.TransicionEntrada{AEstado: opclient.ENDESTINO})
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryConflicto {
		t.Fatalf("categoría = %v; se esperaba Conflicto", got)
	}
	if got := platformerrors.CodigoDe(err); got != "transicion_invalida" {
		t.Fatalf("código = %q; se esperaba transicion_invalida", got)
	}
}

// TestBFFOperacionImponeElMismoPresupuesto: el adaptador aplica el deadline
// bff.PresupuestoOperacion al contexto que recibe operacion (I3, obligación 1).
func TestBFFOperacionImponeElMismoPresupuesto(t *testing.T) {
	fake := &operacionFake{}
	ad := NewBFFOperacion(fake)

	if _, err := ad.ObtenerViaje(context.Background(), opclient.Id{}); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	dl, ok := fake.ctxVisto.Deadline()
	if !ok {
		t.Fatal("el adaptador no impuso deadline al contexto de operacion")
	}
	if restante := time.Until(dl); restante <= 0 || restante > bff.PresupuestoOperacion {
		t.Fatalf("deadline fuera del presupuesto: restante=%v presupuesto=%v", restante, bff.PresupuestoOperacion)
	}
}

// TestBFFOperacionTimeoutDegradado: agotar el presupuesto se traduce a Degradado,
// igual que el DeTransporte del adaptador HTTP de bff ante un timeout de red.
func TestBFFOperacionTimeoutDegradado(t *testing.T) {
	fake := &operacionFake{retardo: bff.PresupuestoOperacion + 500*time.Millisecond}
	ad := NewBFFOperacion(fake)

	_, err := ad.CrearViaje(context.Background(), opclient.CrearViajeEntrada{})
	if err == nil {
		t.Fatal("se esperaba error por timeout")
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado (paridad con HTTP)", got)
	}
}

// TestBFFOperacionCancelacion: una cancelación externa se traduce a Degradado y
// no se cuelga (I3, obligación 2).
func TestBFFOperacionCancelacion(t *testing.T) {
	fake := &operacionFake{retardo: time.Hour}
	ad := NewBFFOperacion(fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ad.ListarViajes(ctx, opclient.ListarViajesParams{})
	if err == nil {
		t.Fatal("se esperaba error por cancelación")
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado", got)
	}
}

// TestBFFOperacionPropagaErrorDeContrato: cada clase de error que operacion
// devuelve (ya categorizada) se propaga con la MISMA categoría, igual que el
// adaptador HTTP traduce cada status (I3, obligación 2).
func TestBFFOperacionPropagaErrorDeContrato(t *testing.T) {
	casos := []struct {
		nombre string
		err    error
		quiere platformerrors.Category
	}{
		{"no_encontrado", platformerrors.NewNoEncontrado("operacion: viaje no existe"), platformerrors.CategoryNoEncontrado},
		{"validacion", platformerrors.NewValidacion("operacion: entrada inválida"), platformerrors.CategoryValidacion},
		{"conflicto", platformerrors.NewConflicto("operacion: estado en conflicto"), platformerrors.CategoryConflicto},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fake := &operacionFake{err: c.err}
			ad := NewBFFOperacion(fake)

			_, err := ad.ObtenerViaje(context.Background(), opclient.Id{})
			if err == nil {
				t.Fatal("se esperaba error")
			}
			if got := platformerrors.AsCategory(err); got != c.quiere {
				t.Fatalf("categoría = %v; se esperaba %v", got, c.quiere)
			}
			if !errors.Is(err, c.err) {
				t.Fatal("el error original no se preservó en la cadena")
			}
		})
	}
}

func (f *operacionFake) FijarEta(ctx context.Context, _ opclient.Id, _ opclient.EtaEntrada) (*opclient.Viaje, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Viaje{}, nil
}

func (f *operacionFake) AsignarRecursos(ctx context.Context, _ opclient.Id, _ opclient.AsignacionEntrada) (*opclient.Asignacion, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Asignacion{}, nil
}

func (f *operacionFake) RegistrarEntradaBitacora(ctx context.Context, _ opclient.Id, _ opclient.EntradaBitacoraEntrada) (*opclient.EntradaBitacora, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.EntradaBitacora{}, nil
}

func (f *operacionFake) AbrirIncidencia(ctx context.Context, _ opclient.Id, _ opclient.IncidenciaEntrada) (*opclient.Incidencia, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Incidencia{}, nil
}

func (f *operacionFake) ResolverIncidencia(ctx context.Context, _, _ opclient.Id, _ opclient.ResolucionIncidenciaEntrada) (*opclient.Incidencia, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Incidencia{}, nil
}

func (f *operacionFake) ListarEstadias(ctx context.Context, _ opclient.Id, _ opclient.ListarEstadiasParams) (*opclient.PaginaEstadias, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.PaginaEstadias{}, nil
}

func (f *operacionFake) AbrirEstadia(ctx context.Context, _ opclient.Id, _ opclient.EstadiaEntrada) (*opclient.Estadia, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Estadia{}, nil
}

func (f *operacionFake) CerrarEstadia(ctx context.Context, _, _ opclient.Id) (*opclient.Estadia, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Estadia{}, nil
}

func (f *operacionFake) ListarAdjuntos(ctx context.Context, _ opclient.Id) (*opclient.PaginaAdjuntos, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.PaginaAdjuntos{}, nil
}

func (f *operacionFake) CrearIntencionSubidaAdjunto(ctx context.Context, _ opclient.Id, _ opclient.IntencionSubidaAdjuntoEntrada) (*opclient.IntencionSubidaAdjunto, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.IntencionSubidaAdjunto{}, nil
}

func (f *operacionFake) DescargarAdjunto(ctx context.Context, _, _ opclient.Id) (*opclient.DescargaAdjunto, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.DescargaAdjunto{}, nil
}

func (f *operacionFake) ObtenerAsignacion(ctx context.Context, id opclient.Id) (*opclient.EscoltaVigente, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.EscoltaVigente{ViajeId: id, Unidades: []opclient.Unidad{}}, nil
}

func (f *operacionFake) ListarUnidades(ctx context.Context, _ opclient.ListarUnidadesParams) (*opclient.PaginaUnidades, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.PaginaUnidades{}, nil
}

func (f *operacionFake) CrearUnidad(ctx context.Context, _ opclient.UnidadEntrada) (*opclient.Unidad, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Unidad{}, nil
}

func (f *operacionFake) ObtenerUnidad(ctx context.Context, id opclient.Id) (*opclient.Unidad, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Unidad{Id: id}, nil
}

func (f *operacionFake) ModificarUnidad(ctx context.Context, id opclient.Id, _ opclient.UnidadModificacion) (*opclient.Unidad, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Unidad{Id: id}, nil
}

func (f *operacionFake) CompletarFechaHora(ctx context.Context, _ opclient.Id, _ opclient.FechaHoraEntrada) (*opclient.Viaje, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.Viaje{}, nil
}

func (f *operacionFake) ListarIncidencias(ctx context.Context, _ opclient.Id, _ opclient.ListarIncidenciasParams) (*opclient.PaginaIncidencias, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.PaginaIncidencias{}, nil
}

func (f *operacionFake) ObtenerFicha(ctx context.Context, id opclient.Id) (*opclient.FichaTecnica, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.FichaTecnica{ViajeId: id}, nil
}

func (f *operacionFake) GuardarFicha(ctx context.Context, id opclient.Id, _ opclient.FichaTecnicaEntrada) (*opclient.FichaTecnica, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return &opclient.FichaTecnica{ViajeId: id}, nil
}

// TestBFFOperacionFlujoInterno: cada método del flujo interno impone el
// presupuesto de operacion y propaga el conflicto con su código de máquina,
// como el adaptador HTTP de bff (I3).
func TestBFFOperacionFlujoInterno(t *testing.T) {
	id := opclient.Id{}
	llamadas := map[string]func(*BFFOperacionEnProceso, context.Context) error{
		"FijarEta": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.FijarEta(ctx, id, opclient.EtaEntrada{})
			return err
		},
		"AsignarRecursos": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.AsignarRecursos(ctx, id, opclient.AsignacionEntrada{})
			return err
		},
		"RegistrarEntradaBitacora": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.RegistrarEntradaBitacora(ctx, id, opclient.EntradaBitacoraEntrada{})
			return err
		},
		"AbrirIncidencia": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.AbrirIncidencia(ctx, id, opclient.IncidenciaEntrada{})
			return err
		},
		"ResolverIncidencia": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.ResolverIncidencia(ctx, id, id, opclient.ResolucionIncidenciaEntrada{})
			return err
		},
		"ListarEstadias": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.ListarEstadias(ctx, id, opclient.ListarEstadiasParams{})
			return err
		},
		"AbrirEstadia": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.AbrirEstadia(ctx, id, opclient.EstadiaEntrada{})
			return err
		},
		"CerrarEstadia": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.CerrarEstadia(ctx, id, id)
			return err
		},
		"ObtenerAsignacion": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.ObtenerAsignacion(ctx, id)
			return err
		},
		"ListarUnidades": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.ListarUnidades(ctx, opclient.ListarUnidadesParams{})
			return err
		},
		"CrearUnidad": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.CrearUnidad(ctx, opclient.UnidadEntrada{})
			return err
		},
		"ObtenerUnidad": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.ObtenerUnidad(ctx, id)
			return err
		},
		"ModificarUnidad": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.ModificarUnidad(ctx, id, opclient.UnidadModificacion{})
			return err
		},
		"CompletarFechaHora": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.CompletarFechaHora(ctx, id, opclient.FechaHoraEntrada{})
			return err
		},
		"ListarIncidencias": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.ListarIncidencias(ctx, id, opclient.ListarIncidenciasParams{})
			return err
		},
		"ObtenerFicha": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.ObtenerFicha(ctx, id)
			return err
		},
		"GuardarFicha": func(a *BFFOperacionEnProceso, ctx context.Context) error {
			_, err := a.GuardarFicha(ctx, id, opclient.FichaTecnicaEntrada{})
			return err
		},
	}
	for nombre, llamar := range llamadas {
		t.Run(nombre, func(t *testing.T) {
			fake := &operacionFake{}
			if err := llamar(NewBFFOperacion(fake), context.Background()); err != nil {
				t.Fatalf("no se esperaba error: %v", err)
			}
			if dl, ok := fake.ctxVisto.Deadline(); !ok || time.Until(dl) > bff.PresupuestoOperacion {
				t.Fatalf("deadline ausente o fuera del presupuesto (ok=%v)", ok)
			}

			fake = &operacionFake{err: platformerrors.NewConflictoCodigo("recurso_no_disponible", "operacion: unidad ocupada")}
			err := llamar(NewBFFOperacion(fake), context.Background())
			if got := platformerrors.CodigoDe(err); got != "recurso_no_disponible" {
				t.Fatalf("código = %q (err %v); se esperaba recurso_no_disponible", got, err)
			}
		})
	}
}
