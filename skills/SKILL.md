---
description: |
  Bashツールでコマンド実行する時、出力が長くなりそうなら `2>&1 | smart-trunc` を付ける。
  pytest, npm run build, make, docker logs, git log など長い出力のコマンドに必須。
  短い出力はそのまま通すので、迷ったら常にパイプに挟んでOK。コスト0。
  「出力が長い」「ログが切れた」「エラーが見えない」時にPROACTIVELY使用すること。
---

# smart-trunc

コマンド出力をパイプで受け取り、エラー行や重要な行を優先的に残しつつ、指定バイト数以内に短縮する。

## モード自動選択ルール

**コマンドを見て、以下のルールで必ずモードを付けること。**

| コマンドパターン | モード | 理由 |
|---|---|---|
| `pytest`, `go test`, `make test`, `jest`, `vitest`, `mocha`, `cargo test` | `--mode test` | PASS行除外、FAIL+サマリ保持 |
| `npm run build`, `make build`, `go build`, `cargo build`, `tsc`, `webpack` | `--mode build` | エラー行優先保持 |
| 上記以外 | 指定なし（general） | 汎用トランケーション |

**判断基準**: コマンド名に `test` が含まれるかテストランナーなら `--mode test`。`build`/`compile` 系なら `--mode build`。迷ったら指定なしでOK。

## 基本的な使い方

```bash
# テスト系 → 必ず --mode test
pytest -v 2>&1 | smart-trunc --mode test
go test ./... 2>&1 | smart-trunc --mode test
make test 2>&1 | smart-trunc --mode test
npx jest 2>&1 | smart-trunc --mode test

# ビルド系 → 必ず --mode build
npm run build 2>&1 | smart-trunc --mode build
go build ./... 2>&1 | smart-trunc --mode build

# その他 → モード指定なし
docker compose logs 2>&1 | smart-trunc
git log --oneline -50 2>&1 | smart-trunc

# カスタムパターンで保持
long-command 2>&1 | smart-trunc -k "CUSTOM_TAG" --context 5

# 構造化出力（メタデータ付き）
command 2>&1 | smart-trunc --yaml
command 2>&1 | smart-trunc --json
```

## いつ使うか

- Bashツールでコマンド実行し、出力が長くなりそうな時
- **テスト実行時は必ず `--mode test`**（PASS行を自動除外、FAIL行とサマリのみ残す）
- **ビルド実行時は必ず `--mode build`**（エラー行を優先保持）
- 短い出力はショートサーキットでそのまま通すので、迷ったら常にパイプに挟んでOK

## testモードの動作（v0.2.0）

`--mode test`では、全PASS時はサマリ行のみ出力し、FAIL時はFAIL行+エラー詳細+サマリを保持する。Go test、pytest、jest/vitestに対応。

```bash
# 全PASS → サマリ1行のみ
$ go test ./... 2>&1 | smart-trunc --mode test
ok  github.com/user/repo  0.42s

# FAIL有り → FAIL行+詳細+サマリ
$ go test ./... 2>&1 | smart-trunc --mode test
--- FAIL: TestSub (0.00s)
    sub_test.go:10: expected 3, got 4
FAIL  github.com/user/repo  0.05s
```

## セットアップ

```bash
# このディレクトリを ~/.claude/skills/ にコピーまたはシンボリックリンク
cp -r skills/ ~/.claude/skills/smart-trunc/
# または
ln -s "$(pwd)/skills" ~/.claude/skills/smart-trunc
```

## 詳細オプション

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `--limit` | 30000 | 出力バイト数上限 |
| `--head` | 20 | 先頭から保持する行数 |
| `--tail` | 20 | 末尾から保持する行数 |
| `--context` | 3 | マッチ行の前後に保持する行数 |
| `--mode` | general | プリセット: `general` / `test` / `build` |
| `--keep-pattern`, `-k` | なし | 追加保持パターン（正規表現、複数指定可） |
| `--json` | false | JSON構造化出力 |
| `--yaml` | false | YAML構造化出力 |
