//go:build !devauth

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/config"
)

func TestConstruirAutenticadorExigeJWKSAudienciaYEmisor(t *testing.T) {
	t.Setenv(envJWTAudience, "")
	t.Setenv(envJWTIssuer, "")
	_, _, err := construirAutenticador(&config.Config{})
	if err == nil {
		t.Fatal("se esperaba error sin JWKS_URL, JWT_AUDIENCE ni JWT_ISSUER")
	}
	for _, v := range []string{config.EnvJWKSURL, envJWTAudience, envJWTIssuer} {
		if !strings.Contains(err.Error(), v) {
			t.Errorf("el error %q no nombra %s", err, v)
		}
	}
}

func TestConstruirAutenticadorCompleto(t *testing.T) {
	t.Setenv(envJWTAudience, "ai27-internal")
	t.Setenv(envJWTIssuer, "http://kc/realms/ai27")
	a, tokens, err := construirAutenticador(&config.Config{JWKSURL: "http://kc/realms/ai27/protocol/openid-connect/certs"})
	if err != nil || a == nil || tokens == nil {
		t.Fatalf("construirAutenticador = %v, %v, %v", a, tokens, err)
	}
}

// claimsFijos es un verificador interno que acepta todo con los claims dados.
type claimsFijos map[string]any

func (c claimsFijos) VerifyToken(context.Context, string) (*auth.Claims, error) {
	return &auth.Claims{Subject: "u1", Raw: c}, nil
}

func TestVerificadorConEmisor(t *testing.T) {
	const emisor = "http://kc/realms/ai27"
	casos := []struct {
		nombre string
		raw    claimsFijos
		acepta bool
	}{
		{"iss esperado", claimsFijos{"iss": emisor}, true},
		{"iss de otro realm", claimsFijos{"iss": "http://kc/realms/master"}, false},
		{"sin iss", claimsFijos{}, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			v := verificadorConEmisor{Authenticator: c.raw, emisor: emisor}
			claims, err := v.VerifyToken(context.Background(), "tok")
			if c.acepta && (err != nil || claims.Subject != "u1") {
				t.Fatalf("se esperaba aceptar: %v", err)
			}
			if !c.acepta && err == nil {
				t.Fatal("se esperaba rechazo por iss")
			}
		})
	}
}
