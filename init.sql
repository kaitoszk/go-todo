-- コンテナ初回起動時に自動実行される
-- todos テーブルを作成する

CREATE TABLE todos (
    -- id：主キー。SERIAL は自動採番（Laravel の auto increment 相当）
    id SERIAL PRIMARY KEY,

    -- title：TODO の内容。NOT NULL で空を禁止
    title TEXT NOT NULL,

    -- done：完了フラグ。デフォルトは false（未完了）
    done BOOLEAN NOT NULL DEFAULT false,

    -- created_at：作成日時。デフォルトで現在時刻が入る
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 動作確認用の初期データを2件入れておく
INSERT INTO todos (title) VALUES ('PostgreSQLをDockerで立てる');
INSERT INTO todos (title) VALUES ('Goから接続する');