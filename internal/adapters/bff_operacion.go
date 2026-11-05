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

// TransicionarViaje reenvía una transición de estado (E9-T7 cliente, E8-T14
// interna). operacion es la única autoridad: valida la arista, el tenant y la
// facultad con los claims del ctx; bff no decide.
func (a *BFFOperacionEnProceso) TransicionarViaje(ctx context.Context, id opclient.Id, body opclient.TransicionEntrada) (*opclient.Viaje, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.TransicionarViaje(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "TransicionarViaje", err)
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

// --- Flujo interno del panel (bff como única puerta pública) ---------------
// Cada método reenvía al de operacion.Consultas() con el mismo presupuesto y la
// misma traducción de error que los anteriores. El actor (sujeto o rol) lo lee
// operacion de los claims del ctx.

// FijarEta reenvía la captura manual del ETA (ADR 20).
func (a *BFFOperacionEnProceso) FijarEta(ctx context.Context, id opclient.Id, body opclient.EtaEntrada) (*opclient.Viaje, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.FijarEta(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "FijarEta", err)
	}
	return out, nil
}

// AsignarRecursos reenvía la reserva atómica de la escolta (ADR 31).
func (a *BFFOperacionEnProceso) AsignarRecursos(ctx context.Context, id opclient.Id, body opclient.AsignacionEntrada) (*opclient.Asignacion, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.AsignarRecursos(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "AsignarRecursos", err)
	}
	return out, nil
}

// RegistrarEntradaBitacora reenvía una entrada append-only de bitácora.
func (a *BFFOperacionEnProceso) RegistrarEntradaBitacora(ctx context.Context, id opclient.Id, body opclient.EntradaBitacoraEntrada) (*opclient.EntradaBitacora, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.RegistrarEntradaBitacora(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "RegistrarEntradaBitacora", err)
	}
	return out, nil
}

// AbrirIncidencia reenvía la apertura de la condición INCIDENCIA.
func (a *BFFOperacionEnProceso) AbrirIncidencia(ctx context.Context, id opclient.Id, body opclient.IncidenciaEntrada) (*opclient.Incidencia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.AbrirIncidencia(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "AbrirIncidencia", err)
	}
	return out, nil
}

// ResolverIncidencia reenvía la resolución de una incidencia.
func (a *BFFOperacionEnProceso) ResolverIncidencia(ctx context.Context, id, iid opclient.Id, body opclient.ResolucionIncidenciaEntrada) (*opclient.Incidencia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ResolverIncidencia(ctx, id, iid, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ResolverIncidencia", err)
	}
	return out, nil
}

// ListarEstadias lee UNA página de estadías del viaje.
func (a *BFFOperacionEnProceso) ListarEstadias(ctx context.Context, id opclient.Id, params opclient.ListarEstadiasParams) (*opclient.PaginaEstadias, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ListarEstadias(ctx, id, params)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ListarEstadias", err)
	}
	return out, nil
}

// AbrirEstadia reenvía la apertura de una estadía.
func (a *BFFOperacionEnProceso) AbrirEstadia(ctx context.Context, id opclient.Id, body opclient.EstadiaEntrada) (*opclient.Estadia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.AbrirEstadia(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "AbrirEstadia", err)
	}
	return out, nil
}

// CerrarEstadia reenvía el cierre de una estadía.
func (a *BFFOperacionEnProceso) CerrarEstadia(ctx context.Context, id, eid opclient.Id) (*opclient.Estadia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.CerrarEstadia(ctx, id, eid)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "CerrarEstadia", err)
	}
	return out, nil
}

// ListarAdjuntos reenvía el listado de metadatos de la evidencia del viaje
// (E6-T4). No devuelve bytes.
func (a *BFFOperacionEnProceso) ListarAdjuntos(ctx context.Context, id opclient.Id) (*opclient.PaginaAdjuntos, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ListarAdjuntos(ctx, id)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ListarAdjuntos", err)
	}
	return out, nil
}

// CrearIntencionSubidaAdjunto reenvía la intención de subida; los bytes no pasan
// por el servicio (URL de vida corta del almacén de objetos).
func (a *BFFOperacionEnProceso) CrearIntencionSubidaAdjunto(ctx context.Context, id opclient.Id, body opclient.IntencionSubidaAdjuntoEntrada) (*opclient.IntencionSubidaAdjunto, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.CrearIntencionSubidaAdjunto(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "CrearIntencionSubidaAdjunto", err)
	}
	return out, nil
}

// DescargarAdjunto reenvía la autorización de descarga; devuelve una URL de vida
// corta, no los bytes.
func (a *BFFOperacionEnProceso) DescargarAdjunto(ctx context.Context, id, aid opclient.Id) (*opclient.DescargaAdjunto, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.DescargarAdjunto(ctx, id, aid)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "DescargarAdjunto", err)
	}
	return out, nil
}

// --- Flujo interno, grupo 2 (E8-T9/T10/T12/T13) -----------------------------

// ListarUnidades reenvía la lectura del roster para asignar la escolta.
func (a *BFFOperacionEnProceso) ListarUnidades(ctx context.Context, params opclient.ListarUnidadesParams) (*opclient.PaginaUnidades, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ListarUnidades(ctx, params)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ListarUnidades", err)
	}
	return out, nil
}

// CompletarFechaHora reenvía la captura de fecha_hora_servicio antes de aprobar.
func (a *BFFOperacionEnProceso) CompletarFechaHora(ctx context.Context, id opclient.Id, body opclient.FechaHoraEntrada) (*opclient.Viaje, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.CompletarFechaHora(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "CompletarFechaHora", err)
	}
	return out, nil
}

// ListarIncidencias reenvía la lectura del histórico de incidencias del viaje.
func (a *BFFOperacionEnProceso) ListarIncidencias(ctx context.Context, id opclient.Id, params opclient.ListarIncidenciasParams) (*opclient.PaginaIncidencias, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ListarIncidencias(ctx, id, params)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ListarIncidencias", err)
	}
	return out, nil
}

// ObtenerFicha reenvía la lectura de la ficha técnica.
func (a *BFFOperacionEnProceso) ObtenerFicha(ctx context.Context, id opclient.Id) (*opclient.FichaTecnica, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.ObtenerFicha(ctx, id)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ObtenerFicha", err)
	}
	return out, nil
}

// GuardarFicha reenvía la edición por partes de la ficha. El handler del bff ya
// tradujo `"tracto": null` a &Tracto{}, que es lo que Consultas() espera.
func (a *BFFOperacionEnProceso) GuardarFicha(ctx context.Context, id opclient.Id, body opclient.FichaTecnicaEntrada) (*opclient.FichaTecnica, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()

	out, err := a.consultas.GuardarFicha(ctx, id, body)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "GuardarFicha", err)
	}
	return out, nil
}
