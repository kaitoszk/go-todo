// handler パッケージ：HTTPリクエストの入口（Laravel の Controller 相当）
// 役割：リクエストの解釈 → repository に処理を依頼 → レスポンスを返す
// SQL は書かない（DB操作は repository に委譲）
// ここから
package handler

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"todo/internal/model"
)

// TodoRepository は handler が必要とする repository の「契約（interface）」
//
// 【重要】使う側=handler が定義する interface。
//
//	handler は「こういうメソッドを持つ何か」が欲しいと宣言するだけで、
//	実装が本物のDB接続かテスト用モックかは知らない・気にしない。
//
// 【暗黙的実装】repository.TodoRepository は既にこの6メソッドを全部持つので、
//
//	何も書き換えなくても自動でこの interface を満たす（Goの構造的型付け）。
//	同じシグネチャのモックを作れば、それも自動で満たす → テストで差し替え可能。
type TodoRepository interface {
	GetAll(ctx context.Context) ([]model.Todo, error)
	GetByID(ctx context.Context, id int) (model.Todo, error)
	Create(ctx context.Context, title string) error
	Update(ctx context.Context, title string, id int) error
	Delete(ctx context.Context, id int) error
	Toggle(ctx context.Context, id int) error
}

// TodoHandler は repository（の契約）を保持し、HTTPハンドラのメソッドを提供する
type TodoHandler struct {
	// interface型で保持する。本番は本物、テストはモックをここに差し込める。
	repo   TodoRepository
	logger *slog.Logger
}

// NewTodoHandler は TodoHandler を生成するコンストラクタ
//
// 引数は interface型（TodoRepository）。"Accept interfaces" の実践。
// main.go 側は本物の *repository.TodoRepository を渡せばよい
// （本物は interface を満たすので interface型の引数にそのまま入る）。
func NewTodoHandler(repo TodoRepository, logger *slog.Logger) *TodoHandler {
	return &TodoHandler{repo: repo, logger: logger}
}

// Index：一覧表示（GET /）
func (h *TodoHandler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	todos, err := h.repo.GetAll(ctx)
	if err != nil {
		// 開発者向け：エラーの中身を構造化して記録
		h.logger.Error("failed to get todos",
			slog.Any("error", err),
			slog.String("handler", "Index"),
		)
		// 利用者向け：固定文章に→ブラウザ上でDB名などの情報を隠せる
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		h.logger.Error("failed to parse template",
			slog.Any("error", err),
			slog.String("handler", "Index"),
			slog.String("template", "index.html"),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, todos); err != nil {
		h.logger.Error("failed to render template",
			slog.Any("error", err),
			slog.String("handler", "Index"),
			slog.String("template", "index.html"),
		)
		return
	}
}

// Add：追加（POST /add）
func (h *TodoHandler) Add(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	if err := h.repo.Create(ctx, title); err != nil {
		h.logger.Error("failed to create todo",
			slog.Any("error", err),
			slog.String("handler", "Add"),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Edit：編集（GET /edit/{id} で表示、POST /edit/{id} で更新）
func (h *TodoHandler) Edit(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/edit/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logger.Debug("invalid id in path",
			slog.String("handler", "Edit"),
			slog.String("path", r.URL.Path),
		)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	// POST：更新処理
	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		if title == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		if err := h.repo.Update(ctx, title, id); err != nil {
			h.logger.Error("failed to update todo",
				slog.Any("error", err),
				slog.String("handler", "Edit"),
				slog.Int("todo_id", id),
			)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// GET：編集画面の表示
	todo, err := h.repo.GetByID(ctx, id)
	if err != nil {
		h.logger.Warn("failed to get todo by id",
			slog.Any("error", err),
			slog.String("handler", "Edit"),
			slog.Int("todo_id", id),
		)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	t, err := template.ParseFiles("templates/edit.html")
	if err != nil {
		h.logger.Error("failed to parse template",
			slog.Any("error", err),
			slog.String("handler", "Edit"),
			slog.String("template", "edit.html"),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, todo); err != nil {
		h.logger.Error("failed to render template",
			slog.Any("error", err),
			slog.String("handler", "Edit"),
			slog.String("template", "edit.html"),
		)
		return
	}
}

// Delete：削除（POST /delete/{id}）
func (h *TodoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	idStr := r.URL.Path[len("/delete/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logger.Debug("invalid id in path",
			slog.String("handler", "Delete"),
			slog.String("path", r.URL.Path),
		)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	if err := h.repo.Delete(ctx, id); err != nil {
		h.logger.Error("failed to delete todo",
			slog.Any("error", err),
			slog.String("handler", "Delete"),
			slog.Int("todo_id", id),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Toggle：完了フラグ切り替え（POST /toggle/{id}）
func (h *TodoHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	// 条件：POST送信じゃなかったらtrue
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	idStr := r.URL.Path[len("/toggle/"):]
	id, err := strconv.Atoi(idStr)
	// 条件：数値に変換できないものがURLにあった場合true
	if err != nil {
		h.logger.Debug("invalid id in path",
			slog.String("handler", "Toggle"),
			slog.String("path", r.URL.Path),
		)

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	// 条件：リポジトリのToggleメソッドでerr（idがない）だったら
	// リポジトリのToggleメソッドが実行され、問題なければ、あちらでreturnされる
	if err := h.repo.Toggle(ctx, id); err != nil {
		h.logger.Error("failed to toggle todo",
			slog.Any("error", err),
			slog.String("handler", "Toggle"),
			slog.Int("todo_id", id),
		)

		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
