// Command core es la raíz de composición del deployable `core` (ADR 17, ADR 29,
// DEC-050 §2). Su único trabajo es cableado: leer configuración, abrir un pool
// por rol, construir cada módulo de `core` que exista con su New(Pools, Deps),
// resolver sus puertos con el adaptador en proceso (aristas co-ubicadas) o HTTP
// (otros deployables), montar sus Routes en un solo servidor y arrancar sus
// Workers, drenándolos al apagar.
//
// NO contiene lógica de negocio, no comparte pool entre roles, no cablea una
// arista no declarada en arch-test/reglas.yaml, y no importa internal/ de otro
// módulo (I1–I5).
//
// Estado incremental (DEC-049): hoy cablea los cinco módulos de core —identidad,
// catalogos, operacion, reportes y bff— con sus pools y adaptadores en proceso.
// El composition root está completo (arch-test exige exactamente esos cinco).
//
// La autenticación la provee construirAutenticador, definida en dos variantes
// por build tag: el verificador JWT real (producción) o el stub devauth
// (desarrollo, DEC-049). El binario de producción no contiene el stub.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/config"
	"github.com/AI27-Tech/pf-pkg-platform-go/observability"
	"github.com/AI27-Tech/pf-pkg-platform-go/pgxpool"
	"github.com/AI27-Tech/pf-pkg-platform-go/server"
	"github.com/AI27-Tech/pf-pkg-platform-go/worker"

	bff "github.com/AI27-Tech/pf-svc-bff"
	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
	"github.com/AI27-Tech/pf-svc-core/internal/adapters"
	identidad "github.com/AI27-Tech/pf-svc-identidad"
	operacion "github.com/AI27-Tech/pf-svc-operacion"
	reportes "github.com/AI27-Tech/pf-svc-reportes"
)

// Claves de entorno de las cadenas de conexión por rol. Cada rol tiene su
// propia cadena con sus credenciales (I1): core nunca comparte un pool entre
// roles, aunque los módulos compartan proceso. Los nombres de rol coinciden con
// los que cada módulo usa en su cmd/api standalone.
const (
	envDBCatalogos    = "DB_SVC_CATALOGOS_URL"
	envDBOpViajes     = "DB_OP_VIAJES_URL"
	envDBOpAsignacion = "DB_OP_ASIGNACION_URL"
	envDBOpBitacora   = "DB_OP_BITACORA_URL"
	envDBOpAuditoria  = "DB_OP_AUDITORIA_URL"
	envDBOpRoster     = "DB_OP_ROSTER_URL"
	envDBReportes     = "DB_SVC_REPORTES_URL"
	envDBIdentidad    = "DB_SVC_IDENTIDAD_URL"
)

// rolesDeBD asocia cada clave de entorno a su rol de BD. Es la fuente única de
// las variables requeridas y del rol con que se abre cada pool.
var rolesDeBD = []struct {
	env, rol string
}{
	{envDBCatalogos, "svc_catalogos"},
	{envDBOpViajes, "op_viajes"},
	{envDBOpAsignacion, "op_asignacion"},
	{envDBOpBitacora, "op_bitacora"},
	{envDBOpAuditoria, "op_auditoria"},
	{envDBOpRoster, "op_roster"},
	{envDBReportes, "svc_reportes"},
	{envDBIdentidad, "svc_identidad"},
}

// module es la superficie pública que cada pf-svc-* expone a la raíz (00-indice
// "Forma de un módulo"). core solo depende de estos tres métodos; nunca de
// internal/ del módulo. Consultas() lo usan los adaptadores en proceso, no la
// raíz directamente, así que no entra en esta interfaz mínima.
type module interface {
	Routes(mux *http.ServeMux)
	Workers() []worker.Worker
}

func main() {
	if err := run(); err != nil {
		slog.Error("core terminó con error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// 1. Configuración (falla con todas las variables faltantes, no solo la
	//    primera). Se exigen las cadenas de conexión por rol de los módulos
	//    cableados (catalogos y operacion).
	requeridas := make([]string, 0, len(rolesDeBD))
	for _, r := range rolesDeBD {
		requeridas = append(requeridas, r.env)
	}
	cfg, err := config.Load(requeridas...)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("arrancando core", slog.String("service", cfg.ServiceName))

	// 2. Observabilidad: trazas + métricas con shutdown combinado.
	shutdownOtel, err := observability.Init(ctx, observability.Config{
		ServiceName: cfg.ServiceName,
		ExporterURL: cfg.OtelExporterURL,
	})
	if err != nil {
		logger.Warn("observabilidad no inicializada", slog.Any("error", err))
	} else {
		defer func() { _ = shutdownOtel(context.Background()) }()
	}

	// 3. Autenticador: real o stub, según el build tag (auth_prod.go /
	//    auth_devauth.go). Se construye aunque todavía no haya rutas protegidas,
	//    para fallar temprano si falta configuración.
	autenticador, err := construirAutenticador(cfg)
	if err != nil {
		return err
	}

	// 4. Servidor HTTP con /health y /ready ya montados.
	srv := server.New(cfg.HTTPAddr, server.WithLogger(logger))

	// 5. Un pool por rol (I1): nunca compartido, aunque los módulos compartan
	//    proceso. Se cierran al apagar y se registran en readiness.
	pls, cerrarPools, err := abrirPools(ctx, srv)
	if err != nil {
		return err
	}
	defer cerrarPools()

	// 6. Cablear los cinco módulos de core con sus pools y adaptadores en
	//    proceso. identidad se devuelve por separado porque su Routes pide el
	//    verificador y la revocación (firma distinta de la interfaz module); el
	//    resto monta por la interfaz común. El autenticador (real o stub devauth)
	//    se inyecta en bff.Deps.Verificador y en las rutas protegidas de identidad.
	modules, idn, err := construirModulos(pls, autenticador, logger)
	if err != nil {
		return err
	}
	for _, m := range modules {
		m.Routes(srv.Mux())
	}
	// identidad monta sus rutas con el verificador y la proyección de revocación.
	// Sin worker de sesion.revocada cableado (bus en espera de ADR 32), la
	// revocación va nil: el middleware verifica firma pero no consulta tombstones.
	idn.Routes(srv.Mux(), autenticador, nil)

	// 7. Arrancar los workers que cada módulo declare, y drenarlos al apagar.
	//    identidad aporta su relay de outbox; entra en la recolección aunque su
	//    Routes vaya aparte.
	var workers []worker.Worker
	for _, m := range modules {
		workers = append(workers, m.Workers()...)
	}
	workers = append(workers, idn.Workers()...)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	workersListos := make(chan error, 1)
	go func() { workersListos <- worker.RunGroup(workerCtx, srv.Registry(), workers...) }()

	srv.MarkReady()
	errServidor := srv.Start(ctx)

	// Al salir el servidor, cancelar y esperar el drenado de los workers.
	cancelWorkers()
	if errWorkers := <-workersListos; errWorkers != nil {
		logger.Error("un worker terminó con error", slog.Any("error", errWorkers))
	}
	return errServidor
}

// pools agrupa los pools por rol abiertos por la raíz. Un pool por rol, nunca
// compartido (I1). bff no tiene pool.
type pools struct {
	catalogos    *pgxpool.Pool
	opViajes     *pgxpool.Pool
	opAsignacion *pgxpool.Pool
	opBitacora   *pgxpool.Pool
	opAuditoria  *pgxpool.Pool
	opRoster     *pgxpool.Pool
	reportes     *pgxpool.Pool
	identidad    *pgxpool.Pool
}

// abrirPools abre un pool por cada rol declarado en rolesDeBD, lo registra en la
// readiness del servidor y devuelve un cierre que los libera en orden inverso.
// Si alguno falla, cierra los ya abiertos antes de devolver el error.
func abrirPools(ctx context.Context, srv *server.Server) (pools, func(), error) {
	abiertos := make(map[string]*pgxpool.Pool, len(rolesDeBD))
	cerrar := func() {
		for i := len(rolesDeBD) - 1; i >= 0; i-- {
			if p := abiertos[rolesDeBD[i].rol]; p != nil {
				p.Close()
			}
		}
	}
	for _, r := range rolesDeBD {
		p, err := pgxpool.New(ctx, r.rol, os.Getenv(r.env))
		if err != nil {
			cerrar()
			return pools{}, func() {}, fmt.Errorf("abrir pool %s: %w", r.rol, err)
		}
		abiertos[r.rol] = p
		srv.Registry().RegisterCheck("db_"+r.rol, p.CheckFunc())
	}
	return pools{
		catalogos:    abiertos["svc_catalogos"],
		opViajes:     abiertos["op_viajes"],
		opAsignacion: abiertos["op_asignacion"],
		opBitacora:   abiertos["op_bitacora"],
		opAuditoria:  abiertos["op_auditoria"],
		opRoster:     abiertos["op_roster"],
		reportes:     abiertos["svc_reportes"],
		identidad:    abiertos["svc_identidad"],
	}, cerrar, nil
}

// construirModulos construye y devuelve los módulos de `core` que existan hoy.
// Es el único punto que cambia en cada incremento (DEC-049): al publicar un
// módulo, se agrega su New(...) aquí con sus pools y adaptadores en proceso.
//
// Hoy cablea catalogos (hoja), operacion y reportes, resolviendo las aristas
// co-ubicadas operacion → catalogos y reportes → catalogos con los adaptadores
// EN PROCESO de internal/adapters/ (I3). Los adaptadores HTTP de esos puertos
// viven en los repos consumidores y NO se cablean aquí porque catalogos está
// co-ubicado. No se cablea un consumidor cuyo destino síncrono no exista (00
// regla 6); no hay fakes de dominio en main. Faltan identidad y bff (DEC-049).
//
// TODO(core): aplicar las migraciones que cada módulo exporte (migrations.FS)
// con el rol dueño de cada esquema antes de servir tráfico. Pendiente del
// aplicador de migraciones acordado; hoy el scaffold no monta rutas de negocio.
func construirModulos(p pools, autenticador auth.Authenticator, logger *slog.Logger) (modules []module, idn *identidad.Module, err error) {
	// identidad: hoja del grafo, un pool/rol (svc_identidad). Es destino de las
	// aristas de revocación (operacion/catalogos/reportes) y de la de sesión
	// (bff), así que se construye primero. BusPublisher e IDP son adaptadores en
	// espera de ADR 32; en el scaffold el módulo se construye con el bus/IdP que
	// la raíz inyecte (hoy nil: el relay de outbox no drena sin bus, y la lógica
	// de sesión es scaffold).
	idn, err = identidad.New(identidad.Pools{Identidad: p.identidad}, identidad.Deps{})
	if err != nil {
		return nil, nil, fmt.Errorf("construir identidad: %w", err)
	}

	// catalogos: hoja del grafo, un pool/rol. Su única arista saliente es la
	// restringida a identidad para revocación (DEC-051), resuelta en proceso.
	cat, err := catalogos.New(catalogos.Pools{Catalogos: p.catalogos}, catalogos.Deps{
		Revocacion: adapters.NewRevocacionCatalogos(idn.Consultas()),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("construir catalogos: %w", err)
	}

	// operacion: cinco pools, uno por rol/esquema (ADR 33). La arista a
	// catalogos se resuelve con el adaptador en proceso sobre cat.Consultas();
	// la restringida a identidad para revocación, con el adaptador de revocación.
	op, err := operacion.New(operacion.Pools{
		Viajes:     p.opViajes,
		Asignacion: p.opAsignacion,
		Bitacora:   p.opBitacora,
		Auditoria:  p.opAuditoria,
		Roster:     p.opRoster,
	}, operacion.Deps{
		Aptitud:    adapters.NewAptitudRecursos(cat.Consultas()),
		Revocacion: adapters.NewRevocacionOperacion(idn.Consultas()),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("construir operacion: %w", err)
	}

	// reportes: un pool/rol (svc_reportes, ADR 18). Arista a catalogos
	// (dimensiones) y la restringida a identidad para revocación, ambas en proceso.
	rep, err := reportes.New(reportes.Pools{Reportes: p.reportes}, reportes.Deps{
		Catalogos:  adapters.NewReportesCatalogos(cat.Consultas()),
		Revocacion: adapters.NewRevocacionReportes(idn.Consultas()),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("construir reportes: %w", err)
	}

	// bff: sin pool (no es dueño de datos). Aristas en proceso a operacion e
	// identidad (fase a); catalogos y reportes son fase (b) y no están en su
	// Deps todavía. El verificador JWT es el autenticador real o el stub devauth,
	// según el build tag. La proyección de tombstones (Revocacion) la alimentaría
	// el worker de sesion.revocada; sin bus (ADR 32) va nil: el middleware no
	// consulta tombstones y la revocación aún no es efectiva end-to-end (scaffold).
	front, err := bff.New(bff.Pools{}, bff.Deps{
		Operacion:   adapters.NewBFFOperacion(op.Consultas()),
		Identidad:   adapters.NewBFFIdentidad(idn.Consultas()),
		Verificador: autenticador,
		Revocacion:  nil,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("construir bff: %w", err)
	}

	logger.Info("módulos cableados: identidad + catalogos + operacion + reportes + bff (aristas co-ubicadas en proceso)")
	// identidad va por separado: su Routes pide verificador y revocación (firma
	// distinta de la interfaz module). Entra igual en la recolección de Workers.
	return []module{cat, op, rep, front}, idn, nil
}
