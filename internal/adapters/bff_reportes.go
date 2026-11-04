package adapters

import (
	"context"
	"errors"

	repclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/reportes"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	bff "github.com/AI27-Tech/pf-svc-bff"
	reportes "github.com/AI27-Tech/pf-svc-reportes"
)

// BFFReportesEnProceso es el adaptador EN PROCESO de la arista bff → reportes
// (puerto bff.ReportesPort). Vive en pf-svc-core porque es el único repo que ve a
// bff y a reportes a la vez (00 regla 2); el adaptador HTTP del mismo puerto vive
// en pf-svc-bff/adapters/http y no se cablea cuando reportes está co-ubicado.
//
// Paridad con el adaptador HTTP de bff (I3, ADR 17):
//
//  1. Mismo presupuesto de tiempo: bff.PresupuestoReportes, el valor que exporta
//     el consumidor. No se duplica el número.
//  2. Misma taxonomía de error: reportes.Consultas() devuelve errores de la
//     plataforma ya categorizados, que se propagan; un vencimiento o cancelación
//     del contexto se traduce a Degradado, igual que el DeTransporte del
//     adaptador HTTP de bff.
//  3. Tipos de pf-pkg-contracts (repclient), nunca tipos de internal/ de reportes.
//
// No abre conexiones: delega en reportes.Consultas(), que usa su pool y su rol.
type BFFReportesEnProceso struct {
	consultas reportes.ConsultasAPI
}

var _ bff.ReportesPort = (*BFFReportesEnProceso)(nil)

// NewBFFReportes construye el adaptador en proceso sobre la superficie
// Consultas() de reportes. La raíz lo inyecta en bff.Deps.Reportes.
func NewBFFReportes(c reportes.ConsultasAPI) *BFFReportesEnProceso {
	return &BFFReportesEnProceso{consultas: c}
}

// ReporteViajes reenvía el reporte de viajes; conserva proyeccion_al y stale
// (DEC-033), que reportes ya trae en la respuesta.
func (a *BFFReportesEnProceso) ReporteViajes(ctx context.Context, params repclient.ReporteViajesParams) (*repclient.ReporteViajes, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoReportes)
	defer cancel()

	out, err := a.consultas.ReporteViajes(ctx, params)
	if err != nil {
		return nil, traducirErrorReportes(ctx, "ReporteViajes", err)
	}
	return out, nil
}

// ReporteEstados reenvía la distribución por estado.
func (a *BFFReportesEnProceso) ReporteEstados(ctx context.Context, params repclient.ReporteEstadosParams) (*repclient.ReporteEstados, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoReportes)
	defer cancel()

	out, err := a.consultas.ReporteEstados(ctx, params)
	if err != nil {
		return nil, traducirErrorReportes(ctx, "ReporteEstados", err)
	}
	return out, nil
}

// traducirErrorReportes mantiene la misma taxonomía que el adaptador HTTP de bff:
// un vencimiento del presupuesto o una cancelación del contexto se traduce a
// Degradado; cualquier otro error de reportes.Consultas() ya es un PlatformError
// categorizado y se propaga sin recategorizar.
func traducirErrorReportes(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "bff: %s contra reportes (en proceso) falló", op)
	}
	return err
}
