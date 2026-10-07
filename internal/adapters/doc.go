// Package adapters contiene los adaptadores EN PROCESO de las aristas síncronas
// co-ubicadas dentro de `core` (00-indice regla 2, DEC-050 §2). pf-svc-core es
// el único repo que ve a ambos extremos de una arista co-ubicada, así que es el
// único lugar donde vive el adaptador en proceso; el adaptador HTTP del mismo
// puerto vive en el repo consumidor (`adapters/http/`).
//
// Cada adaptador implementa el puerto que declara el módulo CONSUMIDOR (en su
// ports.go) y respeta, igual que el adaptador HTTP (I3):
//   - el mismo presupuesto de tiempo (context.WithTimeout con el valor que el
//     consumidor exporta junto a su puerto);
//   - la misma taxonomía de error (traducción a pf-pkg-platform-go/errors);
//   - tipos de pf-pkg-contracts, nunca tipos de internal/ del destino.
//
// Aristas co-ubicadas previstas (ADR 17, v1-topologia §2), a cablear cuando los
// módulos existan:
//   - operacion → catalogos   (puerto AptitudRecursos)
//   - reportes  → catalogos
//   - bff → operacion / catalogos / identidad / reportes
//   - operacion/catalogos/reportes → identidad  (restringida: solo
//     listarRevocadas + obtenerSesion; DEC-051, DEC-058)
//
// bff → mensajeria es SIEMPRE HTTP (otro deployable), así que no tiene adaptador
// en proceso aquí.
//
// Cableados hoy:
//   - AptitudRecursosEnProceso (operacion → catalogos, puerto AptitudRecursos),
//     con sus pruebas de paridad (I3) en aptitud_recursos_test.go.
//
// El resto de las aristas se agregan cuando sus módulos existan; no se escriben
// tipos de wire ni adaptadores contra una superficie inexistente (00 regla 6).
package adapters
