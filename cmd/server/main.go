package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/config"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpserver"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	appConfig, err := config.Load(workingDirectory)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	steamClient := steamapi.NewClient(appConfig.SteamApiTimeout, appConfig.SteamApiCacheInterval, appConfig.SteamApiKey)

	databaseConnection, err := database.Open(ctx, appConfig.DatabasePath)
	if err != nil {
		return err
	}
	defer databaseConnection.Close()
	if err := database.Migrate(ctx, databaseConnection); err != nil {
		return err
	}
	appLogger, err := logging.Open(filepath.Join(workingDirectory, "data", "wgswp.log"))
	if err != nil {
		return err
	}
	defer appLogger.Close()
	application, err := httpserver.New(databaseConnection, appLogger, httpserver.Options{
		AppOrigin:       appConfig.AppOrigin,
		PublicDirectory: filepath.Join(workingDirectory, "web", "public"),
		SteamApiKey:     appConfig.SteamApiKey,
		SteamClient:     &steamClient,
		//TrustedProxyHops: appConfig.TrustedProxyHops,
	})
	if err != nil {
		return err
	}
	defer application.Close()

	applicationServer := httpserver.NewServer(fmt.Sprintf(":%d", appConfig.Port), application.Handler)
	serverErrors := make(chan error, 1)
	serve(applicationServer, fmt.Sprintf("WGSWP is running at http://localhost:%d", appConfig.Port), serverErrors)

	select {
	case <-ctx.Done():
		return shutdownServer(applicationServer)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP server: %w", err)
	}
}

func serve(server *http.Server, message string, serverErrors chan<- error) {
	go func() {
		fmt.Println(message)
		serverErrors <- server.ListenAndServe()
	}()
}

func shutdownServer(server *http.Server) error {
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownContext)
}
