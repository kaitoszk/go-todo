package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"todo/internal/handler"
	"todo/internal/repository"
)

func main() {
	// --- 1. .env 読み込み ---
	if err := godotenv.Load(); err != nil {
		log.Fatal(".env file load error:", err)
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
		log.Fatal("sql open error:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("db connect error:", err)
	}
	log.Println("postgres is connect OK")

	// --- 3. 依存の組み立て（DI）---
	// db → repo → handler の順に注入する
	repo := repository.NewTodoRepository(db)
	h := handler.NewTodoHandler(repo)

	// --- 4. ルーティング ---
	http.HandleFunc("/", h.Index)
	http.HandleFunc("/add", h.Add)
	http.HandleFunc("/edit/", h.Edit)
	http.HandleFunc("/delete/", h.Delete)
	http.HandleFunc("/toggle/", h.Toggle)

	// --- 5. サーバ起動 ---
	log.Println("server is action: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}