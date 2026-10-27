package adapters

import (
	"context"
	"errors"

	msgclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/mensajeria"
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
//  2. Misma taxonomía de error: mensajeria.EmergenciasAPI ya devuelve errores de
//     la plataforma categorizados (autoriza y traduce internamente), que se
//     propagan; un vencimiento o cancelación del contexto se traduce a Degradado.
//  3. Tipos de pf-pkg-contracts (msgclient), nunca tipos de internal/ de mensajeria.
//
// El parámetro bearer del puerto es para el adaptador HTTP (propaga el header);
// en proceso los claims ya viajan en el ctx (el middleware del bff los puso), y
// mensajeria.EmergenciasAPI los lee de ahí, igual que BFFCatalogosEnProceso.
type BFFMensajeriaEnProceso struct {
	api mensajeria.EmergenciasAPI
}

var _ bff.MensajeriaPort = (*BFFMensajeriaEnProceso)(nil)

// NewBFFMensajeria construye el adaptador en proceso sobre la superficie
// Emergencias() de mensajeria. La raíz lo inyecta en bff.Deps.Mensajeria.
func NewBFFMensajeria(api mensajeria.EmergenciasAPI) *BFFMensajeriaEnProceso {
	return &BFFMensajeriaEnProceso{api: api}
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

// traducirErrorMensajeria mantiene la taxonomía del adaptador HTTP de bff: un
// vencimiento del presupuesto o cancelación del contexto se traduce a Degradado;
// cualquier otro error de mensajeria.EmergenciasAPI ya es un PlatformError
// categorizado y se propaga.
func traducirErrorMensajeria(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "bff: %s contra mensajeria (en proceso) falló", op)
	}
	return err
}
