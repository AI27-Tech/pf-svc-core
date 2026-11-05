//go:build !devauth

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/config"
	"github.com/AI27-Tech/pf-svc-core/internal/adapters"
	identidad "github.com/AI27-Tech/pf-svc-identidad"
)

// Claves de entorno del verificador JWT de producción, junto a JWKS_URL de
// platform-go. Con Keycloak: aud = la audiencia que emite el mapper del realm
// (ai27-internal) e iss = la URL del realm (p. ej. https://auth.../realms/ai27).
const (
	envJWTAudience = "JWT_AUDIENCE"
	envJWTIssuer   = "JWT_ISSUER"
)

// construirAutenticador devuelve el verificador JWT real de la plataforma
// (fallo cerrado: firma contra el JWKS, expiración, `aud` e `iss`) y, sobre él,
// el TokenParser que activa el login de identidad. Es el camino de producción:
// el binario sin el tag `devauth` NO contiene el stub de desarrollo. Falla al
// arrancar, nombrando todas las faltantes, si falta JWKS_URL, JWT_AUDIENCE o
// JWT_ISSUER.
func construirAutenticador(cfg *config.Config) (auth.Authenticator, identidad.TokenParser, error) {
	aud, iss := os.Getenv(envJWTAudience), os.Getenv(envJWTIssuer)
	var faltan []string
	for _, v := range []struct{ nombre, valor string }{
		{config.EnvJWKSURL, cfg.JWKSURL}, {envJWTAudience, aud}, {envJWTIssuer, iss},
	} {
		if v.valor == "" {
			faltan = append(faltan, v.nombre)
		}
	}
	if len(faltan) > 0 {
		return nil, nil, fmt.Errorf("core: faltan %s (obligatorias en producción, sin -tags devauth)", strings.Join(faltan, ", "))
	}
	v := verificadorConEmisor{Authenticator: auth.NewVerifier(cfg.JWKSURL, aud, 0), emisor: iss}
	return v, adapters.NewTokensIdentidad(v), nil
}

// verificadorConEmisor rechaza el token cuyo `iss` no es el emisor esperado.
// shortcut: auth.Verifier de platform-go v0.1.9 no valida `iss`; mover esta
// guarda a platform-go cuando su verificador la ofrezca.
type verificadorConEmisor struct {
	auth.Authenticator
	emisor string
}

func (v verificadorConEmisor) VerifyToken(ctx context.Context, token string) (*auth.Claims, error) {
	c, err := v.Authenticator.VerifyToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if iss, _ := c.Raw["iss"].(string); iss != v.emisor {
		return nil, errors.New("auth: emisor del token inesperado")
	}
	return c, nil
}
