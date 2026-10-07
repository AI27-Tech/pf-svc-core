package adapters

import (
	"context"
	"errors"

	catclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/catalogos"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
	reportes "github.com/AI27-Tech/pf-svc-reportes"
)

// ReportesCatalogosEnProceso es el adaptador EN PROCESO de la arista
// reportes → catalogos (puerto reportes.CatalogosPort). Vive en pf-svc-core
// porque es el único repo que ve a ambos módulos (00 regla 2, DEC-050 §2); el
// adaptador HTTP del mismo puerto vive en pf-svc-reportes/adapters/http/catalogos
// y no se cablea cuando catalogos está co-ubicado.
//
// Paridad con el adaptador HTTP (I3, ADR 17 §"Dos adaptadores"):
//
//  1. Mismo presupuesto de tiempo: toma reportes.PresupuestoCatalogos, el MISMO
//     valor que exporta el consumidor junto a su puerto. No se duplica el número.
//  2. Misma taxonomía de error: catalogos.Consultas() ya devuelve
//     *platformerrors.PlatformError categorizado, así que el error se propaga
//     tal cual; un vencimiento de contexto o cancelación se traduce a Degradado,
//     igual que traducirError del adaptador HTTP de reportes.
//  3. Tipos de pf-pkg-contracts (catclient), nunca tipos de internal/ de
//     catalogos (que además son inalcanzables).
//
// No abre conexiones: delega en catalogos.Consultas(), que usa SU pool y SU rol.
type ReportesCatalogosEnProceso struct {
	consultas catalogos.ConsultasAPI
}

var _ reportes.CatalogosPort = (*ReportesCatalogosEnProceso)(nil)

// NewReportesCatalogos construye el adaptador en proceso sobre la superficie
// Consultas() de catalogos. La raíz lo inyecta en reportes.Deps.Catalogos.
func NewReportesCatalogos(consultas catalogos.ConsultasAPI) *ReportesCatalogosEnProceso {
	return &ReportesCatalogosEnProceso{consultas: consultas}
}

// ObtenerContrato lee el contrato vigente a una fecha dada, con el mismo
// presupuesto de tiempo que el adaptador HTTP.
func (a *ReportesCatalogosEnProceso) ObtenerContrato(ctx context.Context, id catclient.Id, aFecha *catclient.MarcaTiempo) (*catclient.Contrato, error) {
	ctx, cancel := context.WithTimeout(ctx, reportes.PresupuestoCatalogos)
	defer cancel()

	contrato, err := a.consultas.ObtenerContrato(ctx, id, aFecha)
	if err != nil {
		return nil, traducirErrorCatalogos(ctx, "ObtenerContrato", err)
	}
	return contrato, nil
}

// ObtenerSla lee el SLA vigente a una fecha dada, con el mismo presupuesto de
// tiempo que el adaptador HTTP.
func (a *ReportesCatalogosEnProceso) ObtenerSla(ctx context.Context, id catclient.Id, aFecha *catclient.MarcaTiempo) (*catclient.Sla, error) {
	ctx, cancel := context.WithTimeout(ctx, reportes.PresupuestoCatalogos)
	defer cancel()

	sla, err := a.consultas.ObtenerSla(ctx, id, aFecha)
	if err != nil {
		return nil, traducirErrorCatalogos(ctx, "ObtenerSla", err)
	}
	return sla, nil
}

// traducirErrorCatalogos mantiene la misma taxonomía que el adaptador HTTP de
// reportes: un vencimiento del presupuesto o una cancelación del contexto se
// traduce a Degradado; cualquier otro error de catalogos.Consultas() ya es un
// PlatformError categorizado y se propaga sin recategorizar.
func traducirErrorCatalogos(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "reportes: %s contra catalogos (en proceso) falló", op)
	}
	return err
}
