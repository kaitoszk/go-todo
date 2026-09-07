package handler

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"todo/internal/model"
)

type mockTodoRepo struct {
	// Addメソッドのテストで、このファイル内のCreateメソッドが呼ばれたかを確認
	createCalled bool   // Createが呼ばれたらtrue
	createTitle  string // Createに渡されたtitleを保存

	updateCalled bool
	updateTitle  string
	updateID     int

	deleteCalled bool
	deleteID     int

	toggleCalled bool
	toggleID     int

	getAllResult []model.Todo
	getAllErr    error
	getByIDErr   error
	createErr    error
	updateErr    error
	deleteErr    error
	toggleErr    error
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// 以下の6メソッドで、todo_handler.goのinterface内のメソッドを満たしているとみなされて
// ダックタイピング的に生合成が取れている状態になる
func (m *mockTodoRepo) GetAll() ([]model.Todo, error) {
	return m.getAllResult, m.getAllErr
}

func (m *mockTodoRepo) GetByID(id int) (model.Todo, error) {
	return model.Todo{ID: id, Title: "dummy"}, m.getByIDErr
}

func (m *mockTodoRepo) Create(title string) error {
	m.createCalled = true
	m.createTitle = title
	return m.createErr
}

func (m *mockTodoRepo) Update(title string, id int) error {
	m.updateCalled = true
	m.updateTitle = title
	m.updateID = id
	return m.updateErr
}

func (m *mockTodoRepo) Delete(id int) error {
	m.deleteCalled = true
	m.deleteID = id
	return m.deleteErr
}

func (m *mockTodoRepo) Toggle(id int) error {
	m.toggleCalled = true
	m.toggleID = id
	return m.toggleErr
}

// Addメソッドのテスト
func TestAdd(t *testing.T) {
	//
	tests := []struct {
		name         string // ケース名（失敗時に表示される）
		method       string // 送るHTTPメソッド
		body         string // 入力内容
		createErr    error  // モックのCreateに仕込むエラー（nilなら成功）
		wantCreated  bool   // Createが呼ばれるべきか
		wantStatus   int    // 期待HTTPステータス
		wantLocation string // 期待リダイレクト先、500
	}{
		{
			name:         "正常系_POSTでtitleアリ",
			method:       http.MethodPost,
			body:         "title=shopping",
			createErr:    nil,
			wantCreated:  true,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "異常系_GETは弾く",
			method:       http.MethodGet,
			body:         "",
			createErr:    nil,
			wantCreated:  false,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "異常系_title空は弾く",
			method:       http.MethodPost,
			body:         "title=",
			createErr:    nil,
			wantCreated:  false,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "異常系_DB書き込み失敗で500",
			method:       http.MethodPost,
			body:         "title=shopping",
			createErr:    errors.New("DB書き込み失敗"),
			wantCreated:  true,
			wantStatus:   http.StatusInternalServerError,
			wantLocation: "",
		},
	}

	// この辺り理解できなかったので、質問しつつobsidianにまとめる
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// todo_handler.goのAddメソッド
			mock := &mockTodoRepo{createErr: tt.createErr}
			h := NewTodoHandler(mock, newTestLogger())

			req := httptest.NewRequest(tt.method, "/add", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			// 実行
			h.Add(rec, req)

			// 検証1：Createが呼ばれたか（期待値と一致するか）
			if mock.createCalled != tt.wantCreated {
				t.Errorf("createCalled: want=%v, got=%v", tt.wantCreated, mock.createCalled)
			}

			// 検証2：ステータスコード
			if rec.Code != tt.wantStatus {
				t.Errorf("status: want=%d, got=%d", tt.wantStatus, rec.Code)
			}

			// 検証3：リダイレクト先（Locationヘッダ）
			if tt.wantLocation != "" {
				gotLocation := rec.Header().Get("Location")
				if gotLocation != tt.wantLocation {
					t.Errorf("Location: want=%s, got=%s", tt.wantLocation, gotLocation)
				}
			}
		})
	}
}

// Deleteメソッドのテスト
// /delete/5 id=5でDeleteが呼ばれ、303
func TestDelete_Success(t *testing.T) {
	mock := &mockTodoRepo{}
	h := NewTodoHandler(mock, newTestLogger())

	req := httptest.NewRequest(http.MethodPost, "/delete/5", nil)
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	// 検証1：Deleteメソッドが呼ばれたか
	if !mock.deleteCalled {
		t.Error("Deleteが呼ばれていない")
	}

	// 検証2：URL /delete/5 から正しく数値5を秀出できたか
	if mock.deleteID != 5 {
		t.Errorf("idが違う：want=5, got=%d", mock.deleteID)
	}

	// 検証3：303リダイレクトか
	if rec.Code != http.StatusSeeOther {
		t.Errorf("ステータスが違う：want=303, got=%d", rec.Code)
	}
}

func TestDelete_InvalidID(t *testing.T) {
	mock := &mockTodoRepo{}
	h := NewTodoHandler(mock, newTestLogger())

	// "abc"は数値変換できないので、strconv.Atoiがエラーを返す
	req := httptest.NewRequest(http.MethodPost, "/delete/abc", nil)
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	// 不正idはDeleteに到達しない
	if mock.deleteCalled {
		t.Error("不正なidなのにDeleteが呼ばれてしまった")
	}
}

func TestIndex_DBError(t *testing.T) {
	mock := &mockTodoRepo{getAllErr: errors.New("db取得失敗")}
	h := NewTodoHandler(mock, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.Index(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスが違う: want=500, got=%d", rec.Code)
	}
}

func TestEdit(t *testing.T) {
	tests := []struct {
		name          string // ケース名
		method        string // HTTPメソッド
		path          string // URL
		body          string // リクエスト
		updateErr     error  // UpdateがPOSTで返すエラー
		getByIDErr    error  // UpdateがGETで返すエラー
		wantUpdated   bool   // Updateが呼ばれるか
		wantUpdatedID int    // Updateに渡されるべきID
		wantStatus    int    // 期待するHTTPステータス
		wantLocation  string // 期待リダイレクト先
	}{
		{
			name:          "正常系_POSTで更新",
			method:        http.MethodPost,
			path:          "/edit/1",
			body:          "title=test99",
			updateErr:     nil,
			wantUpdated:   true,
			wantUpdatedID: 1,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/",
		},
		{
			name:          "異常系_不正idは弾く",
			method:        http.MethodPost,
			path:          "/edit/abc",
			body:          "title=test99",
			updateErr:     nil,
			wantUpdated:   false,
			wantUpdatedID: 0,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/",
		},
		{
			name:          "異常系_title空は弾く",
			method:        http.MethodPost,
			path:          "/edit/1",
			body:          "title=",
			updateErr:     nil,
			wantUpdated:   false,
			wantUpdatedID: 0,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/",
		},
		{
			name:          "異常系_Update失敗で500",
			method:        http.MethodPost,
			path:          "/edit/1",
			body:          "title=test99",
			updateErr:     errors.New("db更新失敗"),
			wantUpdated:   true,
			wantUpdatedID: 1,
			wantStatus:    http.StatusInternalServerError,
			wantLocation:  "",
		},
		{
			name:          "異常系_GETで取得失敗",
			method:        http.MethodGet,
			path:          "/edit/999",
			body:          "",
			getByIDErr:    errors.New("データなし"),
			wantUpdated:   false,
			wantUpdatedID: 0,
			wantStatus:    http.StatusSeeOther,
			wantLocation:  "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTodoRepo{
				updateErr:  tt.updateErr,
				getByIDErr: tt.getByIDErr,
			}
			h := NewTodoHandler(mock, newTestLogger())

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			h.Edit(rec, req)

			// 検証1：Updagaが呼ばれたか
			if mock.updateCalled != tt.wantUpdated {
				t.Errorf("updateCalled: want=%v, got=%v", tt.wantUpdated, mock.updateCalled)
			}

			// 検証2：Updateに正しいidが渡ったか
			if mock.updateID != tt.wantUpdatedID {
				t.Errorf("updatedID: want=%d, got=%d", tt.wantUpdatedID, mock.updateID)
			}

			// 検証3：ステータスコード
			if rec.Code != tt.wantStatus {
				t.Errorf("status: want=%d, got=%d", tt.wantStatus, rec.Code)
			}

			// 検証4：リダイレクト先
			if tt.wantLocation != "" {
				gotLocation := rec.Header().Get("Location")
				if gotLocation != tt.wantLocation {
					t.Errorf("Location: want=%s, got=%s", tt.wantLocation, gotLocation)
				}
			}
		})
	}
}

func TestToggle(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		toggleErr    error
		wantToggled  bool
		wantToggleID int
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "正常系_POSTでURL正常",
			method:       http.MethodPost,
			path:         "/toggle/1",
			toggleErr:    nil,
			wantToggled:  true,
			wantToggleID: 1,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "異常系_GETは弾く",
			method:       http.MethodGet,
			path:         "/toggle/1",
			toggleErr:    nil,
			wantToggleID: 0,
			wantToggled:  false,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "異常系_POSTでURLが文字列の場合弾く",
			method:       http.MethodPost,
			path:         "/toggle/abc",
			toggleErr:    nil,
			wantToggleID: 0,
			wantToggled:  false,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "異常系_POSTでDBエラー",
			method:       http.MethodPost,
			path:         "/toggle/1",
			toggleErr:    errors.New("db更新失敗"),
			wantToggleID: 1,
			wantToggled:  true,
			wantStatus:   http.StatusInternalServerError,
			wantLocation: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTodoRepo{toggleErr: tt.toggleErr}
			h := NewTodoHandler(mock, newTestLogger())

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			h.Toggle(rec, req)

			if mock.toggleCalled != tt.wantToggled {
				t.Errorf("toggleCalled: want=%v, got=%v", tt.wantToggled, mock.toggleCalled)
			}

			if mock.toggleID != tt.wantToggleID {
				t.Errorf("toggledID: want=%d, got=%d", tt.wantToggleID, mock.toggleID)
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("status: want=%d, got=%d", tt.wantStatus, rec.Code)
			}

			if tt.wantLocation != "" {
				gotLocation := rec.Header().Get("Location")
				if gotLocation != tt.wantLocation {
					t.Errorf("Location: want=%s, got=%s", tt.wantLocation, gotLocation)
				}
			}
		})
	}
}

// 2つのテストをしている
// ブラウザ；errの中身が漏れていないか
// ログ：errの中身が記録されているか
func TestErrorResponse_DoesNotLeakDetails(t *testing.T) {
	const dbErrMsg = "pq: relation \"todos\" does not exist"

	tests := []struct {
		name    string
		newMock func(err error) *mockTodoRepo
		call    func(h *TodoHandler, w http.ResponseWriter, r *http.Request)
		req     func() *http.Request
	}{
		{
			name: "Index_GetAll失敗",
			newMock: func(err error) *mockTodoRepo {
				return &mockTodoRepo{getAllErr: err}
			},
			call: func(h *TodoHandler, w http.ResponseWriter, r *http.Request) {
				h.Index(w, r)
			},
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
		},
		{
			name: "Add_Create失敗",
			newMock: func(err error) *mockTodoRepo {
				return &mockTodoRepo{createErr: err}
			},
			call: func(h *TodoHandler, w http.ResponseWriter, r *http.Request) {
				h.Add(w, r)
			},
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/add", strings.NewReader("title=shopping"))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return r
			},
		},
		{
			name: "Delete_Delete失敗",
			newMock: func(err error) *mockTodoRepo {
				return &mockTodoRepo{deleteErr: err}
			},
			call: func(h *TodoHandler, w http.ResponseWriter, r *http.Request) {
				h.Delete(w, r)
			},
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/delete/1", nil)
			},
		},
		{
			name: "Toggle_Toggle失敗",
			newMock: func(err error) *mockTodoRepo {
				return &mockTodoRepo{toggleErr: err}
			},
			call: func(h *TodoHandler, w http.ResponseWriter, r *http.Request) {
				h.Toggle(w, r)
			},
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/toggle/1", nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			mock := tt.newMock(errors.New(dbErrMsg))
			h := NewTodoHandler(mock, logger)
			rec := httptest.NewRecorder()

			tt.call(h, rec, tt.req())

			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status: want=500, got=%d", rec.Code)
			}

			body := rec.Body.String()
			if strings.Contains(body, dbErrMsg) {
				t.Errorf("エラーの中身がレスポンスに漏れている：body=%q", body)
			}

			if !strings.Contains(body, "Internal Server Error") {
				t.Errorf("固定文章が返っていない：body=%q", body)
			}

			logOutput := buf.String()
			if !strings.Contains(logOutput, dbErrMsg) {
				t.Errorf("ログにエラーの中身が記録されていない：log=%q", logOutput)
			}

			if !strings.Contains(logOutput, `"level":"ERROR"`) {
				t.Errorf("ログレベルがERRORではない：log=%q", logOutput)
			}
		})
	}
}
