package adapters

import (
	"context"
	"testing"

	msgclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/mensajeria"
	platformerrors "github.com/AI27-Tech/pf-pkg-platform-go/errors"
	mensajeria "github.com/AI27-Tech/pf-svc-mensajeria"
)

// conversacionesFake implementa solo los métodos que estas pruebas usan; el
// resto de mensajeria.ConversacionesAPI queda en la interfaz embebida (nil).
type conversacionesFake struct {
	mensajeria.ConversacionesAPI
	err     error
	idVisto msgclient.Id
}

func (f *conversacionesFake) EnviarMensaje(_ context.Context, id msgclient.Id, body msgclient.EnviarMensajeEntrada) (*msgclient.Mensaje, error) {
	f.idVisto = id
	if f.err != nil {
		return nil, f.err
	}
	return &msgclient.Mensaje{ConversacionId: id, Cuerpo: body.Cuerpo}, nil
}

func (f *conversacionesFake) ListarTrabajoBandeja(context.Context, msgclient.ListarTrabajoMiBandejaParams) (*msgclient.TrabajoBandeja, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &msgclient.TrabajoBandeja{}, nil
}

// TestBFFMensajeriaConversacion: EnviarMensaje llega a mensajeria con el id sin
// cambio (paridad con HTTP) y la bandeja propaga la categoría del error.
func TestBFFMensajeriaConversacion(t *testing.T) {
	fake := &conversacionesFake{}
	ad := NewBFFMensajeria(nil, fake)
	id := msgclient.Id{1}

	out, err := ad.EnviarMensaje(context.Background(), "", id, msgclient.EnviarMensajeEntrada{Cuerpo: "hola"})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if out.Cuerpo != "hola" || out.ConversacionId != id || fake.idVisto != id {
		t.Fatalf("mensaje = %+v, id = %v; se esperaba hola sobre %v", out, fake.idVisto, id)
	}

	ad = NewBFFMensajeria(nil, &conversacionesFake{err: platformerrors.NewProhibido("mensajeria: fuera del pool")})
	_, err = ad.ListarTrabajoBandeja(context.Background(), "", msgclient.ListarTrabajoMiBandejaParams{})
	if got := platformerrors.AsCategory(err); got != platformerrors.CategoryProhibido {
		t.Fatalf("categoría = %v; se esperaba Prohibido", got)
	}
}

// TestBFFMensajeriaSinRutaDevuelveNoEncontrado: atendida y recibos no tienen
// caso de uso en mensajeria; en proceso devuelven NoEncontrado, lo mismo que el
// adaptador HTTP de bff traduce del 404 de mensajeria.
func TestBFFMensajeriaSinRutaDevuelveNoEncontrado(t *testing.T) {
	ad := NewBFFMensajeria(nil, nil)
	ctx := context.Background()
	casos := map[string]error{
		"MarcarAtendida":           ad.MarcarAtendida(ctx, "", msgclient.Id{}, msgclient.VersionEsperadaEntrada{}),
		"RegistrarReciboEntregado": ad.RegistrarReciboEntregado(ctx, "", msgclient.Id{}, msgclient.ReciboEntrada{}),
		"RegistrarReciboLeido":     ad.RegistrarReciboLeido(ctx, "", msgclient.Id{}, msgclient.ReciboEntrada{}),
	}
	for op, err := range casos {
		if got := platformerrors.AsCategory(err); got != platformerrors.CategoryNoEncontrado {
			t.Errorf("%s: categoría = %v; se esperaba NoEncontrado", op, got)
		}
		if want := "bff: " + op + ": no encontrado aguas abajo"; err == nil || err.Error() != want {
			t.Errorf("%s: error = %v; se esperaba %q", op, err, want)
		}
	}
}
