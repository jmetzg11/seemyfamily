package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"seemyfamily.jmetzg11/internal/models"
)

type application struct {
	logger        *slog.Logger
	templateCache map[string]*template.Template
	people        *models.PersonModel
	users         *models.UserModel
	mediaURL      string
	csp           string
	sessionSecret []byte
	secureCookies bool
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	mediaURL := strings.TrimSuffix(os.Getenv("S3_PUBLIC_URL"), "/")
	if mediaURL == "" {
		logger.Error("S3_PUBLIC_URL is not set")
		os.Exit(1)
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if len(sessionSecret) < 32 {
		logger.Error("SESSION_SECRET is not set, or is shorter than 32 characters")
		os.Exit(1)
	}

	secureCookies := os.Getenv("SECURE_COOKIES") != "false"

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = pool.Ping(ctx)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	templateCache, err := newTemplateCache()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	app := &application{
		logger:        logger,
		templateCache: templateCache,
		people:        &models.PersonModel{DB: pool},
		users:         &models.UserModel{DB: pool},
		mediaURL:      mediaURL,
		csp:           buildCSP(mediaURL),
		sessionSecret: []byte(sessionSecret),
		secureCookies: secureCookies,
	}

	srv := &http.Server{
		Addr:         ":4000",
		Handler:      app.routes(),
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info("starting server", "addr", srv.Addr)

	err = srv.ListenAndServe()
	logger.Error(err.Error())
	os.Exit(1)
}
