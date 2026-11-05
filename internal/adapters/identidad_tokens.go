package adapters

import (
	"context"
	"errors"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	identidad "github.com/AI27-Tech/pf-svc-identidad"
)

// TokensIdentidad implementa identidad.TokenParser, la costura que activa el
// login de E4-T3: identidad lee el `sub` (para resolver el sujeto, DEC-072) y el
// `sesion_id` (para el refresco) del access token que el IdP acaba de emitir.
// Verifica el token con el mismo autenticador que protege las rutas del bff
// (firma contra el JWKS, expiración, `aud` e `iss`), como pide identidad.Deps.
type TokensIdentidad struct {
	verificador auth.Authenticator
}

var _ identidad.TokenParser = (*TokensIdentidad)(nil)

// NewTokensIdentidad construye el parser sobre el autenticador de core.
func NewTokensIdentidad(v auth.Authenticator) *TokensIdentidad {
	return &TokensIdentidad{verificador: v}
}

// SubDelAccessToken devuelve el claim `sub` (id opaco del IdP).
func (t *TokensIdentidad) SubDelAccessToken(accessToken string) (string, error) {
	c, err := t.verificar(accessToken)
	if err != nil {
		return "", err
	}
	if c.Subject == "" {
		return "", errors.New("adapters: access token sin claim sub")
	}
	return c.Subject, nil
}

// SesionIDDelAccessToken devuelve el claim `sesion_id` (propio de AI27).
func (t *TokensIdentidad) SesionIDDelAccessToken(accessToken string) (string, error) {
	c, err := t.verificar(accessToken)
	if err != nil {
		return "", err
	}
	if c.SesionID == "" {
		return "", errors.New("adapters: access token sin claim sesion_id")
	}
	return c.SesionID, nil
}

// verificar usa context.Background porque TokenParser no recibe contexto; el
// verificador solo lo usa para bajar el JWKS, con su propio timeout de cliente.
func (t *TokensIdentidad) verificar(accessToken string) (*auth.Claims, error) {
	return t.verificador.VerifyToken(context.Background(), accessToken)
}
