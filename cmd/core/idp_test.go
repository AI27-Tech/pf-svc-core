package main

import (
	"os"
	"strings"
	"testing"

	identidad "github.com/AI27-Tech/pf-svc-identidad"
)

// entornoKeycloak es la configuración mínima completa del adaptador de
// Keycloak. Las URLs no se contactan: construirIDP no hace red.
func entornoKeycloak() map[string]string {
	return map[string]string{
		"IDENTIDAD_IDP":                            "keycloak",
		"IDENTIDAD_KEYCLOAK_TOKEN_URL":             "http://127.0.0.1:1/realms/ai27/protocol/openid-connect/token",
		"IDENTIDAD_KEYCLOAK_ADMIN_SESSIONS_URL":    "http://127.0.0.1:1/admin/realms/ai27/sessions",
		"IDENTIDAD_KEYCLOAK_ADMIN_USERS_URL":       "http://127.0.0.1:1/admin/realms/ai27/users",
		"IDENTIDAD_KEYCLOAK_ADMIN_GROUPS_URL":      "http://127.0.0.1:1/admin/realms/ai27/groups",
		"IDENTIDAD_KEYCLOAK_CLIENT_ID":             "ai27-login",
		"IDENTIDAD_KEYCLOAK_CLIENT_SECRET":         "s1",
		"IDENTIDAD_KEYCLOAK_SERVICE_CLIENT_ID":     "ai27-identidad-svc",
		"IDENTIDAD_KEYCLOAK_SERVICE_CLIENT_SECRET": "s2",
		"IDENTIDAD_KEYCLOAK_ADMIN_CLIENT_ID":       "ai27-identidad-svc",
		"IDENTIDAD_KEYCLOAK_ADMIN_CLIENT_SECRET":   "s3",
		"IDENTIDAD_KEYCLOAK_TIMEOUT":               "5s",
	}
}

func lookupDe(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

// idpDePrueba construye el adaptador real de Keycloak con configuración ficticia,
// para los tests de cableado que no ejercitan el IdP.
func idpDePrueba(t *testing.T) identidad.IdentityProvider {
	t.Helper()
	p, err := construirIDP(lookupDe(entornoKeycloak()), os.ReadFile)
	if err != nil {
		t.Fatalf("construirIDP: %v", err)
	}
	return p
}

func TestConstruirIDPConKeycloak(t *testing.T) {
	if idpDePrueba(t) == nil {
		t.Fatal("construirIDP devolvió un proveedor nil")
	}
}

func TestConstruirIDPFallaSinKeycloak(t *testing.T) {
	sinSeleccion := entornoKeycloak()
	delete(sinSeleccion, "IDENTIDAD_IDP")
	incompleto := entornoKeycloak()
	delete(incompleto, "IDENTIDAD_KEYCLOAK_TOKEN_URL")
	delete(incompleto, "IDENTIDAD_KEYCLOAK_CLIENT_SECRET")

	casos := []struct {
		nombre string
		env    map[string]string
		debe   []string // fragmentos que el error debe nombrar
	}{
		{"sin IDENTIDAD_IDP (memoria)", sinSeleccion, []string{"IDENTIDAD_IDP=keycloak"}},
		{"keycloak incompleto", incompleto, []string{"IDENTIDAD_KEYCLOAK_TOKEN_URL", "IDENTIDAD_KEYCLOAK_CLIENT_SECRET"}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p, err := construirIDP(lookupDe(c.env), os.ReadFile)
			if err == nil {
				t.Fatalf("se esperaba error, se obtuvo proveedor %T", p)
			}
			for _, f := range c.debe {
				if !strings.Contains(err.Error(), f) {
					t.Errorf("el error %q no nombra %s", err, f)
				}
			}
			if strings.Contains(err.Error(), "s1") || strings.Contains(err.Error(), "s2") {
				t.Errorf("el error filtra un secreto: %q", err)
			}
		})
	}
}
