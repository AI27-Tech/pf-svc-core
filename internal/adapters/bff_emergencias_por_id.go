package adapters

import (
	"context"

	catclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/catalogos"
	msgclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/mensajeria"
	opclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/operacion"
	bff "github.com/AI27-Tech/pf-svc-bff"
)

// --- Bandeja de Reacción y caso por su id (tracer.34, E8-T18) ---------------
// Misma paridad I3 que el resto de BFFMensajeriaEnProceso: presupuesto del bff,
// errores de la plataforma que mensajeria ya categoriza, y vencimiento del
// contexto como Degradado.

func (a *BFFMensajeriaEnProceso) ListarBandejaEmergencias(ctx context.Context, _ string, params msgclient.ListarBandejaEmergenciasParams) (*msgclient.PaginaBandejaEmergencias, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.ListarBandejaEmergencias(ctx, params)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ListarBandejaEmergencias", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ObtenerEmergencia(ctx context.Context, _ string, emergenciaID msgclient.Id) (*msgclient.CasoEmergencia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.ObtenerEmergencia(ctx, emergenciaID)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ObtenerEmergencia", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ListarNotasEmergenciaPorId(ctx context.Context, _ string, emergenciaID msgclient.Id, params msgclient.ListarNotasEmergenciaPorIdParams) (*msgclient.PaginaNotasEmergencia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.ListarNotasEmergenciaPorId(ctx, emergenciaID, params)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ListarNotasEmergenciaPorId", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ReclamarEmergenciaPorId(ctx context.Context, _ string, emergenciaID msgclient.Id, body msgclient.VersionEsperadaEntrada) (*msgclient.Reclamo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.ReclamarEmergenciaPorId(ctx, emergenciaID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ReclamarEmergenciaPorId", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) LiberarReclamoEmergenciaPorId(ctx context.Context, _ string, emergenciaID, reclamoID msgclient.Id, params msgclient.LiberarReclamoEmergenciaPorIdParams) error {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	if err := a.api.LiberarReclamoEmergenciaPorId(ctx, emergenciaID, reclamoID, params); err != nil {
		return traducirErrorMensajeria(ctx, "LiberarReclamoEmergenciaPorId", err)
	}
	return nil
}

func (a *BFFMensajeriaEnProceso) TransferirEmergenciaPorId(ctx context.Context, _ string, emergenciaID msgclient.Id, body msgclient.TransferenciaEntrada) (*msgclient.Reclamo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.TransferirEmergenciaPorId(ctx, emergenciaID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "TransferirEmergenciaPorId", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) ResolverEmergenciaPorId(ctx context.Context, _ string, emergenciaID msgclient.Id, body msgclient.ResolucionEmergenciaEntrada) (*msgclient.EmergenciaReaccion, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.ResolverEmergenciaPorId(ctx, emergenciaID, body)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "ResolverEmergenciaPorId", err)
	}
	return out, nil
}

func (a *BFFMensajeriaEnProceso) CerrarEmergenciaPorId(ctx context.Context, _ string, emergenciaID msgclient.Id) (*msgclient.CasoEmergencia, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoMensajeria)
	defer cancel()
	out, err := a.api.CerrarEmergenciaPorId(ctx, emergenciaID)
	if err != nil {
		return nil, traducirErrorMensajeria(ctx, "CerrarEmergenciaPorId", err)
	}
	return out, nil
}

// --- Lecturas del detalle de Reacción (E8-T18) -----------------------------

func (a *BFFOperacionEnProceso) ObtenerAsignacion(ctx context.Context, id opclient.Id) (*opclient.EscoltaVigente, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoOperacion)
	defer cancel()
	out, err := a.consultas.ObtenerAsignacion(ctx, id)
	if err != nil {
		return nil, traducirErrorOperacion(ctx, "ObtenerAsignacion", err)
	}
	return out, nil
}

func (a *BFFCatalogosEnProceso) ObtenerCliente(ctx context.Context, _ string, id catclient.Id) (*catclient.Cliente, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()
	out, err := a.consultas.ObtenerCliente(ctx, id)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ObtenerCliente", err)
	}
	return out, nil
}

// ObtenerVehiculo y ObtenerCustodio leen la versión vigente (a_fecha nil), igual
// que el adaptador HTTP del bff.
func (a *BFFCatalogosEnProceso) ObtenerVehiculo(ctx context.Context, _ string, id catclient.Id) (*catclient.Vehiculo, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()
	out, err := a.consultas.ObtenerVehiculo(ctx, id, nil)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ObtenerVehiculo", err)
	}
	return out, nil
}

func (a *BFFCatalogosEnProceso) ObtenerCustodio(ctx context.Context, _ string, id catclient.Id) (*catclient.Custodio, error) {
	ctx, cancel := context.WithTimeout(ctx, bff.PresupuestoCatalogos)
	defer cancel()
	out, err := a.consultas.ObtenerCustodio(ctx, id, nil)
	if err != nil {
		return nil, traducirErrorBFFCatalogos(ctx, "ObtenerCustodio", err)
	}
	return out, nil
}
