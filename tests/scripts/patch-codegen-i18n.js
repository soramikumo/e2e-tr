#!/usr/bin/env node
// Playwright codegen の UI ツールチップを日本語化するパッチスクリプト
// npm install 後に postinstall として自動実行される。
//
// codegen には 2 種類の UI があり、それぞれ別ファイルから描画される:
//   1. Inspector ウィンドウ（コード表示パネル）  -> vite/recorder/assets/index-*.js
//   2. 操作対象ページ上に浮かぶツールバー        -> coreBundle.js
// 両方をパッチしないと、片方だけ英語のまま残る。

const fs = require('fs');
const path = require('path');

const PW_CORE = path.join(__dirname, '../node_modules/playwright-core/lib');
const RECORDER_ASSET_DIR = path.join(PW_CORE, 'vite/recorder/assets');
const CORE_BUNDLE = path.join(PW_CORE, 'coreBundle.js');

const PATCH_MARKER = '/* __i18n_patched__ */';

// --- 1. Inspector ウィンドウ (vite bundle) ---------------------------------
// HTML の title 属性が title:"..." という形でバンドルされている。
const VITE_RULES = [
  ['title:"Source chooser"',          'title:"ソース選択（タブ・ページ切替）"'],
  ['title:Ae?"Stop Recording":"Start Recording"', 'title:Ae?"録画を停止":"録画を開始"'],
  ['title:"Pick locator"',            'title:"要素を選択（ロケーター取得）"'],
  ['title:"Assert visibility"',       'title:"表示確認のアサーションを追加"'],
  ['title:"Assert text"',             'title:"テキスト確認のアサーションを追加"'],
  ['title:"Assert value"',            'title:"値確認のアサーションを追加"'],
  ['title:"Assert snapshot"',         'title:"スナップショットのアサーションを追加"'],
  ['title:"Resume (F8)"',             'title:"再開 (F8)"'],
  ['title:"Pause (F8)"',              'title:"一時停止 (F8)"'],
  ['title:"Step over (F10)"',         'title:"ステップオーバー (F10)"'],
  ['title:"Clear"',                   'title:"クリア（コードを消去）"'],
  ['title:"Settings"',                'title:"設定"'],
  ['title:"Automatically generate assertions while recording"',
   'title:"録画中に自動でアサーションを生成"'],
];

// --- 2. ページ上ツールバー (coreBundle) ------------------------------------
// recorder のボタンが this._xxxToggle.title = "..." という形で設定される。
// 変数名込みで完全一致させるので、同名の別文字列を誤置換する心配がない。
const CORE_RULES = [
  ['this._recordToggle.title = "Record"',                 'this._recordToggle.title = "録画 / 停止"'],
  ['this._pickLocatorToggle.title = "Pick locator"',      'this._pickLocatorToggle.title = "要素を選択（ロケーター取得）"'],
  ['this._assertVisibilityToggle.title = "Assert visibility"', 'this._assertVisibilityToggle.title = "表示確認のアサーションを追加"'],
  ['this._assertTextToggle.title = "Assert text"',        'this._assertTextToggle.title = "テキスト確認のアサーションを追加"'],
  ['this._assertValuesToggle.title = "Assert value"',     'this._assertValuesToggle.title = "値確認のアサーションを追加"'],
  ['this._assertSnapshotToggle.title = "Assert snapshot"', 'this._assertSnapshotToggle.title = "スナップショットのアサーションを追加"'],
];

function findViteBundle() {
  const entries = fs.readdirSync(RECORDER_ASSET_DIR)
    .filter(f => f.startsWith('index-') && f.endsWith('.js'));
  if (entries.length === 0) throw new Error('vite recorder bundle not found in ' + RECORDER_ASSET_DIR);
  return path.join(RECORDER_ASSET_DIR, entries[0]);
}

function patchFile(filePath, rules) {
  let src = fs.readFileSync(filePath, 'utf8');
  const rel = path.relative(process.cwd(), filePath);

  if (src.startsWith(PATCH_MARKER)) {
    console.log(`[patch-codegen-i18n] ${rel} は適用済み、スキップ`);
    return;
  }

  for (const [en, ja] of rules) {
    if (!src.includes(en)) {
      console.warn(`[patch-codegen-i18n] WARNING: ${rel} 内に "${en}" が見つからず — スキップ`);
      continue;
    }
    src = src.replaceAll(en, ja);
  }

  fs.writeFileSync(filePath, PATCH_MARKER + '\n' + src, 'utf8');
  console.log(`[patch-codegen-i18n] 日本語化を適用: ${rel}`);
}

patchFile(findViteBundle(), VITE_RULES);
patchFile(CORE_BUNDLE, CORE_RULES);
