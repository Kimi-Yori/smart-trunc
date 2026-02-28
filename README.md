# smart-trunc

LLMエージェント向けのインテリジェントな出力トランケーションツール。

コマンド出力をパイプで受け取り、エラー行や重要な行を優先的に残しつつ、指定バイト数以内に短縮する。

## なぜ必要か

LLMコーディングエージェントはコマンド出力に厳しいバイト数制限がある。制限を超えると機械的にカットされ、肝心のエラーメッセージがちょうど切断面で消えることがある。

| エージェント | 出力制限 |
|-------------|---------|
| Claude Code | 約30,000バイト |
| Codex CLI | 10,000バイト / 256行 |

`head`では中間のエラーを見逃し、`tail`では冒頭の文脈を失い、`grep`では全体構造が見えなくなる。

smart-truncは「先頭・末尾・エラー周辺」を優先保持し、重要でない部分を省略マーカー付きで間引く。入力が制限以下ならそのまま通す（ショートサーキット）ので、迷ったら常にパイプに挟んでおけばよい。

## クイックスタート

```bash
# インストール（Go 1.21以降が必要）
make build
sudo make install

# 基本: パイプに挟むだけ
some-command 2>&1 | smart-trunc

# テスト出力の短縮（FAIL行を優先保持）
pytest -v 2>&1 | smart-trunc --mode test

# ビルドログの短縮（npm ERR!等を優先保持）
npm run build 2>&1 | smart-trunc --mode build

# Codex CLI向け（10KB制限に合わせる）
make test 2>&1 | smart-trunc --limit 10000 --mode test
```

## インストール

### バイナリをダウンロード（Go不要）

静的リンク済みバイナリなので、Goが未インストールでもそのまま動く。

```bash
# Linux x86-64 / WSL2
curl -L https://github.com/Kimi-Yori/smart-trunc/releases/download/v0.2.0/smart-trunc-linux-amd64 -o smart-trunc

# macOS Apple Silicon (M1/M2/M3/M4)
curl -L https://github.com/Kimi-Yori/smart-trunc/releases/download/v0.2.0/smart-trunc-darwin-arm64 -o smart-trunc

# macOS Intel
curl -L https://github.com/Kimi-Yori/smart-trunc/releases/download/v0.2.0/smart-trunc-darwin-amd64 -o smart-trunc

chmod +x smart-trunc
sudo mv smart-trunc /usr/local/bin/
```

### 配布パッケージから（Go不要）

tar.gz を受け取った場合はこちら。展開後、自分のOS用バイナリを選んでインストールする。

```bash
tar xzf smart-trunc-v0.2.0.tar.gz
cd smart-trunc-v0.2.0

# 自分の環境に合ったバイナリを選択
# Linux x86-64 / WSL2 → smart-trunc-linux-amd64
# macOS Apple Silicon  → smart-trunc-darwin-arm64
# macOS Intel          → smart-trunc-darwin-amd64

chmod +x smart-trunc-*
sudo cp smart-trunc-$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') /usr/local/bin/smart-trunc
```

### ソースから

```bash
git clone https://github.com/Kimi-Yori/smart-trunc.git
cd smart-trunc
make build
sudo make install    # /usr/local/bin/ にコピー
```

### go install

```bash
go install github.com/Kimi-Yori/smart-trunc@latest
```

ソースビルドには Go 1.21以降が必要。

## 使い方

```bash
# バイト数制限を指定
long-command 2>&1 | smart-trunc --limit 1000

# カスタムパターンで保持（正規表現、複数指定可）
command 2>&1 | smart-trunc -k "CUSTOM_TAG" -k "IMPORTANT" --context 5

# YAML構造化出力（メタデータ付き）
command 2>&1 | smart-trunc --yaml

# JSON構造化出力
command 2>&1 | smart-trunc --json
```

## オプション

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `--limit` | 30000 | 出力バイト数上限。`0`で無制限 |
| `--head` | 20 | 先頭から保持する行数 |
| `--tail` | 20 | 末尾から保持する行数 |
| `--context` | 3 | マッチ行の前後に保持する行数 |
| `--mode` | general | プリセット: `general` / `test` / `build` |
| `--keep-pattern`, `-k` | なし | 追加保持パターン（正規表現、複数指定可） |
| `--json` | false | JSON構造化出力 |
| `--yaml` | false | YAML構造化出力 |
| `--version` | — | バージョン表示 |

補足:
- `--json`と`--yaml`は排他。両方指定するとexit 2
- `--head`, `--tail`, `--context`は非負値のみ。負値はexit 2
- `--limit 0`または負値は無制限（スコアリングのみ実行してそのまま通す）

## モード別パターン

各モードはデフォルトのスコアリングに加えて、ドメイン固有のパターンを追加する。全てcase-insensitive。

- **general** — `ERROR`, `FATAL`, `WARN`, `PANIC`, `traceback`, `exception`
- **test** — general + `FAIL`, `AssertionError`, `--- FAIL:`, test summary。**v0.2.0**: PASS行を自動除外し、サマリ行を保護。全PASS時はサマリのみ出力。Go test / pytest / jest対応
- **build** — general + `error:`, `warning:`, `npm ERR!`, syntax error, undefined reference

## testモードの動作（v0.2.0）

`--mode test`では、個別のPASS行（`--- PASS:`, `=== RUN`, `PASSED`, `✓`）を自動除外し、サマリ行（`ok github.com/...`, `N passed`, `Tests:`, `Test Suites:`）を保護する。FAIL行（score > 0）は除外されない安全設計。

```bash
# Go test: 全PASS → サマリのみ
$ go test ./... 2>&1 | smart-trunc --mode test
ok  github.com/user/repo  0.42s

# Go test: FAIL有り → FAIL行+詳細+サマリ保持
$ go test ./... 2>&1 | smart-trunc --mode test
--- FAIL: TestSub (0.00s)
    sub_test.go:10: expected 3, got 4
FAIL  github.com/user/repo  0.05s

# pytest: 全PASS → ヘッダー+サマリのみ
$ pytest -v 2>&1 | smart-trunc --mode test
============================= test session starts ==============================
...
============================== 8 passed in 0.12s ==============================
```

general/buildモードには影響なし。

## アルゴリズム

1. stdinから全行読み込み
2. モード別パターン＋カスタムパターンで各行にスコア付与
3. 先頭N行・末尾N行・高スコア行とその前後context行を保持対象にマーク
4. バイト数制限を超える場合、低スコア行から順に削除（Union-Find使用、O(n log n)）。head/tail行は最後に削除
5. 連続する保持行をブロックにまとめ、省略区間に `... (N lines omitted) ...` を挿入
6. 入力がlimit以下ならそのまま通す（ショートサーキット）

### Format別Limit制御

- **Plain text**: `--limit`の90%をコンテンツに使用（省略マーカー分を確保）
- **JSON/YAML**: `--limit`の100%を使用（メタデータも出力の一部）

### 構造化出力の構文保証

JSON/YAML出力は常に有効な構文を保証する。制限超過時はブロック単位で段階的に削減（tail → head → match の順）し、再marshalする。バイト単位の切断で構文が壊れることはない。

## 構造化出力フォーマット

```yaml
summary:
    total_lines: 1500
    kept_lines: 45
    omitted_lines: 1455
    patterns_matched: 3
blocks:
    - type: head
      start_line: 1
      end_line: 20
      content: |
        ...
    - type: omitted
      start_line: 21
      end_line: 340
      omitted_count: 320
    - type: match
      start_line: 341
      end_line: 350
      content: |
        ...
```

ブロックタイプ: `head`, `tail`, `match`, `omitted`

## 既知の制約

- **no-match + structured overflow**: 全行が等スコア（パターンマッチなし）で、`Head>0`かつ`Tail>0`、plainでは収まるがstructuredメタデータで超過する場合、tail側の行が落ちることがある。short-circuit経路が単一ブロックを生成し、削減戦略が連続スライスしか保持できないため。実用上は全行等価値なので情報損失はない。詳細は[TODO.md](TODO.md)参照。

## LLMエージェント Skills 連携

本リポジトリの `skills/SKILL.md` は [Open Agent Skills](https://agentskills.io) 仕様に準拠しており、Claude Code と Codex CLI の両方で使える。登録すると、エージェントがコマンド実行時に自動的に `| smart-trunc` を付けてくれるようになる。

### Claude Code

```bash
# リポジトリの skills/ を ~/.claude/skills/smart-trunc/ にコピー
cp -r skills/ ~/.claude/skills/smart-trunc/

# またはシンボリックリンク（リポジトリ更新が自動反映される）
ln -s "$(pwd)/skills" ~/.claude/skills/smart-trunc
```

登録後、Claude Code の新しいセッションで Skills が認識される。

### Codex CLI

```bash
# ユーザーグローバルに登録
cp -r skills/ ~/.agents/skills/smart-trunc/

# またはプロジェクトローカルに登録
cp -r skills/ .agents/skills/smart-trunc/

# またはシンボリックリンク
ln -s "$(pwd)/skills" ~/.agents/skills/smart-trunc
```

Codex CLI は出力制限が 10KB / 256行と厳しいため、恩恵が特に大きい。スキル認識後、Codex が自動で `| smart-trunc --limit 10000` を付けてくれるようになる。

### 何が変わるか

- エージェントが `pytest`, `make`, `npm run build` 等を実行する際、出力が長くなりそうなら自動で `| smart-trunc` をパイプに挟む
- エラー行・警告行が優先保持されるため、出力制限で肝心のエラーが切れる問題を回避
- 短い出力はショートサーキットでそのまま通るので、常時有効にしてもコスト0

## 開発

```bash
make test          # 全テスト実行（97テスト）
make test-update   # ゴールデンテスト更新
make coverage      # カバレッジ計測（92.9%）
make build         # ビルド
make clean         # 成果物削除
```

### プロジェクト構成

```
smart-trunc/
├── main.go              # CLIエントリポイント
├── truncate/
│   ├── block.go         # ブロック型定義・生成
│   ├── golden_test.go   # ゴールデンファイルテスト
│   ├── mode.go          # モード別パターン定義
│   ├── output.go        # Plain/JSON/YAMLフォーマッタ
│   ├── scorer.go        # 行スコアリングエンジン
│   ├── testaware.go     # テスト認識型フィルタ（PASS行除外・サマリ保護）
│   ├── truncate.go      # コアアルゴリズム
│   └── *_test.go        # ユニットテスト
├── skills/
│   └── SKILL.md         # Claude Code Skills 定義
└── testdata/            # ゴールデンテスト用データ
```

## ライセンス

MIT
