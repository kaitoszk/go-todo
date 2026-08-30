package repository

import (
	"database/sql"
	"errors"
	"reflect"
	"regexp"
	"testing"
	"todo/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
)

// todo_repository.goのGetAllメソッドの正常系
// 2行返ってきた（取得された）時、[]model.Todoに正しく詰められるか
// func (r *TodoRepository) GetAll() ([]model.Todo, error) {
func TestGetAll_Success(t *testing.T) {
	// 偽DBの作成
	// 第一：dbで*sql.DBが返る
	// 第二：Sqlｍockインターフェースが返る
	// 第三：偽DB生成の失敗エラー（生成成功で、nilが入る）
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmockの生成に失敗: %v", err)
	}
	defer db.Close()

	// テスト内で、期待した内容が全て返ってきたか確認している
	defer func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("消化されていない期待がある: %v", err)
		}
	}()

	// NewRowsで、カラムを指定
	// メソッドチェーンで、AddRowを使用し、上記の順番に合わせる形で値を追加
	rows := sqlmock.NewRows([]string{"id", "title", "done", "created_at"}).
		AddRow(1, "買い物", false, "2026-01-01 10:00:00").
		AddRow(2, "掃除", true, "2026-01-02 11:00:00")

	// ExpectQueryで、テストしたい（todo_repository.goのGetAllメソッド）SQL文章を記載
	// QuoteMetaで、SQL文をエスケープ
	// WillReturnRowsで、期待する取得結果を記載
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, title, done, created_at FROM todos ORDER BY id ASC",
	)).WillReturnRows(rows)

	// 実行
	// todo_repository.goのNewTodoRepository関数を実行し、
	repo := NewTodoRepository(db)
	// todo_repository.goのGetAllメソッドを実行
	// gotには、上記AddRowでチェーンした内容が入っている
	got, err := repo.GetAll()

	// 検証1：エラーが返っていないか
	if err != nil {
		t.Fatalf("エラーが返った: %v", err)
	}

	// 検証2：中身が期待通りか
	want := []model.Todo{
		{ID: 1, Title: "買い物", Done: false, CreatedAt: "2026-01-01 10:00:00"},
		{ID: 2, Title: "掃除", Done: true, CreatedAt: "2026-01-02 11:00:00"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("結果が違う:\n want=%+v\n got=%+v", want, got)
	}
}

// DB側で問題が起こった時に、nilとエラーが返るか
// 実DBでは起こせないケースの検証ができる
func TestGetAll_Errors(t *testing.T) {
	const query = "SELECT id, title, done, created_at FROM todos ORDER BY id ASC"

	tests := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			// case1:Query自体が失敗（接続断）
			name: "異常系_Queryが失敗",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(query)).
					WillReturnError(errors.New("connection refused"))
			},
			wantErr: true,
		},
		{
			// case2:Scanが失敗（DBの方と構造体の方が合わない）
			name: "異常系_Scanが失敗",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "title", "done", "created_at"}).
					AddRow("これは数値ではない", "買い物", false, "2026-01-01 10:00:00")
				mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)
			},
			wantErr: true,
		},
		{
			// case3:行の読み取り途中でエラー
			// 1行目は正常に読めたが、2行目の取得中に接続が切れた状況

			name: "異常系_rows.Errが返る",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "title", "done", "created_at"}).
					AddRow(1, "買い物", false, "2026-01-01 10:00:00").
					AddRow(2, "掃除", true, "2026-01-02 11:00:00").
					RowError(1, errors.New("読み取り中に接続が切れた"))
				mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnRows(rows)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmockの生成に失敗: %v", err)
			}
			defer db.Close()

			// 期待の消化チェックを予約
			defer func() {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Errorf("消化されていない期待がある: %v", err)
				}
			}()
			// テストテーブルに持たせた関数を呼ぶ
			tt.setupMock(mock)

			// TODO
			repo := NewTodoRepository(db)
			got, err := repo.GetAll()

			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}

			if tt.wantErr && got != nil {
				t.Errorf("エラー時はnilが返るべき: got=%+v", got)
			}
		})
	}
}

func TestGetByID(t *testing.T) {
	const query = "SELECT id, title, done, created_at FROM todos WHERE id = $1"

	tests := []struct {
		name      string
		id        int
		setupMock func(mock sqlmock.Sqlmock)
		want      model.Todo
		wantErr   bool
	}{
		{
			name: "正常系_1件取得",
			id:   1,
			setupMock: func(mock sqlmock.Sqlmock) {
				// 偽の結果セットを作成
				rows := sqlmock.NewRows([]string{"id", "title", "done", "created_at"}).
					AddRow(1, "買い物", false, "2026-01-01 10:00:00")
				// 期待するSQL文と、取得されるid番号を指定
				mock.ExpectQuery(regexp.QuoteMeta(query)).
					WithArgs(1).
					WillReturnRows(rows)
			},
			want:    model.Todo{ID: 1, Title: "買い物", Done: false, CreatedAt: "2026-01-01 10:00:00"},
			wantErr: false,
		},
		{
			name: "異常系_該当データなし",
			id:   999,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(query)).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			want:    model.Todo{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmockの生成に失敗: %v", err)
			}
			defer db.Close()

			defer func() {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Errorf("消化されていない期待がある: %v", err)
				}
			}()

			tt.setupMock(mock)

			repo := NewTodoRepository(db)
			got, err := repo.GetByID(tt.id)

			// case1 エラーの有無
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}

			// case2 戻り値の中身
			if got != tt.want {
				t.Errorf("結果が違う:\n want=%+v\n got=%+v", tt.want, got)
			}

		})
	}
}

func TestCreate(t *testing.T) {
	const query = "INSERT INTO todos (title) VALUES ($1)"

	tests := []struct {
		name      string
		title     string
		setupMock func(mock sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name:  "正常系_1件登録",
			title: "買い物",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(query)).
					WithArgs("買い物").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name: "異常系_Execが失敗",
			title: "買い物",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(query)).
					WithArgs("買い物").
					WillReturnError(errors.New("insert失敗"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmockの生成に失敗：%v", err)
			}
			defer db.Close()

			defer func() {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Errorf("消化されていない機体がある：%v", err)
				}
			}()

			tt.setupMock(mock)

			repo := NewTodoRepository(db)

			err = repo.Create(tt.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			
		})
	}
}
