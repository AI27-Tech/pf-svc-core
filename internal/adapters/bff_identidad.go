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
	sesion   identidad.SesionConsultas
	gobierno identidad.GobiernoConsultas
}

var _ bff.IdentidadPort = (*BFFIdentidadEnProceso)(nil)

// NewBFFIdentidad construye el adaptador en proceso sobre las sub-fachadas de
// sesión y de gobierno de identidad. La raíz lo inyecta en bff.Deps.Identidad.
func NewBFFIdentidad(c identidad.ConsultasAPI) *BFFIdentidadEnProceso {
	return &BFFIdentidadEnProceso{sesion: c.Sesion(), gobierno: c.Gobierno()}
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

// --- Gobierno de usuarios, alcance y tenants (tracer.40, E8-T19) ---
// Delegan en identidad.Consultas().Gobierno(), que resuelve el actor desde los
// claims del ctx (los deja el middleware de auth del bff) y devuelve sus 403/409
// con el `codigo` del HTTP. Los errores se propagan tal cual.

// ListarSujetos es listarSujetos.
func (a *BFFIdentidadEnProceso) ListarSujetos(ctx context.Context, params idclient.ListarSujetosParams) (*idclient.PaginaSujetos, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	out, err := a.gobierno.ListarSujetos(ctx, params)
	return out, traducirErrorIdentidad(ctx, "ListarSujetos", err)
}

// AltaSujeto es altaSujeto.
func (a *BFFIdentidadEnProceso) AltaSujeto(ctx context.Context, body idclient.AltaSujetoEntrada) (*idclient.Sujeto, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	out, err := a.gobierno.AltaSujeto(ctx, body)
	return out, traducirErrorIdentidad(ctx, "AltaSujeto", err)
}

// ObtenerSujeto es obtenerSujeto.
func (a *BFFIdentidadEnProceso) ObtenerSujeto(ctx context.Context, id idclient.Id) (*idclient.SujetoDetalle, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	out, err := a.gobierno.ObtenerSujeto(ctx, id)
	return out, traducirErrorIdentidad(ctx, "ObtenerSujeto", err)
}

// EditarSujeto es editarSujeto.
func (a *BFFIdentidadEnProceso) EditarSujeto(ctx context.Context, id idclient.Id, body idclient.EdicionSujetoEntrada) (*idclient.SujetoDetalle, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	out, err := a.gobierno.EditarSujeto(ctx, id, body)
	return out, traducirErrorIdentidad(ctx, "EditarSujeto", err)
}

// DesactivarSujeto es desactivarSujeto.
func (a *BFFIdentidadEnProceso) DesactivarSujeto(ctx context.Context, id idclient.Id) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	return traducirErrorIdentidad(ctx, "DesactivarSujeto", a.gobierno.DesactivarSujeto(ctx, id))
}

// ReactivarSujeto es reactivarSujeto.
func (a *BFFIdentidadEnProceso) ReactivarSujeto(ctx context.Context, id idclient.Id) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	return traducirErrorIdentidad(ctx, "ReactivarSujeto", a.gobierno.ReactivarSujeto(ctx, id))
}

// FijarRolSujeto es fijarRolSujeto.
func (a *BFFIdentidadEnProceso) FijarRolSujeto(ctx context.Context, id idclient.Id, body idclient.AsignacionRolEntrada) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	return traducirErrorIdentidad(ctx, "FijarRolSujeto", a.gobierno.FijarRolSujeto(ctx, id, body))
}

// RevocarSesionesSujeto es revocarSesionesSujeto.
func (a *BFFIdentidadEnProceso) RevocarSesionesSujeto(ctx context.Context, id idclient.Id, body idclient.RevocacionSesionesEntrada) (*idclient.RevocacionSesionesResultado, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	out, err := a.gobierno.RevocarSesionesSujeto(ctx, id, body)
	return out, traducirErrorIdentidad(ctx, "RevocarSesionesSujeto", err)
}

// ObtenerAlcanceSujeto es obtenerAlcanceSujeto.
func (a *BFFIdentidadEnProceso) ObtenerAlcanceSujeto(ctx context.Context, id idclient.Id) (*idclient.AlcanceSujeto, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	out, err := a.gobierno.ObtenerAlcanceSujeto(ctx, id)
	return out, traducirErrorIdentidad(ctx, "ObtenerAlcanceSujeto", err)
}

// FijarUnidadesSujeto es fijarUnidadesSujeto.
func (a *BFFIdentidadEnProceso) FijarUnidadesSujeto(ctx context.Context, id idclient.Id, body idclient.UnidadesSujetoEntrada) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	return traducirErrorIdentidad(ctx, "FijarUnidadesSujeto", a.gobierno.FijarUnidadesSujeto(ctx, id, body))
}

// FijarSubordinadosSujeto es fijarSubordinadosSujeto.
func (a *BFFIdentidadEnProceso) FijarSubordinadosSujeto(ctx context.Context, id idclient.Id, body idclient.SubordinadosSujetoEntrada) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	return traducirErrorIdentidad(ctx, "FijarSubordinadosSujeto", a.gobierno.FijarSubordinadosSujeto(ctx, id, body))
}

// ListarTenants es listarTenants.
func (a *BFFIdentidadEnProceso) ListarTenants(ctx context.Context, params idclient.ListarTenantsParams) (*idclient.PaginaTenants, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	out, err := a.gobierno.ListarTenants(ctx, params)
	return out, traducirErrorIdentidad(ctx, "ListarTenants", err)
}

// CrearTenant es crearTenant, idempotente por id: creado es false si ya existía.
func (a *BFFIdentidadEnProceso) CrearTenant(ctx context.Context, id idclient.Id, body idclient.TenantEntrada) (*idclient.Tenant, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	out, creado, err := a.gobierno.CrearTenant(ctx, id, body)
	return out, creado, traducirErrorIdentidad(ctx, "CrearTenant", err)
}

// DesactivarTenant es desactivarTenant.
func (a *BFFIdentidadEnProceso) DesactivarTenant(ctx context.Context, id idclient.Id) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoIdentidad)
	defer cancel()
	return traducirErrorIdentidad(ctx, "DesactivarTenant", a.gobierno.DesactivarTenant(ctx, id))
}

// traducirErrorIdentidad mantiene la taxonomía del adaptador HTTP de bff: un
// vencimiento del presupuesto o cancelación del contexto se traduce a Degradado;
// cualquier otro error de la fachada ya es un PlatformError categorizado y se
// propaga. Sin error devuelve nil, para que las delegaciones de gobierno lo
// llamen sin rama.
func traducirErrorIdentidad(ctx context.Context, op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "bff: %s contra identidad (en proceso) falló", op)
	}
	return err
}
