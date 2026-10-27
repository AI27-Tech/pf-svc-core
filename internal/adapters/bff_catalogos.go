package adapters

import (
	"context"
	"errors"

	catclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/catalogos"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	bff "github.com/AI27-Tech/pf-svc-bff"
	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
)

// BFFCatalogosEnProceso es el adaptador EN PROCESO de la arista bff → catalogos
// (puerto bff.CatalogosPort). Vive en pf-svc-core porque es el único repo que ve
// a bff y a catalogos a la vez (00 regla 2); el adaptador HTTP del mismo puerto
// vive en pf-svc-bff/adapters/http y no se cablea cuando catalogos está
// co-ubicado.
//
// Paridad con el adaptador HTTP de bff (I3, ADR 17):
//
//  1. Mismo presupuesto de tiempo: bff.PresupuestoCatalogos, el valor que
//     exporta el consumidor. No se duplica el número.
//  2. Misma taxonomía de error: catalogos.Consultas() devuelve errores de la
//     plataforma ya categorizados, que se propagan; un vencimiento o cancelación
//     del contexto se traduce a Degradado, igual que el DeTransporte del
//     adaptador HTTP de bff.
//  3. Tipos de pf-pkg-contracts (catclient), nunca tipos de internal/ de catalogos.
//
// El parámetro bearer del puerto es para el adaptador HTTP (propaga el header);
// en proceso los claims ya viajan en el ctx, así que se ignora, igual que en
// BFFOperacionEnProceso.
type BFFCatalogosEnProceso struct {
	consultas catalogos.ConsultasAPI
}

var _ bff.CatalogosPort = (*BFFCatalogosEnProceso)(nil)

// NewBFFCatalogos construye el adaptador en proceso sobre la superficie
// Consultas() de catalogos. La raíz lo inyecta en bff.Deps.Catalogos.
func NewBFFCatalogos(c catalogos.ConsultasAPI) *BFFCatalogosEnProceso {
	return &BFFCatalogosEnProceso{consultas: c}
}

// ListarClientes reenvía el listado de clientes activos (selector de creación de
// viaje, E8-T11). El puerto del bff lleva params; catalogos toma el cursor, así
// que se extrae. bearer se ignora en proceso (los claims ya van en el ctx).
func (a *BFFCatalogosEnProceso) ListarClientes(ctx context.Context, _ string, params catclient.ListarClientesParams) (*catclient.PaginaClientes, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	out, err := a.consultas.ListarClientes(ctx, params.Cursor)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ListarClientes", err)
	}
	return out, nil
}

// traducirErrorBFFCatalogos mantiene la taxonomía del adaptador HTTP de bff: un
// vencimiento del presupuesto o cancelación del contexto se traduce a Degradado;
// cualquier otro error de catalogos.Consultas() ya es un PlatformError
// categorizado y se propaga.
func traducirErrorBFFCatalogos(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "bff: %s contra catalogos (en proceso) falló", op)
	}
	return err
}
