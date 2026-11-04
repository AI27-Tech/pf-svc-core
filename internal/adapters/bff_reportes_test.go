package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	repclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/reportes"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	bff "github.com/AI27-Tech/pf-svc-bff"
	reportes "github.com/AI27-Tech/pf-svc-reportes"
)

// reportesFake implementa reportes.ConsultasAPI para probar la paridad I3 del
// adaptador en proceso bff → reportes. Captura el contexto recibido (para
// verificar el deadline) y simula retardo o error.
type reportesFake struct {
	viajes   *repclient.ReporteViajes
	estados  *repclient.ReporteEstados
	err      error
	retardo  time.Duration
	ctxVisto context.Context
}

var _ reportes.ConsultasAPI = (*reportesFake)(nil)

func (f *reportesFake) esperar(ctx context.Context) error {
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

func (f *reportesFake) ReporteViajes(ctx context.Context, _ repclient.ReporteViajesParams) (*repclient.ReporteViajes, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return f.viajes, nil
}

func (f *reportesFake) ReporteEstados(ctx context.Context, _ repclient.ReporteEstadosParams) (*repclient.ReporteEstados, error) {
	if err := f.esperar(ctx); err != nil {
		return nil, err
	}
	return f.estados, nil
}

// TestBFFReportesImponePresupuesto verifica que el adaptador aplica el deadline
// bff.PresupuestoReportes al contexto que recibe reportes (I3).
func TestBFFReportesImponePresupuesto(t *testing.T) {
	fake := &reportesFake{viajes: &repclient.ReporteViajes{}}
	ad := NewBFFReportes(fake)

	if _, err := ad.ReporteViajes(context.Background(), repclient.ReporteViajesParams{}); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	dl, ok := fake.ctxVisto.Deadline()
	if !ok {
		t.Fatal("el adaptador no impuso deadline al contexto de reportes")
	}
	if restante := time.Until(dl); restante <= 0 || restante > bff.PresupuestoReportes {
		t.Fatalf("deadline fuera del presupuesto: restante=%v presupuesto=%v", restante, bff.PresupuestoReportes)
	}
}

// TestBFFReportesConservaFrescura: el adaptador devuelve el reporte tal cual, con
// proyeccion_al y stale (DEC-033), sin recortar.
func TestBFFReportesConservaFrescura(t *testing.T) {
	al := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	stale := true
	fake := &reportesFake{viajes: &repclient.ReporteViajes{Total: 1, ProyeccionAl: &al, Stale: &stale}}
	ad := NewBFFReportes(fake)

	out, err := ad.ReporteViajes(context.Background(), repclient.ReporteViajesParams{})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if out.ProyeccionAl == nil || !out.ProyeccionAl.Equal(al) || out.Stale == nil || !*out.Stale {
		t.Fatalf("frescura recortada: %+v", out)
	}
}

// TestBFFReportesTimeoutDegradado: agotar el presupuesto produce Degradado, la
// misma categoría que el adaptador HTTP ante un timeout.
func TestBFFReportesTimeoutDegradado(t *testing.T) {
	fake := &reportesFake{retardo: bff.PresupuestoReportes + 500*time.Millisecond}
	ad := NewBFFReportes(fake)

	_, err := ad.ReporteEstados(context.Background(), repclient.ReporteEstadosParams{})
	if err == nil {
		t.Fatal("se esperaba error por timeout")
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado", got)
	}
}

// TestBFFReportesCancelacion: una cancelación externa se traduce a Degradado
// (I3, obligación 2: respeta cancelación).
func TestBFFReportesCancelacion(t *testing.T) {
	fake := &reportesFake{retardo: time.Hour}
	ad := NewBFFReportes(fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ad.ReporteViajes(ctx, repclient.ReporteViajesParams{})
	if err == nil {
		t.Fatal("se esperaba error por cancelación")
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado", got)
	}
}

// TestBFFReportesPropagaError: cada clase de error de reportes.Consultas() se
// propaga con la MISMA categoría, sin recategorizar.
func TestBFFReportesPropagaError(t *testing.T) {
	casos := []struct {
		nombre string
		err    error
		quiere platformerrors.Category
	}{
		{"prohibido", platformerrors.NewProhibido("reportes: solo interno"), platformerrors.CategoryProhibido},
		{"validacion", platformerrors.NewValidacion("reportes: sin rango"), platformerrors.CategoryValidacion},
		{"no_autorizado", platformerrors.NewNoAutorizado("reportes: sin claims"), platformerrors.CategoryNoAutorizado},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fake := &reportesFake{err: c.err}
			ad := NewBFFReportes(fake)

			_, err := ad.ReporteViajes(context.Background(), repclient.ReporteViajesParams{})
			if got := platformerrors.AsCategory(err); got != c.quiere {
				t.Fatalf("categoría = %v; se esperaba %v", got, c.quiere)
			}
			if !errors.Is(err, c.err) {
				t.Fatal("el error original no se preservó en la cadena")
			}
		})
	}
}
