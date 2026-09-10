package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"todo/internal/handler"
	"todo/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// --- 1. .env 読み込み ---
	if err := godotenv.Load(); err != nil {
		logger.Error(".envの読み込みに失敗", slog.Any("error", err))
		os.Exit(1)
	}

	// --- 2. DB接続 ---
	dbConnect := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	db, err := sql.Open("postgres", dbConnect)
	if err != nil {
		logger.Error("DBハンドルの生成に失敗", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Error("DBへの接続に失敗", slog.Any("error", err))
	}
	logger.Info("DBに接続しました")

	// --- 3. 依存の組み立て（DI）---
	// db → repo → handler の順に注入する
	repo := repository.NewTodoRepository(db)
	h := handler.NewTodoHandler(repo, logger)

	mux := http.NewServeMux()

	// --- 4. ルーティング ---
	mux.HandleFunc("GET /{$}", h.Index)
	mux.HandleFunc("POST /add", h.Add)
	mux.HandleFunc("GET /edit/{id}", h.EditForm)
	mux.HandleFunc("POST /edit/{id}", h.Update)
	mux.HandleFunc("POST /delete/{id}", h.Delete)
	mux.HandleFunc("POST /toggle/{id}", h.Toggle)

	// --- 5. サーバ起動 ---
	logger.Info("サーバーを起動します", slog.String("addr", "http://localhost:8080"))

	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.Error("サーバーが異常終了しました", slog.Any("error", err))
		os.Exit(1)
	}
}
