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
		{"tipo.inventado", 1, false},
		{"viaje.estado_cambiado", 99, false},
	}
	for _, c := range casos {
		if got := reg.Allowed(c.tipo, c.version); got != c.esperado {
			t.Errorf("Allowed(%q, %d) = %v, se esperaba %v", c.tipo, c.version, got, c.esperado)
		}
	}
}
