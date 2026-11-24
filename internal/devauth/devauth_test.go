//go:build devauth

package devauth

import (
	"context"
	"testing"

	"github.com/google/uuid"

	roles "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/roles"
	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
)

func TestNuevo_RechazaProduccion(t *testing.T) {
	if _, err := Nuevo(EntornoProduccion); err == nil {
		t.Fatal("el stub NO debe construirse en entorno de producción")
	}
}

func TestNuevo_NueveRolesDosTenants(t *testing.T) {
	s, err := Nuevo(EntornoDesarrollo)
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	// 9 roles × 2 tenants = 18 identidades fake.
	if got := len(s.Tokens()); got != 18 {
		t.Errorf("tokens de desarrollo = %d, se esperaban 18 (9 roles × 2 tenants)", got)
	}
}

func TestVerifyToken_CadaRolYTenant(t *testing.T) {
	s, err := Nuevo(EntornoDesarrollo)
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	ctx := context.Background()

	// alias (va en el token) → UUID que llega en los claims.
	tenants := map[string]string{
		"tenant-dev-alfa": "00000000-0000-4000-8000-0000000000a1",
		"tenant-dev-beta": "00000000-0000-4000-8000-0000000000b1",
	}
	sujetos := map[string]string{}
	for _, rol := range roles.Todos {
		for alias, tenantID := range tenants {
			token := string(rol) + "@" + alias
			claims, err := s.VerifyToken(ctx, token)
			if err != nil {
				t.Fatalf("VerifyToken(%q): %v", token, err)
			}
			if claims.Rol != string(rol) {
				t.Errorf("%q: Rol = %q, se esperaba %q", token, claims.Rol, rol)
			}
			if claims.TenantID != tenantID {
				t.Errorf("%q: TenantID = %q, se esperaba %q", token, claims.TenantID, tenantID)
			}
			// DEC-112 y DEC-049: tenant y sujeto son UUID (operacion y mensajeria
			// rechazan lo contrario), estables entre llamadas y distintos por usuario.
			if _, err := uuid.Parse(claims.Subject); err != nil {
				t.Errorf("%q: Subject %q no es UUID", token, claims.Subject)
			}
			if otro, dup := sujetos[claims.Subject]; dup {
				t.Errorf("%q: Subject %q repetido con %q", token, claims.Subject, otro)
			}
			sujetos[claims.Subject] = token
			de2, _ := s.VerifyToken(ctx, token)
			if de2.Subject != claims.Subject {
				t.Errorf("%q: el Subject no es estable (%q y %q)", token, claims.Subject, de2.Subject)
			}
			// Audiencia: cliente→Cliente; los otros ocho→Interno (DEC-054).
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
