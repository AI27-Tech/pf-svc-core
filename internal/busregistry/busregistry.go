// Package busregistry traduce el registro contractual de tipos de evento
// (api-client-go/registro) al molde agnóstico del bus (bus.TipoRegistrado) y
// construye el bus.Registry que el adaptador del bus necesita.
//
// Vive en core porque core es la raíz de composición: es quien importa el
// contrato y cablea los módulos (DEC-050). platform-go a propósito no conoce
// nombres de evento; core le pasa los datos. El objetivo de diseño es que esta
// conversión sea trivial: cuando un módulo se extraiga a su propio proceso con
// su cmd/<modulo>/api (ADR 17, invariante I5), basta repetir la llamada a
// Nuevo() con la misma lista del contrato, sin tocar lógica de adaptador.
package busregistry

import (
	"github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/registro"
	"github.com/AI27-Tech/pf-pkg-platform-go/bus"
)

// Nuevo construye el bus.Registry de core a partir del registro generado del
// contrato. Es la línea conceptual única de cableado: mapea cada entrada del
// contrato a bus.TipoRegistrado (solo Tipo y Version, que es lo que el Registry
// consume) y delega en bus.NuevoRegistry. La versión del contrato es uint32; el
// molde del bus la lleva como int.
func Nuevo() bus.Registry {
	registrados := make([]bus.TipoRegistrado, 0, len(registro.Tipos))
	for _, t := range registro.Tipos {
		registrados = append(registrados, bus.TipoRegistrado{
			Tipo:    t.Tipo,
			Version: int(t.Version),
		})
	}
	return bus.NuevoRegistry(registrados)
}
