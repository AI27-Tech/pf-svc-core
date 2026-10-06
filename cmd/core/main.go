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
// Estado incremental (DEC-049): hoy no hay ningún pf-svc-* publicado (catalogos
// y operacion están vacíos), así que core arranca SIN módulos y solo sirve
// /health y /ready. A medida que catalogos y operacion (y luego identidad, bff,
// reportes) publiquen versión, se agregan a go.mod y se cablean aquí.
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

	"github.com/AI27-Tech/pf-pkg-platform-go/config"
	"github.com/AI27-Tech/pf-pkg-platform-go/observability"
	"github.com/AI27-Tech/pf-pkg-platform-go/pgxpool"
	"github.com/AI27-Tech/pf-pkg-platform-go/server"
	"github.com/AI27-Tech/pf-pkg-platform-go/worker"

	catalogos "github.com/AI27-Tech/pf-svc-catalogos"
	"github.com/AI27-Tech/pf-svc-core/internal/adapters"
	operacion "github.com/AI27-Tech/pf-svc-operacion"
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

	// 6. Cablear los módulos que existan con sus pools y adaptadores en proceso.
	//    Hoy: catalogos + operacion (DEC-049; identidad, bff, reportes llegan
	//    después). El autenticador se construye arriba para fallar temprano; su
	//    middleware se aplicará a las rutas protegidas cuando los handlers de
	//    negocio existan.
	_ = autenticador
	modules, err := construirModulos(pls, logger)
	if err != nil {
		return err
	}
	for _, m := range modules {
		m.Routes(srv.Mux())
	}

	// 7. Arrancar los workers que cada módulo declare, y drenarlos al apagar.
	var workers []worker.Worker
	for _, m := range modules {
		workers = append(workers, m.Workers()...)
	}

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
	}, cerrar, nil
}

// construirModulos construye y devuelve los módulos de `core` que existan hoy.
// Es el único punto que cambia en cada incremento (DEC-049): al publicar un
// módulo, se agrega su New(...) aquí con sus pools y adaptadores en proceso.
//
// Hoy cablea catalogos (hoja) y operacion, resolviendo la arista
// operacion → catalogos con el adaptador EN PROCESO de internal/adapters/ (I3).
// El adaptador HTTP del mismo puerto vive en pf-svc-operacion y NO se cablea
// aquí porque catalogos está co-ubicado. No se cablea un consumidor cuyo destino
// síncrono no exista (00 regla 6); no hay fakes de dominio en main.
//
// TODO(core): aplicar las migraciones que cada módulo exporte (migrations.FS)
// con el rol dueño de cada esquema antes de servir tráfico. Pendiente del
// aplicador de migraciones acordado; hoy el scaffold no monta rutas de negocio.
func construirModulos(p pools, logger *slog.Logger) ([]module, error) {
	// catalogos: hoja del grafo, un pool/rol. Sin arista de dominio saliente
	// (la restringida a identidad se cablea cuando identidad exista).
	cat, err := catalogos.New(catalogos.Pools{Catalogos: p.catalogos}, catalogos.Deps{})
	if err != nil {
		return nil, fmt.Errorf("construir catalogos: %w", err)
	}

	// operacion: cinco pools, uno por rol/esquema (ADR 33). La arista a
	// catalogos se resuelve con el adaptador en proceso sobre cat.Consultas().
	op, err := operacion.New(operacion.Pools{
		Viajes:     p.opViajes,
		Asignacion: p.opAsignacion,
		Bitacora:   p.opBitacora,
		Auditoria:  p.opAuditoria,
		Roster:     p.opRoster,
	}, operacion.Deps{
		Aptitud: adapters.NewAptitudRecursos(cat.Consultas()),
		// Revocacion: se cablea cuando identidad exista (DEC-051).
	})
	if err != nil {
		return nil, fmt.Errorf("construir operacion: %w", err)
	}

	logger.Info("módulos cableados: catalogos + operacion (operacion → catalogos en proceso)")
	return []module{cat, op}, nil
}
