//go:build devauth

// Package devauth es el stub de autenticación de DESARROLLO (DEC-049).
//
// Existe solo mientras `identidad` no está integrada: produce los mismos
// `auth.Claims` que el verificador real de pf-pkg-platform-go e implementa la
// misma interfaz (`auth.Authenticator`), para que los módulos protegidos no
// distingan el stub del verificador JWT.
//
// Vive en pf-svc-core/internal/ y SOLO se compila con el build tag `devauth`
// (`//go:build devauth`): el binario de producción no lo contiene. Como defensa
// adicional, Nuevo se niega a construirse si el entorno declarado es producción.
//
// Lo que el stub NO hace (DEC-049, DEC-053):
//   - no emite alcance (unidad/cliente): el alcance se resuelve en servidor;
//   - no fabrica el vínculo Monitorista↔unidad (pendiente en DEC-045);
//   - no es evidencia de G2: ni revocación ≤30 s ni aislamiento real de tenant.
//
// Los valores de `Rol` salen del enum publicado de pf-pkg-contracts; no se
// escriben a mano (DEC-050 §7).
package devauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	roles "github.com/AI27-Tech/pf-pkg-contracts/packages/api-client-go/roles"
	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
)

// Entorno declara el ambiente en que corre el proceso. El stub se niega a
// arrancar en producción (defensa en profundidad sobre el build tag).
type Entorno string

const (
	EntornoDesarrollo Entorno = "desarrollo"
	EntornoProduccion Entorno = "produccion"
)

// Los tenants de desarrollo. Su alias (lo que va en el token, "<rol>@<alias>") es
// legible; el TenantID de los claims es un UUID estable, porque el del cliente es
// su cliente_id (DEC-112) y operacion rechaza con 403 un tenant que no lo sea.
const (
	AliasTenantAlfa = "tenant-dev-alfa"
	AliasTenantBeta = "tenant-dev-beta"
	TenantAlfaID    = "00000000-0000-4000-8000-0000000000a1"
	TenantBetaID    = "00000000-0000-4000-8000-0000000000b1"
)

// usuarioFake modela una identidad de desarrollo: los mismos campos que
// `auth.Claims` sin alcance (DEC-053).
type usuarioFake struct {
	Subject  string
	Rol      roles.Rol
	TenantID string
	SesionID string
}

// Stub implementa auth.Authenticator resolviendo un token de desarrollo contra
// un directorio de usuarios fake. El "token" de desarrollo es "<rol>@<alias del tenant>";
// una petición con un token desconocido se rechaza (fallo cerrado).
type Stub struct {
	usuarios map[string]usuarioFake // clave: el token de dev "<rol>@<alias del tenant>"
}

var _ auth.Authenticator = (*Stub)(nil)

// Nuevo construye el stub con usuarios fake de cada uno de los nueve roles (los
// ocho de DEC-045, identificadores de DEC-053, y `cotizacion` de DEC-148) en al
// menos dos tenants distintos, para que las pruebas de aislamiento de tenant tengan algo que distinguir. Devuelve
// error si el entorno es producción o si el enum del contrato no trae los nueve
// roles (brecha contractual; no se completa a mano).
func Nuevo(entorno Entorno) (*Stub, error) {
	if entorno == EntornoProduccion {
		return nil, fmt.Errorf("devauth: el stub de desarrollo no debe arrancar en producción")
	}

	// Los nueve roles deben venir del contrato. Si faltan, es brecha contractual.
	if len(roles.Todos) != 9 {
		return nil, fmt.Errorf("devauth: el enum Rol del contrato tiene %d valores; se esperaban los 9 de DEC-053 y DEC-148 (brecha contractual, no se completa a mano)", len(roles.Todos))
	}

	tenants := []struct{ alias, id string }{{AliasTenantAlfa, TenantAlfaID}, {AliasTenantBeta, TenantBetaID}}
	usuarios := make(map[string]usuarioFake, len(roles.Todos)*len(tenants))

	for ti, tenant := range tenants {
		for _, rol := range roles.Todos {
			// El token de dev es legible: "<rol>@<alias del tenant>". El Subject de los
			// claims es un UUID derivado y estable (DEC-049: mensajeria exige sujeto UUID).
			token := fmt.Sprintf("%s@%s", string(rol), tenant.alias)
			usuarios[token] = usuarioFake{
				Subject:  uuid.NewSHA1(uuid.MustParse(tenant.id), []byte(token)).String(),
				Rol:      rol,
				TenantID: tenant.id,
				SesionID: fmt.Sprintf("sesion-dev-%s-%d", string(rol), ti),
			}
		}
	}

	return &Stub{usuarios: usuarios}, nil
}

// VerifyToken resuelve un token de desarrollo ("<rol>@<alias del tenant>") a
// los Claims equivalentes. Un token vacío o desconocido se rechaza igual que la
// ausencia de JWT (fallo cerrado, DEC-049). La Audiencia de dominio se deriva
// del rol con la misma regla del verificador real (cliente→Cliente, resto→Interno).
func (s *Stub) VerifyToken(ctx context.Context, tokenString string) (*auth.Claims, error) {
	token := strings.TrimSpace(tokenString)
	if token == "" {
		return nil, fmt.Errorf("devauth: identidad de desarrollo faltante")
	}
	u, ok := s.usuarios[token]
	if !ok {
		return nil, fmt.Errorf("devauth: identidad de desarrollo desconocida")
	}

	audiencia := auth.AudienciaInterno
	if u.Rol == roles.RolCliente {
		audiencia = auth.AudienciaCliente
	}

	return &auth.Claims{
		Subject:   u.Subject,
		Rol:       string(u.Rol),
		TenantID:  u.TenantID,
		Audiencia: audiencia,
		SesionID:  u.SesionID,
		Raw: map[string]any{
			"devauth": true,
		},
	}, nil
}

// Tokens devuelve, para diagnóstico y pruebas, los tokens de desarrollo
// disponibles (uno por rol y tenant).
func (s *Stub) Tokens() []string {
	out := make([]string, 0, len(s.usuarios))
	for k := range s.usuarios {
		out = append(out, k)
	}
	return out
}
