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
