package adapters

import (
	"context"
	"errors"

	opclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/operacion"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	bff "github.com/AI27-Tech/pf-svc-bff"
	operacion "github.com/AI27-Tech/pf-svc-operacion"
)

// BFFOperacionEnProceso es el adaptador EN PROCESO de la arista bff → operacion
// (puerto bff.OperacionPort). Vive en pf-svc-core porque es el único repo que ve
// a bff y a operacion a la vez (00 regla 2); el adaptador HTTP del mismo puerto
// vive en pf-svc-bff/adapters/http y no se cablea cuando operacion está
// co-ubicada.
//
// Paridad con el adaptador HTTP de bff (I3, ADR 17):
//
//  1. Mismo presupuesto de tiempo: bff.PresupuestoOperacion, el valor que
//     exporta el consumidor. No se duplica el número.
//  2. Misma taxonomía de error: operacion.Consultas() devuelve errores de la
//     plataforma ya categorizados, que se propagan; un vencimiento o cancelación
//     del contexto se traduce a Degradado, igual que el DeTransporte del
//     adaptador HTTP de bff.
//  3. Tipos de pf-pkg-contracts (opclient), nunca tipos de internal/ de operacion.
//
// No abre conexiones: delega en operacion.Consultas(), que usa sus pools y roles.
type BFFOperacionEnProceso struct {
	consultas operacion.ConsultasAPI
}

var _ bff.OperacionPort = (*BFFOperacionEnProceso)(nil)

// NewBFFOperacion construye el adaptador en proceso sobre la superficie
// Consultas() de operacion. La raíz lo inyecta en bff.Deps.Operacion.
func NewBFFOperacion(c operacion.ConsultasAPI) *BFFOperacionEnProceso {
	return &BFFOperacionEnProceso{consultas: c}
}

// CrearViaje reenvía la creación. El estado inicial lo fija el rol del token en
// operacion (DEC-026); bff no decide.
func (a *BFFOperacionEnProceso) CrearViaje(ctx context.Context, body opclient.CrearViajeEntrada) (*opclient.Viaje, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.CrearViaje(ctx, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "CrearViaje", err)
	}
	return out, nil
}

// ListarViajes lista por alcance; el alcance (unidad/cliente) lo resuelve
// operacion en servidor (DEC-053).
func (a *BFFOperacionEnProceso) ListarViajes(ctx context.Context, params opclient.ListarViajesParams) (*opclient.PaginaViajes, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ListarViajes(ctx, params)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ListarViajes", err)
	}
	return out, nil
}

// ObtenerViaje lee el detalle para la composición de hitos (ADR 27).
func (a *BFFOperacionEnProceso) ObtenerViaje(ctx context.Context, id opclient.Id) (*opclient.ViajeDetalle, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ObtenerViaje(ctx, id)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ObtenerViaje", err)
	}
	return out, nil
}

// ListarBitacora lee UNA página del histórico de bitácora del viaje (E6-T3). La
// composición de la descarga agota las páginas (ADR 27: operacion es la fuente).
func (a *BFFOperacionEnProceso) ListarBitacora(ctx context.Context, id opclient.Id, params opclient.ListarBitacoraParams) (*opclient.PaginaBitacora, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ListarBitacora(ctx, id, params)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ListarBitacora", err)
	}
	return out, nil
}

// ConsultarAuditoria lee UNA página de la auditoría relevante del viaje (E6-T3).
// La composición de la descarga agota las páginas.
func (a *BFFOperacionEnProceso) ConsultarAuditoria(ctx context.Context, id opclient.Id, params opclient.ConsultarAuditoriaParams) (*opclient.PaginaAuditoria, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ConsultarAuditoria(ctx, id, params)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ConsultarAuditoria", err)
	}
	return out, nil
}

// RegistrarDescargaViaje pide a operacion que deje el rastro de auditoría de la
// descarga (E1-T7). El actor sale de los claims del ctx (que operacion lee); sin
// cuerpo. operacion escribe el registro en su propia transacción.
func (a *BFFOperacionEnProceso) RegistrarDescargaViaje(ctx context.Context, id opclient.Id) (*opclient.Viaje, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.RegistrarDescargaViaje(ctx, id)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "RegistrarDescargaViaje", err)
	}
	return out, nil
}

// traducirErrorOperacion mantiene la taxonomía del adaptador HTTP de bff: un
// vencimiento del presupuesto o cancelación del contexto se traduce a Degradado;
// cualquier otro error de operacion.Consultas() ya es un PlatformError
// categorizado y se propaga.
func traducirErrorOperacion(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "bff: %s contra operacion (en proceso) falló", op)
	}
	return err
}
