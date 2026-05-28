import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const BUNDLE = path.resolve(
  __dirname,
  '../node_modules/playwright-core/lib/vite/recorder/assets'
);

function readBundle(): string {
  const file = fs.readdirSync(BUNDLE).find(f => f.startsWith('index-') && f.endsWith('.js'));
  if (!file) throw new Error('recorder bundle not found');
  return fs.readFileSync(path.join(BUNDLE, file), 'utf8');
}

test.describe('codegen ツールチップ日本語化', () => {
  let bundle: string;
  test.beforeAll(() => { bundle = readBundle(); });

  const EXPECTED: [string, string][] = [
    ['ソース選択（タブ・ページ切替）',    'Source chooser'],
    ['要素を選択（ロケーター取得）',      'Pick locator'],
    ['表示確認のアサーションを追加',      'Assert visibility'],
    ['テキスト確認のアサーションを追加',  'Assert text'],
    ['値確認のアサーションを追加',        'Assert value'],
    ['スナップショットのアサーションを追加', 'Assert snapshot'],
    ['クリア（コードを消去）',            'Clear'],
    ['設定',                              'Settings'],
    ['録画を停止',                        'Stop Recording'],
    ['録画を開始',                        'Start Recording'],
  ];

  for (const [ja, en] of EXPECTED) {
    test(`"${en}" → "${ja}" に置換されている`, () => {
      expect(bundle).toContain(`"${ja}"`);
      // title属性として使われている形式で残っていないことを確認
      expect(bundle).not.toContain(`title:"${en}"`);
    });
  }
});
