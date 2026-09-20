// Command tg-server runs a tiny-gather world and serves it over WebSockets.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/vadremix/tiny-gather/internal/sim"
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/internal/wsserver"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "tg-server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addr    = flag.String("addr", "localhost:8080", "address to listen on")
		seed    = flag.Uint64("seed", 1, "world seed; the same seed always generates the same world")
		fixture = flag.String("fixture", "", "load this fixture file instead of generating a world")
		rate    = flag.Duration("tick", 0, "tick interval; zero uses the simulation's own rate")
		verbose = flag.Bool("v", false, "log connections and errors")
	)
	flag.Parse()

	level := slog.LevelWarn
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	s, err := buildSim(*fixture, *seed)
	if err != nil {
		return err
	}

	srv := wsserver.New(s, wsserver.Options{TickRate: *rate, Logger: log})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	mux := http.NewServeMux()
	mux.Handle("/play", srv.Handler())
	listener := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = listener.Shutdown(shutdown)
	}()
	go func() {
		log.Info("listening", "addr", *addr, "seed", *seed, "fixture", *fixture)
		fmt.Fprintf(os.Stderr, "tg-server listening on ws://%s/play\n", *addr)
		if err := listener.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", "error", err)
		}
	}()

	return srv.Run(ctx)
}

func buildSim(fixture string, seed uint64) (*sim.Sim, error) {
	if fixture == "" {
		return sim.New(seed), nil
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		return nil, fmt.Errorf("reading fixture: %w", err)
	}
	w, _, err := world.LoadFixture(data)
	if err != nil {
		return nil, err
	}
	return sim.NewWithWorld(w, seed), nil
}
