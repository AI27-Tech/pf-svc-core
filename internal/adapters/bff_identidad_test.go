package adapters

import (
	"context"
	"strings"
	"testing"

	idclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/identidad"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	identidad "github.com/AI27-Tech/pf-svc-identidad"
)

// TestBFFIdentidadFijarClientesSujeto (DEC-149 #18): el adaptador delega en la
// fachada de gobierno de identidad y conserva su taxonomía. La fachada sin
// construir responde Degradado desde identidad, sin panic, y el error llega tal
// cual; con el contexto ya cancelado, el adaptador lo envuelve con su operación,
// como el resto de las delegaciones de gobierno.
func TestBFFIdentidadFijarClientesSujeto(t *testing.T) {
	a := NewBFFIdentidad(identidad.ConsultasAPI{})
	body := idclient.ClientesSujetoEntrada{ClientesIds: []idclient.Id{{}}}

	err := a.FijarClientesSujeto(context.Background(), idclient.Id{}, body)
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v (err %v); se esperaba Degradado", got, err)
	}
	if !strings.Contains(err.Error(), "identidad: el gobierno en proceso no está construido") {
		t.Fatalf("el error no viene de la fachada de gobierno de identidad: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = a.FijarClientesSujeto(ctx, idclient.Id{}, body)
	if !strings.Contains(err.Error(), "bff: FijarClientesSujeto contra identidad (en proceso) falló") {
		t.Fatalf("con el contexto cancelado, el adaptador no tradujo el error: %v", err)
	}
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryDegradado {
		t.Fatalf("categoría = %v; se esperaba Degradado", got)
	}
}
