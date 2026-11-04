package adapters

import (
	"context"
	"testing"

	catclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/catalogos"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
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
	if _, err := NewBFFCatalogos(fake).ListarTiposServicio(context.Background(), "", catclient.ListarValoresCatalogoParams{Cursor: &cursor}); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if fake.tipoVisto != catclient.TiposServicio {
		t.Fatalf("tipo = %q; se esperaba %q", fake.tipoVisto, catclient.TiposServicio)
	}
	if fake.cursorVisto == nil || *fake.cursorVisto != "pagina-2" {
		t.Fatalf("cursor = %v; se esperaba pagina-2", fake.cursorVisto)
	}

	fake = &valoresCatalogoFake{err: platformerrors.NewProhibido("catalogos: sin facultad")}
	_, err := NewBFFCatalogos(fake).ListarTiposServicio(context.Background(), "", catclient.ListarValoresCatalogoParams{})
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
	if _, err := NewBFFCatalogos(fake).ListarGps(context.Background(), "", catclient.ListarGpsParams{Cursor: &cursor}); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if fake.cursorVisto == nil || *fake.cursorVisto != "pagina-3" {
		t.Fatalf("cursor = %v; se esperaba pagina-3", fake.cursorVisto)
	}

	fake = &gpsFake{err: platformerrors.NewValidacion("catalogos: cursor inválido")}
	_, err := NewBFFCatalogos(fake).ListarGps(context.Background(), "", catclient.ListarGpsParams{})
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryValidacion {
		t.Fatalf("categoría = %v; se esperaba Validacion", got)
	}
}
