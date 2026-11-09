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
//
// Las escrituras de vehículos y custodios (E8-T21) van por catalogos.Comandos(),
// la superficie de escritura administrativa, separada de la de lectura.
type BFFCatalogosEnProceso struct {
	consultas catalogos.ConsultasAPI
	comandos  catalogos.ComandosAPI
}

var _ bff.CatalogosPort = (*BFFCatalogosEnProceso)(nil)

// NewBFFCatalogos construye el adaptador en proceso sobre las superficies
// Consultas() y Comandos() de catalogos. La raíz lo inyecta en
// bff.Deps.Catalogos.
func NewBFFCatalogos(c catalogos.ConsultasAPI, cmd catalogos.ComandosAPI) *BFFCatalogosEnProceso {
	return &BFFCatalogosEnProceso{consultas: c, comandos: cmd}
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

// ListarTiposServicio reenvía el listado del catálogo tipos_servicio (selector
// del wizard de solicitud del cliente, E9-T4) sobre ListarValoresCatalogo con el
// tipo fijo, como el adaptador HTTP de bff. El filtro a activos lo hace el bff al
// recortar. bearer se ignora en proceso (los claims ya van en el ctx).
func (a *BFFCatalogosEnProceso) ListarTiposServicio(ctx context.Context, _ string, params catclient.ListarValoresCatalogoParams) (*catclient.PaginaValoresCatalogo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	out, err := a.consultas.ListarValoresCatalogo(ctx, catclient.TiposServicio, params.Cursor)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ListarTiposServicio", err)
	}
	return out, nil
}

// ListarGps reenvía el listado de dispositivos GPS activos (selector de la ficha
// técnica, E8-T12). Como ListarClientes, catalogos toma solo el cursor.
func (a *BFFCatalogosEnProceso) ListarGps(ctx context.Context, _ string, params catclient.ListarGpsParams) (*catclient.PaginaGps, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	out, err := a.consultas.ListarGps(ctx, params.Cursor)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ListarGps", err)
	}
	return out, nil
}

// --- Vehículos y custodios de /catalogos (adelanto de E8-T16, DEC-142 #6) ---
// La guarda de rol la aplica el handler del bff antes de llamar. Los listados
// replican el despacho del handler HTTP de catalogos: con ids, lectura por lote;
// ids y cursor juntos, 400.

// ListarVehiculos reenvía el listado (o el lote por ids) de vehículos vigentes.
func (a *BFFCatalogosEnProceso) ListarVehiculos(ctx context.Context, _ string, params catclient.ListarVehiculosParams) (*catclient.PaginaVehiculos, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	var (
		out *catclient.PaginaVehiculos
		err error
	)
	switch {
	case params.Ids != nil && params.Cursor != nil:
		err = platformerrors.NewValidacion("ids y cursor son mutuamente excluyentes")
	case params.Ids != nil:
		out, err = a.consultas.ListarVehiculosPorLote(ctx, *params.Ids)
	default:
		out, err = a.consultas.ListarVehiculos(ctx, params.Cursor)
	}
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ListarVehiculos", err)
	}
	return out, nil
}

// CrearVehiculo reenvía el alta de un vehículo de la flota (sin cliente, DEC-143).
func (a *BFFCatalogosEnProceso) CrearVehiculo(ctx context.Context, _ string, body catclient.VehiculoEntrada) (*catclient.Vehiculo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	out, err := a.comandos.CrearVehiculo(ctx, body)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "CrearVehiculo", err)
	}
	return out, nil
}

// ActualizarVehiculo reenvía la edición de un vehículo (nueva versión vigente).
func (a *BFFCatalogosEnProceso) ActualizarVehiculo(ctx context.Context, _ string, id catclient.Id, body catclient.VehiculoCambios) (*catclient.Vehiculo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	out, err := a.comandos.ActualizarVehiculo(ctx, id, body)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ActualizarVehiculo", err)
	}
	return out, nil
}

// ListarCustodios reenvía el listado (o el lote por ids) de custodios vigentes.
func (a *BFFCatalogosEnProceso) ListarCustodios(ctx context.Context, _ string, params catclient.ListarCustodiosParams) (*catclient.PaginaCustodios, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	var (
		out *catclient.PaginaCustodios
		err error
	)
	switch {
	case params.Ids != nil && params.Cursor != nil:
		err = platformerrors.NewValidacion("ids y cursor son mutuamente excluyentes")
	case params.Ids != nil:
		out, err = a.consultas.ListarCustodiosPorLote(ctx, *params.Ids)
	default:
		out, err = a.consultas.ListarCustodios(ctx, params.Cursor)
	}
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ListarCustodios", err)
	}
	return out, nil
}

// CrearCustodio reenvía el alta de un custodio.
func (a *BFFCatalogosEnProceso) CrearCustodio(ctx context.Context, _ string, body catclient.CustodioEntrada) (*catclient.Custodio, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	out, err := a.comandos.CrearCustodio(ctx, body)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "CrearCustodio", err)
	}
	return out, nil
}

// ActualizarCustodio reenvía la edición de un custodio (nueva versión vigente).
func (a *BFFCatalogosEnProceso) ActualizarCustodio(ctx context.Context, _ string, id catclient.Id, body catclient.CustodioCambios) (*catclient.Custodio, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()

	out, err := a.comandos.ActualizarCustodio(ctx, id, body)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ActualizarCustodio", err)
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
