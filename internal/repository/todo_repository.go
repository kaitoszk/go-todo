// repository パッケージ：DB操作（SQL）を集約する
// Laravel の Eloquent / Repository 層に相当
package repository

import (
	"database/sql"

	// 自分のモジュール配下の model パッケージを import する
	// パスは go.mod の module 名 + フォルダパス
	// go.mod が "module todo" なら "todo/internal/model"
	"todo/internal/model"
)

// TodoRepository は DB接続を保持し、todos に対する操作を提供する構造体
//
// なぜ構造体にするのか（グローバル変数 db をやめる理由）：
//   これまで db をグローバル変数にしていたが、実務では避ける
//   理由：どこからでも書き換えられて、依存関係が見えなくなる
//   代わりに「db を持った構造体」を作り、db接続が必要なメソッドにレシーバとして渡す（依存性注入）
type TodoRepository struct {
	db *sql.DB // この repository が使うDB接続
}

// NewTodoRepository は TodoRepository を生成するコンストラクタ関数
//
// Go には Laravel の new のような構文がないので、
// 慣習として New〇〇 という関数で構造体を初期化して返す
// 引数で db を受け取り、フィールドにセットして返す
func NewTodoRepository(db *sql.DB) *TodoRepository {
	return &TodoRepository{db: db}
}

// 以降、これまでの関数を「TodoRepository のメソッド」に変える
// レシーバ (r *TodoRepository) を付けることで、r.db でDB接続にアクセスできる

// GetAll：全件取得
func (r *TodoRepository) GetAll() ([]model.Todo, error) {
	rows, err := r.db.Query("SELECT id, title, done, created_at FROM todos ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []model.Todo
	for rows.Next() {
		var t model.Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return todos, nil
}

// GetByID：1件取得
func (r *TodoRepository) GetByID(id int) (model.Todo, error) {
	var t model.Todo
	err := r.db.QueryRow(
		"SELECT id, title, done, created_at FROM todos WHERE id = $1",
		id,
	).Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt)
	if err != nil {
		return model.Todo{}, err
	}
	return t, nil
}

// Create：追加
func (r *TodoRepository) Create(title string) error {
	_, err := r.db.Exec("INSERT INTO todos (title) VALUES ($1)", title)
	return err
}

// Update：タイトル更新
func (r *TodoRepository) Update(title string, id int) error {
	_, err := r.db.Exec("UPDATE todos SET title = $1 WHERE id = $2", title, id)
	return err
}

// Delete：削除
func (r *TodoRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM todos WHERE id = $1", id)
	return err
}

// Toggle：完了フラグ反転
func (r *TodoRepository) Toggle(id int) error {
	_, err := r.db.Exec("UPDATE todos SET done = NOT done WHERE id = $1", id)
	return err
}