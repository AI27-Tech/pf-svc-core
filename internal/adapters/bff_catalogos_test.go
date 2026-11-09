package adapters

import (
	"context"
	"testing"
	"time"

	catclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/catalogos"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	bff "github.com/AI27-Tech/pf-svc-bff"
	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
)

// valoresCatalogoFake implementa solo ListarValoresCatalogo; el resto de
// catalogos.ConsultasAPI queda en la interfaz embebida (nil) y no debe invocarse.
type valoresCatalogoFake struct {
	catalogos.ConsultasAPI
	err         error
	tipoVisto   catclient.TipoCatalogo
	cursorVisto *catclient.Cursor
}

func (f *valoresCatalogoFake) ListarValoresCatalogo(_ context.Context, tipo catclient.TipoCatalogo, cursor *catclient.Cursor) (*catclient.PaginaValoresCatalogo, error) {
	f.tipoVisto, f.cursorVisto = tipo, cursor
	if f.err != nil {
		return nil, f.err
	}
	return &catclient.PaginaValoresCatalogo{}, nil
}

// TestBFFCatalogosListarTiposServicio: el adaptador fija el tipo tipos_servicio,
// reenvía el cursor del bff y propaga la categoría del error de catalogos.
func TestBFFCatalogosListarTiposServicio(t *testing.T) {
	fake := &valoresCatalogoFake{}
	cursor := catclient.Cursor("pagina-2")
	if _, err := NewBFFCatalogos(fake, nil).ListarTiposServicio(context.Background(), "", catclient.ListarValoresCatalogoParams{Cursor: &cursor}); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if fake.tipoVisto != catclient.TiposServicio {
		t.Fatalf("tipo = %q; se esperaba %q", fake.tipoVisto, catclient.TiposServicio)
	}
	if fake.cursorVisto == nil || *fake.cursorVisto != "pagina-2" {
		t.Fatalf("cursor = %v; se esperaba pagina-2", fake.cursorVisto)
	}

	fake = &valoresCatalogoFake{err: platformerrors.NewProhibido("catalogos: sin facultad")}
	_, err := NewBFFCatalogos(fake, nil).ListarTiposServicio(context.Background(), "", catclient.ListarValoresCatalogoParams{})
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryProhibido {
		t.Fatalf("categoría = %v; se esperaba Prohibido", got)
	}
}

// gpsFake implementa solo ListarGps; el resto de catalogos.ConsultasAPI queda en
// la interfaz embebida (nil) y no debe invocarse.
type gpsFake struct {
	catalogos.ConsultasAPI
	err         error
	cursorVisto *catclient.Cursor
}

func (f *gpsFake) ListarGps(_ context.Context, cursor *catclient.Cursor) (*catclient.PaginaGps, error) {
	f.cursorVisto = cursor
	if f.err != nil {
		return nil, f.err
	}
	return &catclient.PaginaGps{Items: []catclient.DispositivoGps{}}, nil
}

// TestBFFCatalogosListarGps: el adaptador reenvía el cursor del bff y propaga la
// categoría del error de catalogos (un cursor no UUID es Validacion → 400).
func TestBFFCatalogosListarGps(t *testing.T) {
	fake := &gpsFake{}
	cursor := catclient.Cursor("pagina-3")
	if _, err := NewBFFCatalogos(fake, nil).ListarGps(context.Background(), "", catclient.ListarGpsParams{Cursor: &cursor}); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if fake.cursorVisto == nil || *fake.cursorVisto != "pagina-3" {
		t.Fatalf("cursor = %v; se esperaba pagina-3", fake.cursorVisto)
	}

	fake = &gpsFake{err: platformerrors.NewValidacion("catalogos: cursor inválido")}
	_, err := NewBFFCatalogos(fake, nil).ListarGps(context.Background(), "", catclient.ListarGpsParams{})
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryValidacion {
		t.Fatalf("categoría = %v; se esperaba Validacion", got)
	}
}

// flotaFake implementa las lecturas y escrituras de vehículos y custodios
// (E8-T21); el resto de las dos superficies queda en las interfaces embebidas
// (nil) y no debe invocarse. Registra qué método se llamó y el contexto.
type flotaFake struct {
	catalogos.ConsultasAPI
	catalogos.ComandosAPI
	err      error
	llamado  string
	ctxVisto context.Context
}

func (f *flotaFake) ver(ctx context.Context, metodo string) error {
	f.llamado, f.ctxVisto = metodo, ctx
	return f.err
}

func (f *flotaFake) ListarVehiculos(ctx context.Context, _ *catclient.Cursor) (*catclient.PaginaVehiculos, error) {
	return &catclient.PaginaVehiculos{}, f.ver(ctx, "ListarVehiculos")
}

func (f *flotaFake) ListarVehiculosPorLote(ctx context.Context, _ []catclient.Id) (*catclient.PaginaVehiculos, error) {
	return &catclient.PaginaVehiculos{}, f.ver(ctx, "ListarVehiculosPorLote")
}

func (f *flotaFake) CrearVehiculo(ctx context.Context, _ catclient.VehiculoEntrada) (*catclient.Vehiculo, error) {
	return &catclient.Vehiculo{}, f.ver(ctx, "CrearVehiculo")
}

func (f *flotaFake) ActualizarVehiculo(ctx context.Context, _ catclient.Id, _ catclient.VehiculoCambios) (*catclient.Vehiculo, error) {
	return &catclient.Vehiculo{}, f.ver(ctx, "ActualizarVehiculo")
}

func (f *flotaFake) ListarCustodios(ctx context.Context, _ *catclient.Cursor) (*catclient.PaginaCustodios, error) {
	return &catclient.PaginaCustodios{}, f.ver(ctx, "ListarCustodios")
}

func (f *flotaFake) ListarCustodiosPorLote(ctx context.Context, _ []catclient.Id) (*catclient.PaginaCustodios, error) {
	return &catclient.PaginaCustodios{}, f.ver(ctx, "ListarCustodiosPorLote")
}

func (f *flotaFake) CrearCustodio(ctx context.Context, _ catclient.CustodioEntrada) (*catclient.Custodio, error) {
	return &catclient.Custodio{}, f.ver(ctx, "CrearCustodio")
}

func (f *flotaFake) ActualizarCustodio(ctx context.Context, _ catclient.Id, _ catclient.CustodioCambios) (*catclient.Custodio, error) {
	return &catclient.Custodio{}, f.ver(ctx, "ActualizarCustodio")
}

// TestBFFCatalogosFlota: cada método de vehículos y custodios llega al método
// esperado de catalogos con el presupuesto del bff y propaga la categoría del
// error; los listados despachan por lote con ids y rechazan ids con cursor,
// como el handler HTTP de catalogos (I3).
func TestBFFCatalogosFlota(t *testing.T) {
	id := catclient.Id{}
	ids := []catclient.Id{id}
	cursor := catclient.Cursor("c")
	casos := []struct {
		nombre, esperado string
		llamar           func(*BFFCatalogosEnProceso) error
	}{
		{"ListarVehiculos", "ListarVehiculos", func(a *BFFCatalogosEnProceso) error {
			_, err := a.ListarVehiculos(context.Background(), "", catclient.ListarVehiculosParams{Cursor: &cursor})
			return err
		}},
		{"ListarVehiculos con ids", "ListarVehiculosPorLote", func(a *BFFCatalogosEnProceso) error {
			_, err := a.ListarVehiculos(context.Background(), "", catclient.ListarVehiculosParams{Ids: &ids})
			return err
		}},
		{"CrearVehiculo", "CrearVehiculo", func(a *BFFCatalogosEnProceso) error {
			_, err := a.CrearVehiculo(context.Background(), "", catclient.VehiculoEntrada{})
			return err
		}},
		{"ActualizarVehiculo", "ActualizarVehiculo", func(a *BFFCatalogosEnProceso) error {
			_, err := a.ActualizarVehiculo(context.Background(), "", id, catclient.VehiculoCambios{})
			return err
		}},
		{"ListarCustodios", "ListarCustodios", func(a *BFFCatalogosEnProceso) error {
			_, err := a.ListarCustodios(context.Background(), "", catclient.ListarCustodiosParams{})
			return err
		}},
		{"ListarCustodios con ids", "ListarCustodiosPorLote", func(a *BFFCatalogosEnProceso) error {
			_, err := a.ListarCustodios(context.Background(), "", catclient.ListarCustodiosParams{Ids: &ids})
			return err
		}},
		{"CrearCustodio", "CrearCustodio", func(a *BFFCatalogosEnProceso) error {
			_, err := a.CrearCustodio(context.Background(), "", catclient.CustodioEntrada{})
			return err
		}},
		{"ActualizarCustodio", "ActualizarCustodio", func(a *BFFCatalogosEnProceso) error {
			_, err := a.ActualizarCustodio(context.Background(), "", id, catclient.CustodioCambios{})
			return err
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fake := &flotaFake{}
			if err := c.llamar(NewBFFCatalogos(fake, fake)); err != nil {
				t.Fatalf("no se esperaba error: %v", err)
			}
			if fake.llamado != c.esperado {
				t.Fatalf("llamado = %q; se esperaba %q", fake.llamado, c.esperado)
			}
			if dl, ok := fake.ctxVisto.Deadline(); !ok || time.Until(dl) > bff.PresupuestoCatalogos {
				t.Fatalf("deadline ausente o fuera del presupuesto (ok=%v)", ok)
			}

			fake = &flotaFake{err: platformerrors.NewValidacion("catalogos: entrada inválida")}
			if got := platformerrors.AsCategory(c.llamar(NewBFFCatalogos(fake, fake))); got != platformerrors.CategoryValidacion {
				t.Fatalf("categoría = %v; se esperaba Validacion", got)
			}
		})
	}

	fake := &flotaFake{}
	a := NewBFFCatalogos(fake, fake)
	_, errV := a.ListarVehiculos(context.Background(), "", catclient.ListarVehiculosParams{Ids: &ids, Cursor: &cursor})
	_, errC := a.ListarCustodios(context.Background(), "", catclient.ListarCustodiosParams{Ids: &ids, Cursor: &cursor})
	for _, err := range []error{errV, errC} {
		if got := platformerrors.AsCategory(err); got != platformerrors.CategoryValidacion {
			t.Fatalf("ids y cursor: categoría = %v; se esperaba Validacion", got)
		}
	}
	if fake.llamado != "" {
		t.Fatalf("ids y cursor no debe llegar a catalogos; llegó a %q", fake.llamado)
	}
}
