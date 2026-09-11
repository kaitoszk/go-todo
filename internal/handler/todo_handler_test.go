package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todo/internal/model"
)

// repositoryの偽物。DB接続は一切持たない。
// フィールドは2種類：
//   記録用（〜Called, 〜Title, 〜ID）：呼ばれた事実と引数を保存。モックが書き、テストが読む
//   仕込み用（〜Err, 〜Result）      ：返させたい値。テストが書き、モックが読んで返す
type mockTodoRepo struct {
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

// テスト用のlogger。io.Discardに書くのでターミナルには何も出力されない
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// 以下の6メソッドで、todo_handler.goのinterface内のメソッドを満たしているとみなされて
// ダックタイピング的に整合性が取れている状態になる。
// ctxは中で使わないが、シグネチャを揃えないとinterfaceを満たせないので受け取る
func (m *mockTodoRepo) GetAll(ctx context.Context) ([]model.Todo, error) {
	return m.getAllResult, m.getAllErr
}

func (m *mockTodoRepo) GetByID(ctx context.Context, id int) (model.Todo, error) {
	return model.Todo{ID: id, Title: "dummy"}, m.getByIDErr
}

func (m *mockTodoRepo) Create(ctx context.Context, title string) error {
	m.createCalled = true
	m.createTitle = title
	return m.createErr
}

func (m *mockTodoRepo) Update(ctx context.Context, title string, id int) error {
	m.updateCalled = true
	m.updateTitle = title
	m.updateID = id
	return m.updateErr
}

func (m *mockTodoRepo) Delete(ctx context.Context, id int) error {
	m.deleteCalled = true
	m.deleteID = id
	return m.deleteErr
}

func (m *mockTodoRepo) Toggle(ctx context.Context, id int) error {
	m.toggleCalled = true
	m.toggleID = id
	return m.toggleErr
}

// Addのテスト（POST /add）
func TestAdd(t *testing.T) {
	tests := []struct {
		name         string // ケース名（失敗時に表示される）
		body         string // 入力内容
		createErr    error  // モックのCreateに仕込むエラー（nilなら成功）
		wantCreated  bool   // Createが呼ばれるべきか
		wantStatus   int    // 期待HTTPステータス
		wantLocation string // 期待リダイレクト先（""なら検証しない）
	}{
		{
			name:         "正常系_titleアリ",
			body:         "title=shopping",
			createErr:    nil,
			wantCreated:  true,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "異常系_title空は弾く",
			body:         "title=",
			createErr:    nil,
			wantCreated:  false,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "異常系_DB書き込み失敗で500",
			body:         "title=shopping",
			createErr:    errors.New("DB書き込み失敗"),
			wantCreated:  true,
			wantStatus:   http.StatusInternalServerError,
			wantLocation: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTodoRepo{createErr: tt.createErr}
			h := NewTodoHandler(mock, newTestLogger())

			req := httptest.NewRequest(http.MethodPost, "/add", strings.NewReader(tt.body))
			// Addはr.FormValueでボディを読むのでContent-Typeが必須。
			// 付け忘れるとボディがパースされず、titleが常に空になる
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

// Deleteのテスト（POST /delete/{id}）
func TestDelete(t *testing.T) {
	tests := []struct {
		name         string // ケース名
		pathValue    string // {id} に入る値
		deleteErr    error  // モックのDeleteに仕込むエラー
		wantDeleted  bool   // Deleteが呼ばれるべきか
		wantDeleteID int    // Deleteに渡されるべきID（呼ばれないケースは0）
		wantStatus   int    // 期待HTTPステータス
	}{
		{
			name:         "正常系_1件削除",
			pathValue:    "5",
			deleteErr:    nil,
			wantDeleted:  true,
			wantDeleteID: 5,
			wantStatus:   http.StatusSeeOther,
		},
		{
			// "abc"は数値変換できないので、strconv.Atoiがエラーを返す
			name:         "異常系_数値でないidは弾く",
			pathValue:    "abc",
			deleteErr:    nil,
			wantDeleted:  false,
			wantDeleteID: 0,
			wantStatus:   http.StatusSeeOther,
		},
		{
			name:         "異常系_DB削除失敗で500",
			pathValue:    "5",
			deleteErr:    errors.New("db削除失敗"),
			wantDeleted:  true,
			wantDeleteID: 5,
			wantStatus:   http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTodoRepo{deleteErr: tt.deleteErr}
			h := NewTodoHandler(mock, newTestLogger())

			req := httptest.NewRequest(http.MethodPost, "/delete/"+tt.pathValue, nil)

			// 【修正①・最重要】PathValueを手動でセットする。
			//
			// r.PathValue("id") が返す値は、muxがパターン "POST /delete/{id}" と
			// マッチさせたときに *http.Request へ書き込むもの。
			// handlerを直接呼ぶこのテストではmuxを通らないため、
			// これを書かないと r.PathValue("id") が空文字を返す。
			//
			// 結果、strconv.Atoi("") が失敗して全ケースが「不正id」扱いになり、
			// 正常系が落ちる。SetPathValueはこの用途のためにGo 1.22で追加された
			req.SetPathValue("id", tt.pathValue)

			rec := httptest.NewRecorder()

			h.Delete(rec, req)

			// 検証1：Deleteが呼ばれたか
			if mock.deleteCalled != tt.wantDeleted {
				t.Errorf("deleteCalled: want=%v, got=%v", tt.wantDeleted, mock.deleteCalled)
			}

			// 検証2：正しくidを抽出できたか
			if mock.deleteID != tt.wantDeleteID {
				t.Errorf("deleteID: want=%d, got=%d", tt.wantDeleteID, mock.deleteID)
			}

			// 検証3：ステータスコード
			if rec.Code != tt.wantStatus {
				t.Errorf("status: want=%d, got=%d", tt.wantStatus, rec.Code)
			}
		})
	}
}

// Toggleのテスト（POST /toggle/{id}）
func TestToggle(t *testing.T) {
	tests := []struct {
		name         string // ケース名
		pathValue    string // {id} に入る値
		toggleErr    error  // モックのToggleに仕込むエラー
		wantToggled  bool   // Toggleが呼ばれるべきか
		wantToggleID int    // Toggleに渡されるべきID
		wantStatus   int    // 期待HTTPステータス
	}{
		{
			name:         "正常系_1件更新",
			pathValue:    "1",
			toggleErr:    nil,
			wantToggled:  true,
			wantToggleID: 1,
			wantStatus:   http.StatusSeeOther,
		},
		{
			name:         "異常系_数値でないidは弾く",
			pathValue:    "abc",
			toggleErr:    nil,
			wantToggled:  false,
			wantToggleID: 0,
			wantStatus:   http.StatusSeeOther,
		},
		{
			name:         "異常系_DB更新失敗で500",
			pathValue:    "1",
			toggleErr:    errors.New("db更新失敗"),
			wantToggled:  true,
			wantToggleID: 1,
			wantStatus:   http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTodoRepo{toggleErr: tt.toggleErr}
			h := NewTodoHandler(mock, newTestLogger())

			req := httptest.NewRequest(http.MethodPost, "/toggle/"+tt.pathValue, nil)
			req.SetPathValue("id", tt.pathValue) // 【修正①】
			rec := httptest.NewRecorder()

			h.Toggle(rec, req)

			// 検証1：Toggleが呼ばれたか
			if mock.toggleCalled != tt.wantToggled {
				t.Errorf("toggleCalled: want=%v, got=%v", tt.wantToggled, mock.toggleCalled)
			}

			// 検証2：正しくidを抽出できたか
			if mock.toggleID != tt.wantToggleID {
				t.Errorf("toggleID: want=%d, got=%d", tt.wantToggleID, mock.toggleID)
			}

			// 検証3：ステータスコード
			if rec.Code != tt.wantStatus {
				t.Errorf("status: want=%d, got=%d", tt.wantStatus, rec.Code)
			}
		})
	}
}

// Indexのテスト（GET /）
func TestIndex_DBError(t *testing.T) {
	mock := &mockTodoRepo{getAllErr: errors.New("db取得失敗")}
	h := NewTodoHandler(mock, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.Index(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want=500, got=%d", rec.Code)
	}
}

// EditFormのテスト（GET /edit/{id}）
func TestEditForm(t *testing.T) {
	tests := []struct {
		name       string // ケース名
		pathValue  string // {id} に入る値
		getByIDErr error  // モックのGetByIDに仕込むエラー
		wantStatus int    // 期待HTTPステータス
	}{
		{
			name:       "異常系_数値でないidは弾く",
			pathValue:  "abc",
			getByIDErr: nil,
			wantStatus: http.StatusSeeOther,
		},
		{
			name:       "異常系_該当データなしで一覧へ戻す",
			pathValue:  "999",
			getByIDErr: errors.New("データなし"),
			wantStatus: http.StatusSeeOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTodoRepo{getByIDErr: tt.getByIDErr}
			h := NewTodoHandler(mock, newTestLogger())

			req := httptest.NewRequest(http.MethodGet, "/edit/"+tt.pathValue, nil)
			req.SetPathValue("id", tt.pathValue) // 【修正①】
			rec := httptest.NewRecorder()

			h.EditForm(rec, req)

			// 検証1：ステータスコード
			if rec.Code != tt.wantStatus {
				t.Errorf("status: want=%d, got=%d", tt.wantStatus, rec.Code)
			}

			// 検証2：リダイレクト先が一覧か
			if rec.Code == http.StatusSeeOther {
				if got := rec.Header().Get("Location"); got != "/" {
					t.Errorf("Location: want=/, got=%s", got)
				}
			}
		})
	}
}

// Updateのテスト（POST /edit/{id}）
//
// 【修正②】旧TestEditからPOST側だけを取り出したもの
func TestUpdate(t *testing.T) {
	tests := []struct {
		name         string // ケース名
		pathValue    string // {id} に入る値
		body         string // リクエストボディ
		updateErr    error  // モックのUpdateに仕込むエラー
		wantUpdated  bool   // Updateが呼ばれるべきか
		wantUpdateID int    // Updateに渡されるべきID
		wantStatus   int    // 期待HTTPステータス
	}{
		{
			name:         "正常系_1件更新",
			pathValue:    "1",
			body:         "title=test99",
			updateErr:    nil,
			wantUpdated:  true,
			wantUpdateID: 1,
			wantStatus:   http.StatusSeeOther,
		},
		{
			name:         "異常系_数値でないidは弾く",
			pathValue:    "abc",
			body:         "title=test99",
			updateErr:    nil,
			wantUpdated:  false,
			wantUpdateID: 0,
			wantStatus:   http.StatusSeeOther,
		},
		{
			name:         "異常系_title空は弾く",
			pathValue:    "1",
			body:         "title=",
			updateErr:    nil,
			wantUpdated:  false,
			wantUpdateID: 0,
			wantStatus:   http.StatusSeeOther,
		},
		{
			name:         "異常系_DB更新失敗で500",
			pathValue:    "1",
			body:         "title=test99",
			updateErr:    errors.New("db更新失敗"),
			wantUpdated:  true,
			wantUpdateID: 1,
			wantStatus:   http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTodoRepo{updateErr: tt.updateErr}
			h := NewTodoHandler(mock, newTestLogger())

			req := httptest.NewRequest(http.MethodPost, "/edit/"+tt.pathValue,
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.SetPathValue("id", tt.pathValue) // 【修正①】
			rec := httptest.NewRecorder()

			h.Update(rec, req)

			// 検証1：Updateが呼ばれたか
			if mock.updateCalled != tt.wantUpdated {
				t.Errorf("updateCalled: want=%v, got=%v", tt.wantUpdated, mock.updateCalled)
			}

			// 検証2：Updateに正しいidが渡ったか
			if mock.updateID != tt.wantUpdateID {
				t.Errorf("updateID: want=%d, got=%d", tt.wantUpdateID, mock.updateID)
			}

			// 検証3：ステータスコード
			if rec.Code != tt.wantStatus {
				t.Errorf("status: want=%d, got=%d", tt.wantStatus, rec.Code)
			}
		})
	}
}

// エラー発生時の「2つの宛先」を検証する。
//   宛先1（ブラウザ）：固定文言のみ。errの中身が漏れていないこと
//   宛先2（ログ）    ：errの中身が記録されていること
//
// 将来 http.Error(w, err.Error(), ...) に戻されたら、ここで赤くなって止まる
func TestErrorResponse_DoesNotLeakDetails(t *testing.T) {
	// JSONHandlerはダブルクォートを \" にエスケープするため、
	// 検証用の文字列にクォートを含めない
	const dbErrMsg = "pq: password authentication failed for user"

	tests := []struct {
		name string

		// モックの作り方。仕込むフィールドがケースごとに違う
		// （getAllErr / createErr / ...）ためデータでは表現できず、関数で持つ
		newMock func(err error) *mockTodoRepo

		// 呼ぶhandlerメソッド。名前がケースごとに違うため関数で持つ。
		// handlerは全部(w, r)の同じシグネチャなので、中身は1行で済む
		call func(h *TodoHandler, w http.ResponseWriter, r *http.Request)

		// リクエストの作り方。ボディやPathValueの有無がケースごとに違う
		req func() *http.Request
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
				r := httptest.NewRequest(http.MethodPost, "/add",
					strings.NewReader("title=shopping"))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return r
			},
		},
		{
			// 【修正②】handler分割に伴い、Updateのケースを追加
			name: "Update_Update失敗",
			newMock: func(err error) *mockTodoRepo {
				return &mockTodoRepo{updateErr: err}
			},
			call: func(h *TodoHandler, w http.ResponseWriter, r *http.Request) {
				h.Update(w, r)
			},
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/edit/1",
					strings.NewReader("title=updated"))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				r.SetPathValue("id", "1") // 【修正①】
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
				r := httptest.NewRequest(http.MethodPost, "/delete/1", nil)
				r.SetPathValue("id", "1") // 【修正①】
				return r
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
				r := httptest.NewRequest(http.MethodPost, "/toggle/1", nil)
				r.SetPathValue("id", "1") // 【修正①】
				return r
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ログの出力先をバッファにする。
			// io.Discard（捨てる）ではなく &buf（溜める）を渡すことで、
			// 出力内容を buf.String() で取り出して検証できる
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			mock := tt.newMock(errors.New(dbErrMsg))
			h := NewTodoHandler(mock, logger)
			rec := httptest.NewRecorder()

			// tt.req() でリクエストを作り、tt.call(...) でhandlerを呼ぶ
			tt.call(h, rec, tt.req())

			// 検証1：ステータスは500
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status: want=500, got=%d", rec.Code)
			}

			// 検証2：ブラウザにerrの中身が漏れていない（このテストの本命）
			body := rec.Body.String()
			if strings.Contains(body, dbErrMsg) {
				t.Errorf("エラーの中身がレスポンスに漏れている: body=%q", body)
			}

			// 検証3：ブラウザには固定文言が返っている
			if !strings.Contains(body, "Internal Server Error") {
				t.Errorf("固定文言が返っていない: body=%q", body)
			}

			// 検証4：ログにはerrの中身が記録されている
			logOutput := buf.String()
			if !strings.Contains(logOutput, dbErrMsg) {
				t.Errorf("ログにエラーの中身が記録されていない: log=%q", logOutput)
			}

			// 検証5：ログレベルがERROR
			if !strings.Contains(logOutput, `"level":"ERROR"`) {
				t.Errorf("ログレベルがERRORではない: log=%q", logOutput)
			}
		})
	}
}

// ルーティングのテスト
//
// 【修正⑥】新規追加。
// メソッド判定がhandlerからmuxへ移ったので、
// TestAdd / TestToggle から削除した「GETは弾く」の検証をここへ移した。
//
// handlerを直接呼ぶのではなく mux.ServeHTTP を通すのがポイント。
// これでルーティングの判定が実際に走る
func TestRouter(t *testing.T) {
	tests := []struct {
		name       string // ケース名
		method     string // HTTPメソッド
		path       string // リクエストURL
		wantStatus int    // 期待HTTPステータス
	}{
		{
			// 405はmuxが返す。h.Addは呼ばれない
			name:       "GETでaddは405",
			method:     http.MethodGet,
			path:       "/add",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "GETでtoggleは405",
			method:     http.MethodGet,
			path:       "/toggle/1",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			// ルートはGETのみ登録しているのでPOSTは405
			name:       "POSTでルートは405",
			method:     http.MethodPost,
			path:       "/",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			// "GET /{$}" にしたことで未登録パスが404になる。
			// {$}を付けずに "GET /" とすると前方一致になり、
			// ここが200（Indexが表示される）になってしまう
			name:       "未登録パスは404",
			method:     http.MethodGet,
			path:       "/foobar",
			wantStatus: http.StatusNotFound,
		},
		{
			// {id}は1セグメントにしかマッチしないので、
			// 余計な階層があるとどのパターンにも一致しない
			name:       "余計な階層は404",
			method:     http.MethodPost,
			path:       "/toggle/1/2",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTodoRepo{}
			h := NewTodoHandler(mock, newTestLogger())

			// 本番と同じルーティングテーブルを組み立てる。
			// main.goに直書きしていたらこれができない。
			// router.goに切り出したのはこのため
			mux := NewRouter(h)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			// handlerを直接呼ぶのではなく、muxに処理させる
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status: want=%d, got=%d", tt.wantStatus, rec.Code)
			}
		})
	}
}