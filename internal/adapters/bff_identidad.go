package adapters

import (
	"context"
	"errors"

	idclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/identidad"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	bff "github.com/AI27-Tech/pf-svc-bff"
	identidad "github.com/AI27-Tech/pf-svc-identidad"
)

// BFFIdentidadEnProceso es el adaptador EN PROCESO de la arista bff → identidad
// (puerto bff.IdentidadPort). Vive en pf-svc-core porque es el único repo que ve
// a bff y a identidad a la vez (00 regla 2); el adaptador HTTP del mismo puerto
// vive en pf-svc-bff/adapters/http y no se cablea cuando identidad está
// co-ubicada.
//
// Paridad con el adaptador HTTP de bff (I3, ADR 17):
//
//  1. Mismo presupuesto de tiempo: bff.PresupuestoIdentidad, el valor que
//     exporta el consumidor. No se duplica el número.
//  2. Misma taxonomía de error: identidad.Consultas().Sesion() devuelve errores
//     de la plataforma ya categorizados, que se propagan; un vencimiento o
//     cancelación del contexto se traduce a Degradado, igual que el DeTransporte
//     del adaptador HTTP de bff.
//  3. Tipos de pf-pkg-contracts (idclient), nunca tipos de internal/ de identidad.
//
// No abre conexiones: delega en identidad.Consultas().Sesion(), que usa el pool
// y rol de identidad.
type BFFIdentidadEnProceso struct {
	sesion identidad.SesionConsultas
}

var _ bff.IdentidadPort = (*BFFIdentidadEnProceso)(nil)

// NewBFFIdentidad construye el adaptador en proceso sobre la sub-fachada de
// sesión de identidad. La raíz lo inyecta en bff.Deps.Identidad.
func NewBFFIdentidad(c identidad.ConsultasAPI) *BFFIdentidadEnProceso {
	return &BFFIdentidadEnProceso{sesion: c.Sesion()}
}

// EmitirSesion reenvía credenciales (login). bff no valida credenciales.
func (a *BFFIdentidadEnProceso) EmitirSesion(ctx context.Context, body idclient.CredencialesEntrada) (*idclient.SesionEmitida, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()

	out, err := a.sesion.EmitirSesion(ctx, body)
	if err != nil {
		return nil, traducirErrorIdentidad(ctx, "EmitirSesion", err)
	}
	return out, nil
}

// RefrescarSesion renueva la sesión (reevaluación de claims, ADR 26).
func (a *BFFIdentidadEnProceso) RefrescarSesion(ctx context.Context, body idclient.RefrescoEntrada) (*idclient.SesionEmitida, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()

	out, err := a.sesion.RefrescarSesion(ctx, body)
	if err != nil {
		return nil, traducirErrorIdentidad(ctx, "RefrescarSesion", err)
	}
	return out, nil
}

// RevocarSesion revoca la sesión dada.
func (a *BFFIdentidadEnProceso) RevocarSesion(ctx context.Context, id idclient.Id) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()

	if err := a.sesion.RevocarSesion(ctx, id); err != nil {
		return traducirErrorIdentidad(ctx, "RevocarSesion", err)
	}
	return nil
}

// ListarSesionesRevocadas lee el snapshot de reconstrucción de tombstones.
func (a *BFFIdentidadEnProceso) ListarSesionesRevocadas(ctx context.Context) (*idclient.PaginaRevocaciones, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()

	out, err := a.sesion.ListarSesionesRevocadas(ctx)
	if err != nil {
		return nil, traducirErrorIdentidad(ctx, "ListarSesionesRevocadas", err)
	}
	return out, nil
}

// ObtenerSesion consulta el estado de una sesión (fallback ADR 26 §5).
func (a *BFFIdentidadEnProceso) ObtenerSesion(ctx context.Context, id idclient.Id) (*idclient.EstadoSesion, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()

	out, err := a.sesion.ObtenerSesion(ctx, id)
	if err != nil {
		return nil, traducirErrorIdentidad(ctx, "ObtenerSesion", err)
	}
	return out, nil
}

// traducirErrorIdentidad mantiene la taxonomía del adaptador HTTP de bff: un
// vencimiento del presupuesto o cancelación del contexto se traduce a Degradado;
// cualquier otro error de la fachada ya es un PlatformError categorizado y se
// propaga.
func traducirErrorIdentidad(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "bff: %s contra identidad (en proceso) falló", op)
	}
	return err
}
