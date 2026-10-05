//go:build devauth

package devauth

import (
	"context"
	"testing"

	roles "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/roles"
	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
)

func TestNuevo_RechazaProduccion(t *testing.T) {
	if _, err := Nuevo(EntornoProduccion); err == nil {
		t.Fatal("el stub NO debe construirse en entorno de producción")
	}
}

func TestNuevo_OchoRolesDosTenants(t *testing.T) {
	s, err := Nuevo(EntornoDesarrollo)
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	// 8 roles × 2 tenants = 16 identidades fake.
	if got := len(s.Tokens()); got != 16 {
		t.Errorf("tokens de desarrollo = %d, se esperaban 16 (8 roles × 2 tenants)", got)
	}
}

func TestVerifyToken_CadaRolYTenant(t *testing.T) {
	s, err := Nuevo(EntornoDesarrollo)
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	ctx := context.Background()

	tenants := []string{"tenant-dev-alfa", "tenant-dev-beta"}
	for _, rol := range roles.Todos {
		for _, tenant := range tenants {
			token := string(rol) + "@" + tenant
			claims, err := s.VerifyToken(ctx, token)
			if err != nil {
				t.Fatalf("VerifyToken(%q): %v", token, err)
			}
			if claims.Rol != string(rol) {
				t.Errorf("%q: Rol = %q, se esperaba %q", token, claims.Rol, rol)
			}
			if claims.TenantID != tenant {
				t.Errorf("%q: TenantID = %q, se esperaba %q", token, claims.TenantID, tenant)
			}
			// Audiencia: cliente→Cliente; los otros siete→Interno (DEC-054).
			esperada := auth.AudienciaInterno
			if rol == roles.RolCliente {
				esperada = auth.AudienciaCliente
			}
			if claims.Audiencia != esperada {
				t.Errorf("%q: Audiencia = %q, se esperaba %q", token, claims.Audiencia, esperada)
			}
		}
	}
}

func TestVerifyToken_FallaCerrado(t *testing.T) {
	s, err := Nuevo(EntornoDesarrollo)
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	ctx := context.Background()

	casos := []struct {
		nombre string
		token  string
	}{
		{"vacío", ""},
		{"solo espacios", "   "},
		{"desconocido", "fantasma@tenant-dev-alfa"},
		{"rol inválido", "superusuario@tenant-dev-alfa"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := s.VerifyToken(ctx, c.token); err == nil {
				t.Errorf("token %q debería rechazarse (fallo cerrado)", c.token)
			}
		})
	}
}

// TestImplementaAuthenticator confirma que el stub satisface la interfaz que
// consumen los módulos.
func TestImplementaAuthenticator(t *testing.T) {
	var _ auth.Authenticator = (*Stub)(nil)
}
