package handler

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"todo/internal/model"
)

type TodoRepository interface {
	GetAll(ctx context.Context) ([]model.Todo, error)
	GetByID(ctx context.Context, id int) (model.Todo, error)
	Create(ctx context.Context, title string) error
	Update(ctx context.Context, title string, id int) error
	Delete(ctx context.Context, id int) error
	Toggle(ctx context.Context, id int) error
}

type TodoHandler struct {
	repo   TodoRepository
	logger *slog.Logger
}

func NewTodoHandler(repo TodoRepository, logger *slog.Logger) *TodoHandler {
	return &TodoHandler{repo: repo, logger: logger}
}

// Index：一覧表示（GET /）
func (h *TodoHandler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	todos, err := h.repo.GetAll(ctx)
	if err != nil {
		h.logger.Error("failed to get todos",
			slog.Any("error", err),
			slog.String("handler", "Index"),
		)
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
//
// メソッド判定は mux が担当するようになったので、
// if r.Method != http.MethodPost のブロックを削除した。
// GETで来た場合は net/http が 405 Method Not Allowed を返し、
// このメソッドは呼ばれない
func (h *TodoHandler) Add(w http.ResponseWriter, r *http.Request) {
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

// EditForm：編集画面の表示（GET /edit/{id}）
//
// 旧Editから「GETのとき」の処理だけを取り出したもの。
// 1つのメソッドが1つの責務を持つ形になり、if r.Method の分岐が消えた
func (h *TodoHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	// r.PathValue("id") でパスパラメータを取得する。
	// mux に "GET /edit/{id}" と登録したので、{id} の部分がここで取れる。
	//
	// 旧: r.URL.Path[len("/edit/"):]  ← 文字列のスライス操作。
	//     プレフィックスの長さを手で数えるので、パスを変えると壊れる
	// 新: r.PathValue("id")           ← ルーターが解析済みの値を受け取るだけ
	idStr := r.PathValue("id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logger.Debug("invalid id in path",
			slog.String("handler", "EditForm"),
			slog.String("path", r.URL.Path),
		)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	todo, err := h.repo.GetByID(ctx, id)
	if err != nil {
		h.logger.Warn("failed to get todo by id",
			slog.Any("error", err),
			slog.String("handler", "EditForm"),
			slog.Int("todo_id", id),
		)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	t, err := template.ParseFiles("templates/edit.html")
	if err != nil {
		h.logger.Error("failed to parse template",
			slog.Any("error", err),
			slog.String("handler", "EditForm"),
			slog.String("template", "edit.html"),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, todo); err != nil {
		h.logger.Error("failed to render template",
			slog.Any("error", err),
			slog.String("handler", "EditForm"),
			slog.String("template", "edit.html"),
		)
		return
	}
}

// Update：更新処理（POST /edit/{id}）
//
// 旧Editから「POSTのとき」の処理だけを取り出したもの
func (h *TodoHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logger.Debug("invalid id in path",
			slog.String("handler", "Update"),
			slog.String("path", r.URL.Path),
		)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

	if err := h.repo.Update(ctx, title, id); err != nil {
		h.logger.Error("failed to update todo",
			slog.Any("error", err),
			slog.String("handler", "Update"),
			slog.Int("todo_id", id),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Delete：削除（POST /delete/{id}）
func (h *TodoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

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
	idStr := r.PathValue("id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logger.Debug("invalid id in path",
			slog.String("handler", "Toggle"),
			slog.String("path", r.URL.Path),
		)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	ctx := r.Context()

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