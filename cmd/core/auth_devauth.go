//go:build devauth

package main

import (
	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/config"
	"github.com/AI27-Tech/pf-svc-core/internal/devauth"
)

// construirAutenticador devuelve el stub de desarrollo (DEC-049) en lugar del
// verificador JWT real. Solo se compila con el tag `devauth`; el binario de
// producción nunca entra por aquí.
//
// El stub se niega a arrancar si el entorno declarado es producción (defensa en
// profundidad sobre el build tag). El entorno se lee de la configuración.
func construirAutenticador(cfg *config.Config) (auth.Authenticator, error) {
	entorno := devauth.EntornoDesarrollo
	if config.GetEnvBool("CORE_PRODUCCION", false) {
		entorno = devauth.EntornoProduccion
	}
	return devauth.Nuevo(entorno)
}
