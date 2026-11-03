package adapters

import (
	"context"
	"errors"

	msgclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/mensajeria"
	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	bff "github.com/AI27-Tech/pf-svc-bff"
	mensajeria "github.com/AI27-Tech/pf-svc-mensajeria"
)

// BFFMensajeriaEnProceso es el adaptador EN PROCESO de la arista bff → mensajeria
// (puerto bff.MensajeriaPort). Vive en pf-svc-core porque es el único repo que ve
// a bff y a mensajeria a la vez (00 regla 2); el adaptador HTTP del mismo puerto
// vive en pf-svc-bff/adapters/http y no se cablea cuando mensajeria está
// co-ubicada.
//
// Paridad con el adaptador HTTP de bff (I3, ADR 17):
//
//  1. Mismo presupuesto de tiempo: bff.PresupuestoMensajeria, el valor que
//     exporta el consumidor.
//  2. Misma taxonomía de error: mensajeria.EmergenciasAPI y
//     mensajeria.ConversacionesAPI ya devuelven errores de la plataforma
//     categorizados (autorizan y traducen internamente), que se propagan; un
//     vencimiento o cancelación del contexto se traduce a Degradado.
//  3. Tipos de pf-pkg-contracts (msgclient), nunca tipos de internal/ de mensajeria.
//
// El parámetro bearer del puerto es para el adaptador HTTP (propaga el header);
// en proceso los claims ya viajan en el ctx (el middleware del bff los puso), y
// mensajeria.EmergenciasAPI y mensajeria.ConversacionesAPI los leen de ahí, igual
// que BFFCatalogosEnProceso.
//
// ObtenerConversacion, ListarMensajes y EnviarMensaje los usa el bff con dos
// tipos de id: el portal cliente (/viajes/{id}/conversacion, solo audiencia
// Cliente) entrega el viaje_id y la superficie interna (/conversaciones/{id},
// solo Interno) entrega el conversacion_id. Con audiencia Cliente el adaptador
// resuelve primero la conversación del viaje (obtenerConversacionDeViaje,
// DEC-029) y sigue por id; con audiencia interna el id pasa sin cambio. El resto
// de los ids pasa sin cambio, igual que por HTTP.
type BFFMensajeriaEnProceso struct {
	api  mensajeria.EmergenciasAPI
	conv mensajeria.ConversacionesAPI
}

var _ bff.MensajeriaPort = (*BFFMensajeriaEnProceso)(nil)

// NewBFFMensajeria construye el adaptador en proceso sobre las superficies
// Emergencias() y Conversaciones() de mensajeria. La raíz lo inyecta en
// bff.Deps.Mensajeria.
func NewBFFMensajeria(api mensajeria.EmergenciasAPI, conv mensajeria.ConversacionesAPI) *BFFMensajeriaEnProceso {
	return &BFFMensajeriaEnProceso{api: api, conv: conv}
}

func (a *BFFMensajeriaEnProceso) ListarEmergencias(ctx context.Context, _ string, viajeID msgclient.Id) (*msgclient.PaginaEmergencias, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.ListarEmergencias(ctx, viajeID)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ListarEmergencias", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ActivarEmergencia(ctx context.Context, _ string, viajeID msgclient.Id, body msgclient.ActivarEmergenciaEntrada) (*msgclient.CasoEmergencia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	// El puerto del bff no transporta la señal de idempotencia (200 vs 201); se
	// descarta aquí, igual que la perdería una llamada HTTP que solo ve el cuerpo.
	out, _, err := a.api.ActivarEmergencia(ctx, viajeID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ActivarEmergencia", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) AgregarNotaEmergencia(ctx context.Context, _ string, viajeID, emergenciaID msgclient.Id, body msgclient.NotaEmergenciaEntrada) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.api.AgregarNotaEmergencia(ctx, viajeID, emergenciaID, body); err != nil {
		return traducirErrorMensajeria(ctx, "AgregarNotaEmergencia", err)
	}
	return nil
}

func (a *BFFMensajeriaEnProceso) ReclamarEmergencia(ctx context.Context, _ string, viajeID, emergenciaID msgclient.Id, body msgclient.ReclamarEmergenciaJSONRequestBody) (*msgclient.Reclamo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.ReclamarEmergencia(ctx, viajeID, emergenciaID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ReclamarEmergencia", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) LiberarReclamoEmergencia(ctx context.Context, _ string, viajeID, emergenciaID, reclamoID msgclient.Id, params msgclient.LiberarReclamoEmergenciaParams) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.api.LiberarReclamoEmergencia(ctx, viajeID, emergenciaID, reclamoID, params.VersionEsperada); err != nil {
		return traducirErrorMensajeria(ctx, "LiberarReclamoEmergencia", err)
	}
	return nil
}

func (a *BFFMensajeriaEnProceso) TransferirEmergencia(ctx context.Context, _ string, viajeID, emergenciaID msgclient.Id, body msgclient.TransferirEmergenciaJSONRequestBody) (*msgclient.Reclamo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.TransferirEmergencia(ctx, viajeID, emergenciaID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "TransferirEmergencia", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ResolverEmergencia(ctx context.Context, _ string, viajeID, emergenciaID msgclient.Id, body msgclient.ResolverEmergenciaJSONRequestBody) (*msgclient.CasoEmergencia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.api.ResolverEmergencia(ctx, viajeID, emergenciaID, body); err != nil {
		return nil, traducirErrorMensajeria(ctx, "ResolverEmergencia", err)
	}
	// El puerto devuelve el caso; el dominio de resolución no lo recompone (la
	// transición es a resuelta). Se devuelve el caso con solo el id, misma forma
	// que la respuesta del carril HTTP, que tampoco recompone el caso completo.
	return &msgclient.CasoEmergencia{Id: emergenciaID}, nil
}

func (a *BFFMensajeriaEnProceso) CerrarEmergencia(ctx context.Context, _ string, viajeID, emergenciaID msgclient.Id) (*msgclient.CasoEmergencia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.api.CerrarEmergencia(ctx, viajeID, emergenciaID); err != nil {
		return nil, traducirErrorMensajeria(ctx, "CerrarEmergencia", err)
	}
	return &msgclient.CasoEmergencia{Id: emergenciaID}, nil
}

// --- Conversación del viaje (E9-T8) y bandeja interna (E8-T15) ---------------

// conversacionID traduce el id que entrega el bff al conversacion_id de
// mensajeria. Solo la audiencia Cliente llega con viaje_id: el bff v0.1.13
// rechaza en sus handlers a la audiencia que no corresponde a cada ruta.
// shortcut: el tipo de id se infiere de la audiencia, no del puerto; separar los
// métodos de viaje en bff.MensajeriaPort y borrar este despacho cuando exista.
func (a *BFFMensajeriaEnProceso) conversacionID(ctx context.Context, id msgclient.Id) (msgclient.Id, error) {
	if c := auth.ClaimsFromContext(ctx); c == nil || c.Audiencia != auth.AudienciaCliente {
		return id, nil
	}
	conv, err := a.conv.ObtenerConversacionDeViaje(ctx, id)
	if err != nil {
		return msgclient.Id{}, err
	}
	return conv.Id, nil
}

func (a *BFFMensajeriaEnProceso) ObtenerConversacion(ctx context.Context, _ string, id msgclient.Id) (*msgclient.Conversacion, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	convID, err := a.conversacionID(ctx, id)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ObtenerConversacion", err)
	}
	out, err := a.conv.ObtenerConversacion(ctx, convID)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ObtenerConversacion", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ListarMensajes(ctx context.Context, _ string, id msgclient.Id, params msgclient.ListarMensajesParams) (*msgclient.PaginaMensajes, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	convID, err := a.conversacionID(ctx, id)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ListarMensajes", err)
	}
	out, err := a.conv.ListarMensajes(ctx, convID, params)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ListarMensajes", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) EnviarMensaje(ctx context.Context, _ string, id msgclient.Id, body msgclient.EnviarMensajeEntrada) (*msgclient.Mensaje, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	convID, err := a.conversacionID(ctx, id)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "EnviarMensaje", err)
	}
	out, err := a.conv.EnviarMensaje(ctx, convID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "EnviarMensaje", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ListarTrabajoBandeja(ctx context.Context, _ string, params msgclient.ListarTrabajoMiBandejaParams) (*msgclient.TrabajoBandeja, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.conv.ListarTrabajoBandeja(ctx, params)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ListarTrabajoBandeja", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ReclamarConversacion(ctx context.Context, _ string, conversacionID msgclient.Id, body msgclient.VersionEsperadaEntrada) (*msgclient.Reclamo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.conv.ReclamarConversacion(ctx, conversacionID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ReclamarConversacion", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) LiberarReclamo(ctx context.Context, _ string, conversacionID, reclamoID msgclient.Id, params msgclient.LiberarReclamoParams) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.conv.LiberarReclamo(ctx, conversacionID, reclamoID, params); err != nil {
		return traducirErrorMensajeria(ctx, "LiberarReclamo", err)
	}
	return nil
}

func (a *BFFMensajeriaEnProceso) TransferirConversacion(ctx context.Context, _ string, conversacionID msgclient.Id, body msgclient.TransferenciaEntrada) (*msgclient.Reclamo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.conv.TransferirConversacion(ctx, conversacionID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "TransferirConversacion", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) MarcarAtendida(ctx context.Context, _ string, conversacionID msgclient.Id, body msgclient.VersionEsperadaEntrada) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.conv.MarcarAtendida(ctx, conversacionID, body); err != nil {
		return traducirErrorMensajeria(ctx, "MarcarAtendida", err)
	}
	return nil
}

func (a *BFFMensajeriaEnProceso) RegistrarReciboEntregado(ctx context.Context, _ string, conversacionID msgclient.Id, body msgclient.ReciboEntrada) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.conv.RegistrarReciboEntregado(ctx, conversacionID, body); err != nil {
		return traducirErrorMensajeria(ctx, "RegistrarReciboEntregado", err)
	}
	return nil
}

func (a *BFFMensajeriaEnProceso) RegistrarReciboLeido(ctx context.Context, _ string, conversacionID msgclient.Id, body msgclient.ReciboEntrada) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.conv.RegistrarReciboLeido(ctx, conversacionID, body); err != nil {
		return traducirErrorMensajeria(ctx, "RegistrarReciboLeido", err)
	}
	return nil
}

// traducirErrorMensajeria mantiene la taxonomía del adaptador HTTP de bff: un
// vencimiento del presupuesto o cancelación del contexto se traduce a Degradado;
// cualquier otro error de mensajeria.EmergenciasAPI o ConversacionesAPI ya es un
// PlatformError
// categorizado y se propaga.
func traducirErrorMensajeria(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "bff: %s contra mensajeria (en proceso) falló", op)
	}
	return err
}
