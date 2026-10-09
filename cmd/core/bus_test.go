//go:build nats

// Verifica el cableado del bus en la raíz: abrirBus conecta a un NATS real y
// falla duro si el broker no responde. Build tag "nats" para no correr en CI
// sin broker.
//
//	docker run --rm -p 4222:4222 -p 8222:8222 nats:2-alpine -js -m 8222
//	go test -tags nats ./cmd/core/
package main

import (
	"os"
	"testing"
)

func urlNATS() string {
	if u := os.Getenv("NATS_URL"); u != "" {
		return u
	}
	return "nats://127.0.0.1:4222"
}

// TestAbrirBusConectaYCierra: con NATS vivo, abrirBus devuelve los puertos del
// bus usables (Publisher durable y Broadcaster) y un cierre que no entra en pánico.
func TestAbrirBusConectaYCierra(t *testing.T) {
	bc, cerrar, err := abrirBus(urlNATS())
	if err != nil {
		t.Fatalf("abrirBus con NATS vivo devolvió error: %v", err)
	}
	if bc.durable == nil {
		t.Error("abrirBus devolvió un Publisher durable nil")
	}
	if bc.conexion == nil {
		t.Error("abrirBus devolvió una Conexion (Broadcaster) nil")
	}
	cerrar()
}

// TestAbrirBusFallaDuroSinBroker: con una URL que no responde, abrirBus devuelve
// error (core falla al arrancar, no degrada).
func TestAbrirBusFallaDuroSinBroker(t *testing.T) {
	_, _, err := abrirBus("nats://127.0.0.1:4299") // puerto sin broker
	if err == nil {
		t.Fatal("abrirBus sin broker debió fallar, devolvió nil")
	}
	t.Logf("falló duro como se esperaba: %v", err)
}
