package adapters

import (
	"context"
	"errors"
	"testing"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
)

// verificadorFijo devuelve siempre los mismos claims o el mismo error.
type verificadorFijo struct {
	claims *auth.Claims
	err    error
}

func (v verificadorFijo) VerifyToken(context.Context, string) (*auth.Claims, error) {
	return v.claims, v.err
}

func TestTokensIdentidadLeeSubYSesion(t *testing.T) {
	p := NewTokensIdentidad(verificadorFijo{claims: &auth.Claims{Subject: "kc-sub-1", SesionID: "0b6c5c3e-0000-4000-8000-000000000001"}})

	sub, err := p.SubDelAccessToken("tok")
	if err != nil || sub != "kc-sub-1" {
		t.Fatalf("SubDelAccessToken = %q, %v; se esperaba kc-sub-1", sub, err)
	}
	sid, err := p.SesionIDDelAccessToken("tok")
	if err != nil || sid != "0b6c5c3e-0000-4000-8000-000000000001" {
		t.Fatalf("SesionIDDelAccessToken = %q, %v", sid, err)
	}
}

func TestTokensIdentidadFallaCerrado(t *testing.T) {
	rechazo := errors.New("auth: verificar token: firma inválida")
	casos := map[string]verificadorFijo{
		"token rechazado": {err: rechazo},
		"claims vacíos":   {claims: &auth.Claims{}},
	}
	for nombre, v := range casos {
		t.Run(nombre, func(t *testing.T) {
			p := NewTokensIdentidad(v)
			if sub, err := p.SubDelAccessToken("tok"); err == nil {
				t.Errorf("SubDelAccessToken = %q sin error", sub)
			}
			if sid, err := p.SesionIDDelAccessToken("tok"); err == nil {
				t.Errorf("SesionIDDelAccessToken = %q sin error", sid)
			}
			if v.err != nil {
				if _, err := p.SubDelAccessToken("tok"); !errors.Is(err, rechazo) {
					t.Errorf("el error del verificador no se propagó: %v", err)
				}
			}
		})
	}
}
