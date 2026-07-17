// run 実行履歴(GET /api/runs)の共有型とヘルパ。
// Dashboard(直近の実行・成功率トレンド)と Test cases(実行履歴)の両画面で使う。

export type RunStatus = 'running' | 'done' | 'failed';

// 実行履歴サマリ。ログ全文は含まず、必要時に /api/stream で取得する(backend と対応)。
export interface RunSummary {
  id: string;
  tag?: string;
  file?: string;
  files?: string[];
  status: RunStatus;
  started_at: string;
  finished_at?: string;
}

export const badgeLabel: Record<RunStatus, string> = {
  running: '実行中...',
  done: '成功',
  failed: '失敗',
};

// 履歴 run の表示ラベル(タグ実行は @tag、単体は file、タグ複数は連結)。
export function historyLabel(r: RunSummary): string {
  return r.tag ? `@${r.tag}` : r.file || (r.files && r.files.join(', ')) || r.id;
}

// ISO 文字列をローカル表記へ。空/不正値は空文字を返す。
export function fmtTime(s?: string): string {
  if (!s) return '';
  const d = new Date(s);
  return isNaN(d.getTime()) ? '' : d.toLocaleString();
}
