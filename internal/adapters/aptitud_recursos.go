package adapters

import (
	"context"
	"errors"

	catclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/catalogos"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
	operacion "github.com/AI27-Tech/pf-svc-operacion"
)

// AptitudRecursosEnProceso es el adaptador EN PROCESO de la arista
// operacion → catalogos (puerto operacion.AptitudRecursos). Vive en pf-svc-core
// porque es el único repo que ve a ambos módulos (00 regla 2, DEC-050 §2); el
// adaptador HTTP del mismo puerto vive en pf-svc-operacion/adapters/http y no se
// cablea cuando catalogos está co-ubicado.
//
// Paridad con el adaptador HTTP (I3, ADR 17 §"Dos adaptadores"):
//
//  1. Mismo presupuesto de tiempo: toma operacion.PresupuestoAptitud, el MISMO
//     valor que exporta el consumidor junto a su puerto. No se duplica el número.
//  2. Misma taxonomía de error: catalogos.Consultas() ya devuelve
//     *platformerrors.PlatformError categorizado, así que el error se propaga
//     tal cual; un vencimiento de contexto o cancelación se traduce a Degradado,
//     exactamente como lo hace traducirError del adaptador HTTP (la aptitud es
//     informativa y su indisponibilidad no gatea, DEC-055).
//  3. Tipos de pf-pkg-contracts (catclient), nunca tipos de internal/ de
//     catalogos (que además son inalcanzables).
//
// No abre conexiones: delega en catalogos.Consultas(), que usa SU pool y SU rol.
type AptitudRecursosEnProceso struct {
	consultas catalogos.ConsultasAPI
}

var _ operacion.AptitudRecursos = (*AptitudRecursosEnProceso)(nil)

// NewAptitudRecursos construye el adaptador en proceso sobre la superficie
// Consultas() de catalogos. La raíz lo inyecta en operacion.Deps.Aptitud.
func NewAptitudRecursos(consultas catalogos.ConsultasAPI) *AptitudRecursosEnProceso {
	return &AptitudRecursosEnProceso{consultas: consultas}
}

// AptitudVehiculo consulta la aptitud informativa de un vehículo-átomo (DEC-055),
// evaluada contra el contrato/SLA del cliente (tracer.4), con el mismo
// presupuesto de tiempo que el adaptador HTTP.
func (a *AptitudRecursosEnProceso) AptitudVehiculo(ctx context.Context, vehiculoID, clienteID catclient.Id) (*catclient.Aptitud, error) {
	ctx, cancel := context.WithTimeout(ctx, operacion.PresupuestoAptitud)
	defer cancel()

	apt, err := a.consultas.AptitudVehiculo(ctx, vehiculoID, clienteID)
	if err != nil {
		return nil, traducirError(ctx, "AptitudVehiculo", err)
	}
	return apt, nil
}

// AptitudCustodio consulta la aptitud informativa de un custodio (DEC-055),
// evaluada contra el contrato/SLA del cliente (tracer.4), con el mismo
// presupuesto de tiempo que el adaptador HTTP.
func (a *AptitudRecursosEnProceso) AptitudCustodio(ctx context.Context, custodioID, clienteID catclient.Id) (*catclient.Aptitud, error) {
	ctx, cancel := context.WithTimeout(ctx, operacion.PresupuestoAptitud)
	defer cancel()

	apt, err := a.consultas.AptitudCustodio(ctx, custodioID, clienteID)
	if err != nil {
		return nil, traducirError(ctx, "AptitudCustodio", err)
	}
	return apt, nil
}

// traducirError mantiene la misma taxonomía que el adaptador HTTP de operacion:
//   - un vencimiento del presupuesto o una cancelación del contexto se traduce a
//     Degradado (la aptitud es informativa y no gatea, DEC-055), igual que
//     traducirError del HTTP, que envuelve el timeout de red como Degradado;
//   - cualquier otro error de catalogos.Consultas() ya es un PlatformError
//     categorizado y se propaga sin recategorizar.
func traducirError(ctx context.Context, op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return platformerrors.NewDegradadoErr(err, "operacion: %s contra catalogos (en proceso) falló", op)
	}
	return err
}
