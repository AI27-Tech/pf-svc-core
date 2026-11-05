package main

import (
	"fmt"

	identidad "github.com/AI27-Tech/pf-svc-identidad"
	"github.com/AI27-Tech/pf-svc-identidad/idpconfig"
)

// construirIDP arma el proveedor de identidad que identidad.Deps.IDP exige,
// desde IDENTIDAD_IDP / IDENTIDAD_KEYCLOAK_*, con la misma lógica que el cmd
// standalone de identidad (idpconfig.Cargar: junta todas las faltantes en un
// error, resuelve los secretos _FILE y nunca imprime valores). No hace red.
//
// Exige Keycloak en los dos builds: desde identidad v0.1.3, New falla sin IDP, y
// el doble en memoria vive en internal/ de identidad, inalcanzable desde core.
func construirIDP(lookup func(string) (string, bool), readFile func(string) ([]byte, error)) (identidad.IdentityProvider, error) {
	cfg, err := idpconfig.Cargar(lookup, readFile)
	if err != nil {
		return nil, fmt.Errorf("core: %w", err)
	}
	if cfg.Seleccion != idpconfig.Keycloak {
		return nil, fmt.Errorf("core: IDENTIDAD_IDP=%s es obligatorio (el doble en memoria de identidad no se puede construir desde core)", idpconfig.Keycloak)
	}
	return identidad.NewIDPKeycloak(cfg.Keycloak)
}
