package busregistry_test

import (
	"testing"

	"github.com/AI27-Tech/pf-svc-core/internal/busregistry"
)

// TestNuevoAceptaTiposDelContrato verifica que el Registry construido desde el
// registro del contrato acepta tipos reales (en su versión 1) y rechaza un tipo
// inventado y una versión equivocada de un tipo conocido.
func TestNuevoAceptaTiposDelContrato(t *testing.T) {
	reg := busregistry.Nuevo()

	casos := []struct {
		tipo     string
		version  int
		esperado bool
	}{
		{"viaje.estado_cambiado", 1, true},
		{"sesion.revocada", 1, true},
		{"catalogo.actualizado", 1, true},
		{"alcance.cambiado", 1, true}, // DEC-149 #20: lo publica identidad, lo consumen operacion, reportes y mensajeria
		// Ola de solicitudes (tracer.45/46, DEC-148): los consume mensajeria
		// (mensajeria-conversacion-solicitud y mensajeria-roles). Sin estar en el
		// registro, el bus rechaza publicarlos y suscribirse.
		{"solicitud.creada", 1, true},
		{"solicitud.estado_cambiado", 1, true},
		{"rol.asignado", 1, true},
		{"rol.revocado", 1, true},
		{"sujeto.desactivado", 1, true},
		{"sujeto.reactivado", 1, true},
		{"tipo.inventado", 1, false},
		{"viaje.estado_cambiado", 99, false},
	}
	for _, c := range casos {
		if got := reg.Allowed(c.tipo, c.version); got != c.esperado {
			t.Errorf("Allowed(%q, %d) = %v, se esperaba %v", c.tipo, c.version, got, c.esperado)
		}
	}
}
