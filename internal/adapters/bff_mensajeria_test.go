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
	// llamada registra qué método de atención/recibos se invocó y con qué cuerpo.
	llamada string
	cuerpo  any
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

func (f *conversacionesFake) MarcarAtendida(_ context.Context, id msgclient.Id, body msgclient.VersionEsperadaEntrada) error {
	f.idVisto, f.llamada, f.cuerpo = id, "MarcarAtendida", body
	return f.err
}

func (f *conversacionesFake) RegistrarReciboEntregado(_ context.Context, id msgclient.Id, body msgclient.ReciboEntrada) error {
	f.idVisto, f.llamada, f.cuerpo = id, "RegistrarReciboEntregado", body
	return f.err
}

func (f *conversacionesFake) RegistrarReciboLeido(_ context.Context, id msgclient.Id, body msgclient.ReciboEntrada) error {
	f.idVisto, f.llamada, f.cuerpo = id, "RegistrarReciboLeido", body
	return f.err
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

// TestBFFMensajeriaAtencionYRecibos: atendida y recibos se reenvían a
// mensajeria.ConversacionesAPI con id y cuerpo sin cambio, y el error categorizado
// de mensajeria se propaga tal cual.
func TestBFFMensajeriaAtencionYRecibos(t *testing.T) {
	ctx := context.Background()
	id := msgclient.Id{7}
	version := msgclient.VersionEsperadaEntrada{VersionEsperada: 3}
	recibo := msgclient.ReciboEntrada{HastaSecuencia: 5}
	casos := []struct {
		op     string
		cuerpo any
		llamar func(*BFFMensajeriaEnProceso) error
	}{
		{"MarcarAtendida", version, func(ad *BFFMensajeriaEnProceso) error { return ad.MarcarAtendida(ctx, "", id, version) }},
		{"RegistrarReciboEntregado", recibo, func(ad *BFFMensajeriaEnProceso) error { return ad.RegistrarReciboEntregado(ctx, "", id, recibo) }},
		{"RegistrarReciboLeido", recibo, func(ad *BFFMensajeriaEnProceso) error { return ad.RegistrarReciboLeido(ctx, "", id, recibo) }},
	}
	for _, c := range casos {
		t.Run(c.op, func(t *testing.T) {
			fake := &conversacionesFake{}
			if err := c.llamar(NewBFFMensajeria(nil, fake)); err != nil {
				t.Fatalf("no se esperaba error: %v", err)
			}
			if fake.llamada != c.op || fake.idVisto != id || fake.cuerpo != c.cuerpo {
				t.Fatalf("llamada = %q id = %v cuerpo = %+v; se esperaba %s sobre %v con %+v", fake.llamada, fake.idVisto, fake.cuerpo, c.op, id, c.cuerpo)
			}

			conflicto := platformerrors.NewConflicto("mensajeria: versión obsoleta")
			if err := c.llamar(NewBFFMensajeria(nil, &conversacionesFake{err: conflicto})); err != conflicto {
				t.Fatalf("error = %v; se esperaba el conflicto de mensajeria sin cambio", err)
			}
		})
	}
}
