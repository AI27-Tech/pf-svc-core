//go:build !devauth

package main

import (
	"fmt"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/config"
)

// construirAutenticador devuelve el verificador JWT real de la plataforma
// (fallo cerrado, refresco proactivo del JWKS). Es el camino de producción: el
// binario sin el tag `devauth` NO contiene el stub de desarrollo.
//
// La URL del JWKS y la audiencia esperada se inyectan por configuración; el
// verificador no conoce al proveedor (IdP on-premise; adaptador real EN ESPERA
// de ADR 32, pero el verificador JWKS es neutral respecto del producto).
func construirAutenticador(cfg *config.Config) (auth.Authenticator, error) {
	if cfg.JWKSURL == "" {
		return nil, fmt.Errorf("core: %s es obligatorio en producción (sin -tags devauth)", config.EnvJWKSURL)
	}
	// La audiencia técnica esperada (`aud`) se toma de la configuración del
	// despliegue. Se usa el nombre de servicio como respaldo mínimo para
	// desarrollo sin devauth; un despliegue real la fija explícitamente.
	expectedAud := cfg.ServiceName
	return auth.NewVerifier(cfg.JWKSURL, expectedAud, 0), nil
}
