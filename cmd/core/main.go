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
	"log/slog"
	"net/http"
	"os"

	"github.com/AI27-Tech/pf-pkg-platform-go/auth"
	"github.com/AI27-Tech/pf-pkg-platform-go/config"
	"github.com/AI27-Tech/pf-pkg-platform-go/observability"
	"github.com/AI27-Tech/pf-pkg-platform-go/server"
	"github.com/AI27-Tech/pf-pkg-platform-go/worker"
)

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
	//    primera). Hoy no se exige ninguna variable de módulo porque no hay
	//    módulos cableados; cada incremento agregará sus requeridas.
	cfg, err := config.Load()
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

	// 5. Cablear los módulos que existan. Hoy: ninguno (DEC-049). Cuando
	//    catalogos y operacion publiquen versión, aquí se construyen con sus
	//    pools y adaptadores, se agregan a `modules`, y el middleware de auth se
	//    aplica a sus rutas protegidas.
	modules := construirModulos(ctx, cfg, autenticador, logger)
	for _, m := range modules {
		m.Routes(srv.Mux())
	}

	// 6. Arrancar los workers que cada módulo declare, y drenarlos al apagar.
	//    Con cero módulos, RunGroup no arranca nada.
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

// construirModulos construye y devuelve los módulos de `core` que existan hoy.
// Es el único punto que cambia en cada incremento (DEC-049): al publicar un
// módulo, se agrega su New(...) aquí con sus pools y adaptadores en proceso.
//
// Hoy devuelve una lista vacía: no hay ningún pf-svc-* publicado. No se cablea
// un consumidor cuyo destino síncrono no exista (00 regla 6); no hay fakes de
// dominio en main.
func construirModulos(_ context.Context, _ *config.Config, _ auth.Authenticator, logger *slog.Logger) []module {
	logger.Info("sin módulos cableados todavía: core sirve solo /health y /ready (DEC-049)")
	return nil
}
