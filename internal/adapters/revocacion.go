package adapters

import (
	"context"
	"errors"
	"time"

	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
	identidad "github.com/AI27-Tech/pf-svc-identidad"
	operacion "github.com/AI27-Tech/pf-svc-operacion"
	reportes "github.com/AI27-Tech/pf-svc-reportes"
)

// RevocacionEnProceso es el adaptador EN PROCESO de la arista RESTRINGIDA a
// identidad (DEC-051/058): solo listar revocadas y consultar una sesión puntual.
// La consumen operacion, catalogos y reportes, que declaran puertos de firma
// IDÉNTICA (ListarRevocadas []string, ObtenerSesion bool); por eso un solo tipo
// implementa los tres. Vive en pf-svc-core porque es el único repo que ve al
// consumidor y a identidad a la vez (00 regla 2, DEC-050 §2); el adaptador HTTP
// del mismo puerto vive en cada repo consumidor y no se cablea cuando identidad
// está co-ubicada.
//
// Paridad con el adaptador HTTP (I3, ADR 17):
//
//  1. Mismo presupuesto de tiempo: cada constructor toma el PresupuestoRevocacion
//     que EXPORTA su consumidor (operacion/catalogos/reportes). No se duplica el
//     número; los tres valen 2s hoy, pero cada arista lee el suyo.
//  2. Misma taxonomía de error: identidad.Consultas() devuelve errores de la
//     plataforma ya categorizados, que se propagan; un vencimiento o cancelación
//     del contexto se traduce a Degradado, igual que el adaptador HTTP.
//  3. Tipos primitivos del puerto (string/bool), nunca tipos de internal/ de
//     identidad.
//
// No abre conexiones: delega en identidad.Consultas().Revocacion(), que usa el
// pool y rol de identidad.
type RevocacionEnProceso struct {
	rev         identidad.RevocacionConsultas
	presupuesto time.Duration
}

// Las tres interfaces de consumidor que este adaptador satisface. Firma idéntica
// (DEC-051): un solo tipo las implementa.
var (
	_ operacion.Revocacion     = (*RevocacionEnProceso)(nil)
	_ catalogos.RevocacionPort = (*RevocacionEnProceso)(nil)
	_ reportes.RevocacionPort  = (*RevocacionEnProceso)(nil)
)

// NewRevocacionOperacion construye el adaptador para la arista operacion →
// identidad, con el presupuesto que exporta operacion (I3).
func NewRevocacionOperacion(c identidad.ConsultasAPI) *RevocacionEnProceso {
	return &RevocacionEnProceso{rev: c.Revocacion(), presupuesto: operacion.PresupuestoRevocacion}
}

// NewRevocacionCatalogos construye el adaptador para catalogos → identidad, con
// el presupuesto que exporta catalogos (I3).
func NewRevocacionCatalogos(c identidad.ConsultasAPI) *RevocacionEnProceso {
	return &RevocacionEnProceso{rev: c.Revocacion(), presupuesto: catalogos.PresupuestoRevocacion}
}

// NewRevocacionReportes construye el adaptador para reportes → identidad, con el
// presupuesto que exporta reportes (I3).
func NewRevocacionReportes(c identidad.ConsultasAPI) *RevocacionEnProceso {
	return &RevocacionEnProceso{rev: c.Revocacion(), presupuesto: reportes.PresupuestoRevocacion}
}

// ListarRevocadas devuelve los sesion_id revocados para el snapshot inicial de
// la proyección de tombstones del consumidor (ADR 26 §4).
func (a *RevocacionEnProceso) ListarRevocadas(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.presupuesto)
	defer cancel()

	ids, err := a.rev.ListarRevocadas(ctx)
	if err != nil {
		return nil, traducirErrorRevocacion(ctx, "ListarRevocadas", err)
	}
	return ids, nil
}

// ObtenerSesion consulta si una sesión puntual está revocada (fallback ADR 26 §5).
func (a *RevocacionEnProceso) ObtenerSesion(ctx context.Context, sesionID string) (revocada bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, a.presupuesto)
	defer cancel()

	revocada, err = a.rev.ObtenerSesion(ctx, sesionID)
	if err != nil {
		return false, traducirErrorRevocacion(ctx, "ObtenerSesion", err)
	}
	return revocada, nil
}

// traducirErrorRevocacion mantiene la misma taxonomía que el adaptador HTTP: un
// vencimiento del presupuesto o cancelación del contexto se traduce a Degradado
// (la revocación cae a modo degradado, ADR 26 §5); cualquier otro error de
// identidad.Consultas() ya es un PlatformError categorizado y se propaga.
func traducirErrorRevocacion(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "revocacion: %s contra identidad (en proceso) falló", op)
	}
	return err
}
