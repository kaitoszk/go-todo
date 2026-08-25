// handler パッケージ：HTTPリクエストの入口（Laravel の Controller 相当）
// 役割：リクエストの解釈 → repository に処理を依頼 → レスポンスを返す
// SQL は書かない（DB操作は repository に委譲）
package handler

import (
	"html/template"
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
	GetAll() ([]model.Todo, error)
	GetByID(id int) (model.Todo, error)
	Create(title string) error
	Update(title string, id int) error
	Delete(id int) error
	Toggle(id int) error
}

// TodoHandler は repository（の契約）を保持し、HTTPハンドラのメソッドを提供する
type TodoHandler struct {
	// interface型で保持する。本番は本物、テストはモックをここに差し込める。
	repo TodoRepository
}

// NewTodoHandler は TodoHandler を生成するコンストラクタ
//
// 引数は interface型（TodoRepository）。"Accept interfaces" の実践。
// main.go 側は本物の *repository.TodoRepository を渡せばよい
// （本物は interface を満たすので interface型の引数にそのまま入る）。
func NewTodoHandler(repo TodoRepository) *TodoHandler {
	return &TodoHandler{repo: repo}
}

// Index：一覧表示（GET /）
func (h *TodoHandler) Index(w http.ResponseWriter, r *http.Request) {
	todos, err := h.repo.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, todos)
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

	if err := h.repo.Create(title); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Edit：編集（GET /edit/{id} で表示、POST /edit/{id} で更新）
func (h *TodoHandler) Edit(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/edit/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// POST：更新処理
	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		if title == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		if err := h.repo.Update(title, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// GET：編集画面の表示
	todo, err := h.repo.GetByID(id)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	t, err := template.ParseFiles("templates/edit.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.Execute(w, todo)
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
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err := h.repo.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 条件：リポジトリのToggleメソッドでerr（idがない）だったら
	// リポジトリのToggleメソッドが実行され、問題なければ、あちらでreturnされる
	if err := h.repo.Toggle(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
