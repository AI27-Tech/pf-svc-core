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
