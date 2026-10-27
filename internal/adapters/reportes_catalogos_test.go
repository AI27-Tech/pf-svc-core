package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	catclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/catalogos"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
	reportes "github.com/AI27-Tech/pf-svc-reportes"
)

// catalogosDimFake implementa catalogos.ConsultasAPI para probar la paridad del
// adaptador en proceso reportes → catalogos (I3). Solo se usan ObtenerContrato y
// ObtenerSla; el resto satisface la interfaz y no debe invocarse.
type catalogosDimFake struct {
	contrato *catclient.Contrato
	sla      *catclient.Sla
	err      error
	retardo  time.Duration
	ctxVisto context.Context
}

var _ catalogos.ConsultasAPI = (*catalogosDimFake)(nil)

func (f *catalogosDimFake) esperar(ctx context.Context) error {
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

func (f *catalogosDimFake) ObtenerContrato(ctx context.Context, _ catclient.Id, _ *catclient.MarcaTiempo) (*catclient.Contrato, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return f.contrato, nil
}

func (f *catalogosDimFake) ObtenerSla(ctx context.Context, _ catclient.Id, _ *catclient.MarcaTiempo) (*catclient.Sla, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return f.sla, nil
}

func (f *catalogosDimFake) ObtenerCliente(context.Context, catclient.Id) (*catclient.Cliente, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) AptitudCustodio(context.Context, catclient.Id, catclient.Id) (*catclient.Aptitud, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) AptitudVehiculo(context.Context, catclient.Id, catclient.Id) (*catclient.Aptitud, error) {
	panic("no debe invocarse en estas pruebas")
}

// Resto de ConsultasAPI (listados y obtención de catalogos v0.1.3): no se usan
// en estas pruebas de dimensiones; stubs que fallan si se invocan.
func (f *catalogosDimFake) ListarClientes(context.Context, *catclient.Cursor) (*catclient.PaginaClientes, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) ListarVehiculos(context.Context, *catclient.Cursor) (*catclient.PaginaVehiculos, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) ListarVehiculosPorLote(context.Context, []catclient.Id) (*catclient.PaginaVehiculos, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) ObtenerVehiculo(context.Context, catclient.Id, *catclient.MarcaTiempo) (*catclient.Vehiculo, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) ListarCustodios(context.Context, *catclient.Cursor) (*catclient.PaginaCustodios, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) ListarCustodiosPorLote(context.Context, []catclient.Id) (*catclient.PaginaCustodios, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) ObtenerCustodio(context.Context, catclient.Id, *catclient.MarcaTiempo) (*catclient.Custodio, error) {
	panic("no debe invocarse en estas pruebas")
}

func (f *catalogosDimFake) ListarValoresCatalogo(context.Context, catclient.TipoCatalogo, *catclient.Cursor) (*catclient.PaginaValoresCatalogo, error) {
	panic("no debe invocarse en estas pruebas")
}

// TestReportesCatalogosImponePresupuesto verifica que el adaptador aplica el
// deadline reportes.PresupuestoCatalogos al contexto que recibe catalogos (I3).
func TestReportesCatalogosImponePresupuesto(t *testing.T) {
	fake := &catalogosDimFake{contrato: &catclient.Contrato{}}
	ad := NewReportesCatalogos(fake)

	if _, err := ad.ObtenerContrato(context.Background(), catclient.Id{}, nil); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	dl, ok := fake.ctxVisto.Deadline()
	if !ok {
		t.Fatal("el adaptador no impuso deadline al contexto de catalogos")
	}
	if restante := time.Until(dl); restante <= 0 || restante > reportes.PresupuestoCatalogos {
		t.Fatalf("deadline fuera del presupuesto: restante=%v presupuesto=%v", restante, reportes.PresupuestoCatalogos)
	}
}

// TestReportesCatalogosTimeoutDegradado verifica que agotar el presupuesto
// produce la MISMA categoría que el adaptador HTTP ante un timeout: Degradado.
func TestReportesCatalogosTimeoutDegradado(t *testing.T) {
	fake := &catalogosDimFake{retardo: reportes.PresupuestoCatalogos + 500*time.Millisecond}
	ad := NewReportesCatalogos(fake)

	_, err := ad.ObtenerSla(context.Background(), catclient.Id{}, nil)
	if err == nil {
		t.Fatal("se esperaba error por timeout")
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado", got)
	}
}

// TestReportesCatalogosCancelacion verifica que una cancelación externa se
// traduce a Degradado (I3, obligación 2: respeta cancelación).
func TestReportesCatalogosCancelacion(t *testing.T) {
	fake := &catalogosDimFake{retardo: time.Hour}
	ad := NewReportesCatalogos(fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ad.ObtenerContrato(ctx, catclient.Id{}, nil)
	if err == nil {
		t.Fatal("se esperaba error por cancelación")
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado", got)
	}
}

// TestReportesCatalogosPropagaErrorDeContrato verifica que cada clase de error
// de catalogos.Consultas() se propaga con la MISMA categoría, sin recategorizar.
func TestReportesCatalogosPropagaErrorDeContrato(t *testing.T) {
	casos := []struct {
		nombre string
		err    error
		quiere platformerrors.Category
	}{
		{"no_encontrado", platformerrors.NewNoEncontrado("catalogos: no existe"), platformerrors.CategoryNoEncontrado},
		{"no_autorizado", platformerrors.NewNoAutorizado("catalogos: no autorizado"), platformerrors.CategoryNoAutorizado},
		{"degradado", platformerrors.NewDegradado("catalogos: no disponible"), platformerrors.CategoryDegradado},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fake := &catalogosDimFake{err: c.err}
			ad := NewReportesCatalogos(fake)

			_, err := ad.ObtenerContrato(context.Background(), catclient.Id{}, nil)
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
