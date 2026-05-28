#!/usr/bin/env node
// Playwright codegenツールバーのツールチップを日本語化するパッチスクリプト
// npm install 後に postinstall として自動実行される

const fs = require('fs');
const path = require('path');

const RECORDER_ASSET_DIR = path.join(
  __dirname,
  '../node_modules/playwright-core/lib/vite/recorder/assets'
);

const TRANSLATIONS = [
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

function findRecorderBundle() {
  const entries = fs.readdirSync(RECORDER_ASSET_DIR).filter(f => f.startsWith('index-') && f.endsWith('.js'));
  if (entries.length === 0) throw new Error('recorder bundle not found in ' + RECORDER_ASSET_DIR);
  return path.join(RECORDER_ASSET_DIR, entries[0]);
}

function applyPatch() {
  const bundlePath = findRecorderBundle();
  let src = fs.readFileSync(bundlePath, 'utf8');

  if (src.includes('__i18n_patched__')) {
    console.log('[patch-codegen-i18n] already patched, skipping.');
    return;
  }

  let patched = src;
  for (const [en, ja] of TRANSLATIONS) {
    if (!patched.includes(en)) {
      console.warn(`[patch-codegen-i18n] WARNING: "${en}" not found — skipped`);
      continue;
    }
    patched = patched.replaceAll(en, ja);
  }

  // パッチ適用済みマーカーを先頭に追加
  patched = '/* __i18n_patched__ */\n' + patched;

  fs.writeFileSync(bundlePath, patched, 'utf8');
  console.log('[patch-codegen-i18n] Japanese tooltips applied to', path.relative(process.cwd(), bundlePath));
}

applyPatch();
