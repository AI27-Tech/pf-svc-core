package adapters

import (
	"context"
	"testing"

	msgclient "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/mensajeria"
	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
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
	// convDeViaje es la conversación que resuelve ObtenerConversacionDeViaje;
	// errDeViaje, su error; viajeVisto, el viaje que se le pidió.
	convDeViaje msgclient.Id
	errDeViaje  error
	viajeVisto  msgclient.Id
}

func (f *conversacionesFake) ObtenerConversacionDeViaje(_ context.Context, viajeID msgclient.Id) (*msgclient.Conversacion, error) {
	f.viajeVisto = viajeID
	if f.errDeViaje != nil {
		return nil, f.errDeViaje
	}
	return &msgclient.Conversacion{Id: f.convDeViaje}, nil
}

func (f *conversacionesFake) ObtenerConversacion(_ context.Context, id msgclient.Id) (*msgclient.Conversacion, error) {
	f.idVisto = id
	return &msgclient.Conversacion{Id: id}, f.err
}

func (f *conversacionesFake) ListarMensajes(_ context.Context, id msgclient.Id, _ msgclient.ListarMensajesParams) (*msgclient.PaginaMensajes, error) {
	f.idVisto = id
	return &msgclient.PaginaMensajes{}, f.err
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

// TestBFFMensajeriaConversacionDeViaje: con audiencia Cliente el bff entrega el
// viaje_id y mensajeria recibe el conversacion_id resuelto; con audiencia interna
// el id pasa sin cambio; el 404 de la resolución se propaga tal cual y no se
// llama al método por id.
func TestBFFMensajeriaConversacionDeViaje(t *testing.T) {
	viaje, conv, nada := msgclient.Id{0xa}, msgclient.Id{0xc}, msgclient.Id{}
	cliente := auth.ContextWithClaims(context.Background(), &auth.Claims{Audiencia: auth.AudienciaCliente})
	interno := auth.ContextWithClaims(context.Background(), &auth.Claims{Audiencia: auth.AudienciaInterno})
	metodos := []struct {
		op     string
		llamar func(context.Context, *BFFMensajeriaEnProceso) error
	}{
		{"ObtenerConversacion", func(ctx context.Context, ad *BFFMensajeriaEnProceso) error {
			_, err := ad.ObtenerConversacion(ctx, "", viaje)
			return err
		}},
		{"ListarMensajes", func(ctx context.Context, ad *BFFMensajeriaEnProceso) error {
			_, err := ad.ListarMensajes(ctx, "", viaje, msgclient.ListarMensajesParams{})
			return err
		}},
		{"EnviarMensaje", func(ctx context.Context, ad *BFFMensajeriaEnProceso) error {
			_, err := ad.EnviarMensaje(ctx, "", viaje, msgclient.EnviarMensajeEntrada{Cuerpo: "hola"})
			return err
		}},
	}
	for _, m := range metodos {
		t.Run(m.op, func(t *testing.T) {
			fake := &conversacionesFake{convDeViaje: conv}
			if err := m.llamar(cliente, NewBFFMensajeria(nil, fake)); err != nil {
				t.Fatalf("cliente: no se esperaba error: %v", err)
			}
			if fake.viajeVisto != viaje || fake.idVisto != conv {
				t.Fatalf("cliente: viaje = %v, id = %v; se esperaba viaje %v y conversación %v", fake.viajeVisto, fake.idVisto, viaje, conv)
			}

			fake = &conversacionesFake{convDeViaje: conv}
			if err := m.llamar(interno, NewBFFMensajeria(nil, fake)); err != nil {
				t.Fatalf("interno: no se esperaba error: %v", err)
			}
			if fake.viajeVisto != nada || fake.idVisto != viaje {
				t.Fatalf("interno: viaje = %v, id = %v; se esperaba el id %v sin resolver", fake.viajeVisto, fake.idVisto, viaje)
			}

			noEncontrado := platformerrors.NewNoEncontrado("mensajeria: el viaje no tiene conversación")
			fake = &conversacionesFake{errDeViaje: noEncontrado}
			if err := m.llamar(cliente, NewBFFMensajeria(nil, fake)); err != noEncontrado {
				t.Fatalf("404: error = %v; se esperaba el NoEncontrado de mensajeria sin cambio", err)
			}
			if fake.idVisto != nada {
				t.Fatalf("404: se llamó al método por id con %v", fake.idVisto)
			}
		})
	}
}
