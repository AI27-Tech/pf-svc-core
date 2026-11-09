package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	catclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/catalogos"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
	operacion "github.com/AI27-Tech/pf-svc-operacion"
)

// catalogosFake implementa catalogos.ConsultasAPI para probar la paridad del
// adaptador en proceso (I3). Solo se usan AptitudUnidad y AptitudCustodio; el
// resto satisface la interfaz y no debe invocarse.
type catalogosFake struct {
	apt *catclient.Aptitud
	err error
	// retardo simula una dependencia lenta para agotar el presupuesto de tiempo.
	retardo time.Duration
	// ctxVisto captura el contexto recibido para verificar que lleva deadline.
	ctxVisto context.Context
}

var _ catalogos.ConsultasAPI = (*catalogosFake)(nil)

func (f *catalogosFake) responder(ctx context.Context) (*catclient.Aptitud, error) {
	f.ctxVisto = ctx
	if f.retardo > 0 {
		select {
		case <-time.After(f.retardo):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.apt, f.err
}

func (f *catalogosFake) AptitudVehiculo(ctx context.Context, _, _ catclient.Id) (*catclient.Aptitud, error) {
	return f.responder(ctx)
}

func (f *catalogosFake) AptitudCustodio(ctx context.Context, _, _ catclient.Id) (*catclient.Aptitud, error) {
	return f.responder(ctx)
}

func (f *catalogosFake) ObtenerCliente(context.Context, catclient.Id) (*catclient.Cliente, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosFake) ObtenerContrato(context.Context, catclient.Id, *catclient.MarcaTiempo) (*catclient.Contrato, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosFake) ObtenerSla(context.Context, catclient.Id, *catclient.MarcaTiempo) (*catclient.Sla, error) {
	panic("no debe invocarse en estas pruebas")
}

// Resto de ConsultasAPI (listados y obtención de catalogos v0.1.3): no se usan
// en estas pruebas de aptitud; stubs que fallan si se invocan.
func (f *catalogosFake) ListarClientes(context.Context, *catclient.Cursor) (*catclient.PaginaClientes, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosFake) ListarVehiculos(context.Context, *catclient.Cursor) (*catclient.PaginaVehiculos, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosFake) ListarVehiculosPorLote(context.Context, []catclient.Id) (*catclient.PaginaVehiculos, error) {
	panic("no debe invocarse en estas pruebas")
}

// ObtenerVehiculo lo usa el puerto AtomosCatalogo (E8-T21).
func (f *catalogosFake) ObtenerVehiculo(ctx context.Context, _ catclient.Id, _ *catclient.MarcaTiempo) (*catclient.Vehiculo, error) {
	if _, err := f.responder(ctx); err != nil {
		return nil, err
	}
	return &catclient.Vehiculo{}, nil
}

func (f *catalogosFake) ListarCustodios(context.Context, *catclient.Cursor) (*catclient.PaginaCustodios, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosFake) ListarCustodiosPorLote(context.Context, []catclient.Id) (*catclient.PaginaCustodios, error) {
	panic("no debe invocarse en estas pruebas")
}

// ObtenerCustodio lo usa el puerto AtomosCatalogo (E8-T21).
func (f *catalogosFake) ObtenerCustodio(ctx context.Context, _ catclient.Id, _ *catclient.MarcaTiempo) (*catclient.Custodio, error) {
	if _, err := f.responder(ctx); err != nil {
		return nil, err
	}
	return &catclient.Custodio{}, nil
}

func (f *catalogosFake) ListarGps(context.Context, *catclient.Cursor) (*catclient.PaginaGps, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosFake) ListarValoresCatalogo(context.Context, catclient.TipoCatalogo, *catclient.Cursor) (*catclient.PaginaValoresCatalogo, error) {
	panic("no debe invocarse en estas pruebas")
}

// TestAdaptadorImponeElMismoPresupuesto verifica que el adaptador en proceso
// aplica un deadline (operacion.PresupuestoAptitud) al contexto que recibe
// catalogos, igual que el HTTP (I3, obligación 1).
func TestAdaptadorImponeElMismoPresupuesto(t *testing.T) {
	fake := &catalogosFake{apt: &catclient.Aptitud{}}
	ad := NewAptitudRecursos(fake)

	if _, err := ad.AptitudVehiculo(context.Background(), catclient.Id{}, catclient.Id{}); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	dl, ok := fake.ctxVisto.Deadline()
	if !ok {
		t.Fatal("el adaptador no impuso deadline al contexto de catalogos")
	}
	restante := time.Until(dl)
	// El deadline debe caer dentro del presupuesto exportado por el consumidor.
	if restante <= 0 || restante > operacion.PresupuestoAptitud {
		t.Fatalf("deadline fuera del presupuesto: restante=%v presupuesto=%v", restante, operacion.PresupuestoAptitud)
	}
}

// TestTimeoutSeTraduceADegradado verifica que agotar el presupuesto produce la
// MISMA categoría que el adaptador HTTP ante un timeout de red: Degradado
// (la aptitud es informativa y no gatea, DEC-055).
func TestTimeoutSeTraduceADegradado(t *testing.T) {
	// retardo > presupuesto para forzar el vencimiento del deadline.
	fake := &catalogosFake{retardo: operacion.PresupuestoAptitud + 500*time.Millisecond}
	ad := NewAptitudRecursos(fake)

	_, err := ad.AptitudCustodio(context.Background(), catclient.Id{}, catclient.Id{})
	if err == nil {
		t.Fatal("se esperaba error por timeout")
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado (paridad con HTTP)", got)
	}
}

// TestCancelacionSePropaga verifica que una cancelación externa se traduce a
// Degradado y no se queda colgada (I3, obligación 2: respeta cancelación).
func TestCancelacionSePropaga(t *testing.T) {
	fake := &catalogosFake{retardo: time.Hour} // nunca responde por sí mismo
	ad := NewAptitudRecursos(fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // ya cancelado al entrar

	_, err := ad.AptitudVehiculo(ctx, catclient.Id{}, catclient.Id{})
	if err == nil {
		t.Fatal("se esperaba error por cancelación")
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado", got)
	}
}

// TestErrorDeContratoSePropagaSinRecategorizar verifica que cada clase de error
// que catalogos.Consultas() devuelve (ya categorizada por la plataforma) se
// propaga con la MISMA categoría, igual que el adaptador HTTP traduce cada
// status a su categoría (I3, obligación 2).
func TestErrorDeContratoSePropagaSinRecategorizar(t *testing.T) {
	casos := []struct {
		nombre string
		err    error
		quiere platformerrors.Category
	}{
		{"no_encontrado", platformerrors.NewNoEncontrado("catalogos: no existe"), platformerrors.CategoryNoEncontrado},
		{"no_autorizado", platformerrors.NewNoAutorizado("catalogos: no autorizado"), platformerrors.CategoryNoAutorizado},
		{"degradado", platformerrors.NewDegradado("catalogos: aptitud no disponible"), platformerrors.CategoryDegradado},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fake := &catalogosFake{err: c.err}
			ad := NewAptitudRecursos(fake)

			_, err := ad.AptitudVehiculo(context.Background(), catclient.Id{}, catclient.Id{})
			if err == nil {
				t.Fatal("se esperaba error")
			}
			if got := platformerrors.AsCategory(err); got != c.quiere {
				t.Fatalf("categoría = %v; se esperaba %v", got, c.quiere)
			}
			// El error original debe seguir alcanzable en la cadena.
			if !errors.Is(err, c.err) {
				t.Fatal("el error original no se preservó en la cadena")
			}
		})
	}
}

// TestAtomosParidadConHTTP: la lectura por id del roster (AtomosCatalogo) usa el
// mismo presupuesto que la aptitud, propaga el no_encontrado (el roster lo vuelve
// 400) y traduce el vencimiento a Degradado (el roster responde 503), como
// AptitudRecursosHTTP de operacion.
func TestAtomosParidadConHTTP(t *testing.T) {
	llamadas := map[string]func(*AptitudRecursosEnProceso, context.Context) error{
		"ObtenerVehiculo": func(a *AptitudRecursosEnProceso, ctx context.Context) error {
			_, err := a.ObtenerVehiculo(ctx, catclient.Id{})
			return err
		},
		"ObtenerCustodio": func(a *AptitudRecursosEnProceso, ctx context.Context) error {
			_, err := a.ObtenerCustodio(ctx, catclient.Id{})
			return err
		},
	}
	for nombre, llamar := range llamadas {
		t.Run(nombre, func(t *testing.T) {
			fake := &catalogosFake{}
			if err := llamar(NewAptitudRecursos(fake), context.Background()); err != nil {
				t.Fatalf("no se esperaba error: %v", err)
			}
			if dl, ok := fake.ctxVisto.Deadline(); !ok || time.Until(dl) > operacion.PresupuestoAptitud {
				t.Fatalf("deadline ausente o fuera del presupuesto (ok=%v)", ok)
			}

			fake = &catalogosFake{err: platformerrors.NewNoEncontrado("catalogos: no existe")}
			if got := platformerrors.AsCategory(llamar(NewAptitudRecursos(fake), context.Background())); got != platformerrors.CategoryNoEncontrado {
				t.Fatalf("categoría = %v; se esperaba NoEncontrado", got)
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			fake = &catalogosFake{retardo: time.Hour}
			if got := platformerrors.AsCategory(llamar(NewAptitudRecursos(fake), ctx)); got != platformerrors.CategoryDegradado {
				t.Fatalf("categoría = %v; se esperaba Degradado", got)
			}
		})
	}
}
