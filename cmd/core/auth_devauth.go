//go:build devauth

package main

import (
	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/config"
	"github.com/AI27-Tech/pf-svc-core/internal/devauth"
	identidad "github.com/AI27-Tech/pf-svc-identidad"
)

// construirAutenticador devuelve el stub de desarrollo (DEC-049) en lugar del
// verificador JWT real. Solo se compila con el tag `devauth`; el binario de
// producción nunca entra por aquí.
//
// El stub se niega a arrancar si el entorno declarado es producción (defensa en
// profundidad sobre el build tag). El entorno se lee de la configuración.
//
// No hay TokenParser: el stub no lee tokens del IdP, así que con devauth el
// login y el refresco de identidad responden 501 y se trabaja con los tokens de
// desarrollo del stub.
func construirAutenticador(cfg *config.Config) (auth.Authenticator, identidad.TokenParser, error) {
	entorno := devauth.EntornoDesarrollo
	if config.GetEnvBool("CORE_PRODUCCION", false) {
		entorno = devauth.EntornoProduccion
	}
	stub, err := devauth.Nuevo(entorno)
	if err != nil {
		return nil, nil, err
	}
	return stub, nil, nil
}
