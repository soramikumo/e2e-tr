'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { TagModal, TagChip, TagDef, contrastText } from '../TagModal';
import { CodeEditorModal } from '../CodeEditorModal';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

type RunStatus = 'running' | 'done' | 'failed';

interface Scenario {
  name: string;
  modified: string;
  size: number;
  tags?: string[];
}

interface EnvView {
  id: string;
  name: string;
  baseURL: string;
  hasAuthPass: boolean;
}

// 実行履歴(GET /api/runs)のサマリ。ログ全文は含まず、表示時に stream で取得する。
interface RunSummary {
  id: string;
  tag?: string;
  file?: string;
  files?: string[];
  status: RunStatus;
  started_at: string;
  finished_at?: string;
}

interface LogLine {
  text: string;
  kind: 'default' | 'info' | 'error';
}

// 並列実行に対応するため、実行中/完了の run を個別に保持する。
interface RunState {
  id: string;
  label: string;
  status: RunStatus;
  logs: LogLine[];
}

function classifyLine(text: string): LogLine['kind'] {
  if (text.startsWith('[info]')) return 'info';
  if (text.startsWith('[error]') || text.startsWith('[stderr]')) return 'error';
  return 'default';
}

const badgeLabel: Record<RunStatus, string> = {
  running: '実行中...',
  done: '成功',
  failed: '失敗',
};

export default function Home() {
  const [tags, setTags] = useState<TagDef[]>([]);
  const [scenarios, setScenarios] = useState<Scenario[]>([]);
  const [runs, setRuns] = useState<RunState[]>([]);
  const [tagTrace, setTagTrace] = useState(false);
  const [scenarioTrace, setScenarioTrace] = useState<Record<string, boolean>>({});
  // タグ編集モーダルの対象シナリオ(null なら閉じている)。
  const [modalScenario, setModalScenario] = useState<string | null>(null);
  // ソース編集モーダルの対象シナリオ(null なら閉じている)。
  const [editCodeScenario, setEditCodeScenario] = useState<string | null>(null);
  // 実行先 Environment(dev/staging/prod)。null なら spec/config 既定の URL を使う。
  // ID で持つことで、env リスト側で baseURL や認証が更新されても次の実行で追従する。
  const [environments, setEnvironments] = useState<EnvView[]>([]);
  const [environmentId, setEnvironmentId] = useState<string>('');
  // 実行履歴(SQLite に永続化され再起動でも残る)。マウント時と run 完了時に取得する。
  const [history, setHistory] = useState<RunSummary[]>([]);
  // インライン名前編集の対象シナリオ名(null なら非編集)と入力中ドラフト。
  const [editingName, setEditingName] = useState<string | null>(null);
  const [draftName, setDraftName] = useState('');
  // Enter/Escape で編集を閉じると input がアンマウントされ、ブラウザが blur を
  // 発火して onBlur が再度走る。キー操作で処理済みのときは onBlur を 1 回だけ
  // 握り潰すためのフラグ（state だと非同期で間に合わないため ref を使う）。
  const renameHandledRef = useRef(false);

  const tagByName = (name: string): TagDef =>
    tags.find((t) => t.name === name) ?? { name, color: '#6e7781' };

  /**
   * シナリオ/タグ/環境のリストと、run 履歴を backend から取得して state にセットする。
   */
  const fetchData = useCallback(() => {
    fetch(`${API}/api/tags`)
      .then((r) => r.json())
      .then((d) => setTags(d.tags ?? []))
      .catch(() => {});
    fetch(`${API}/api/scenarios`)
      .then((r) => r.json())
      .then((d) => setScenarios(d.scenarios ?? []))
      .catch(() => {});
    fetch(`${API}/api/environments`)
      .then((r) => r.json())
      .then((d) => setEnvironments(d.environments ?? []))
      .catch(() => {});
    fetch(`${API}/api/runs`)
      .then((r) => r.json())
      .then((d) => setHistory(d.runs ?? []))
      .catch(() => {});
  }, []);

  useEffect(() => { fetchData(); }, [fetchData]);

  /**
   * 指定した run に SSE で受け取ったログ行を1行追加する。
   */
  const appendRunLog = (id: string, message: string) => {
    const logLine: LogLine = { text: message, kind: classifyLine(message) };

    // currentRuns は React が渡す「更新前の runs 配列」。対象 id の run だけを差し替える。
    setRuns((currentRuns) =>
      currentRuns.map((run) => {
        if (run.id !== id) return run;
        return { ...run, logs: [...run.logs, logLine] };
      }),
    );
  };

  /**
   * 指定した run の実行ステータスを更新する。
   */
  const updateRunStatus = (id: string, status: RunStatus) => {
    setRuns((currentRuns) =>
      currentRuns.map((run) => {
        if (run.id !== id) return run;
        return { ...run, status };
      }),
    );
  };

  /**
   * SSE 接続エラー時に、まだ実行中の run だけ失敗扱いにする。
   */
  const markRunFailedIfRunning = (id: string) => {
    setRuns((currentRuns) =>
      currentRuns.map((run) => {
        if (run.id !== id || run.status !== 'running') return run;
        return { ...run, status: 'failed' };
      }),
    );
  };

  /**
   * 実行開始と同時に、running ステータスで空ログの run を追加する。
   * SSE でログが届いたら同じ id の run にログを追加していく。
   */
  const addRunningRun = (id: string, label: string) => {
    setRuns((currentRuns) => [
      { id, label, status: 'running', logs: [] },
      ...currentRuns.filter((run) => run.label !== label),
    ]);
  };

  const startRun = async (label: string, body: { tag?: string; file?: string }, trace: boolean) => {
    let id: string;
    try {
      const res = await fetch(`${API}/api/run`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...body, trace, environmentId: environmentId || undefined }),
      });
      if (!res.ok) {
        const msg = (await res.text()).trim();
        setRuns((currentRuns) => [
          { id: `err-${Date.now()}`, label, status: 'failed', logs: [{ text: `[error] 起動失敗: ${msg}`, kind: 'error' }] },
          ...currentRuns,
        ]);
        return;
      }
      ({ id } = await res.json());
    } catch (e) {
      setRuns((currentRuns) => [
        { id: `err-${Date.now()}`, label, status: 'failed', logs: [{ text: `[error] 起動失敗: ${e}`, kind: 'error' }] },
        ...currentRuns,
      ]);
      return;
    }

    addRunningRun(id, label);

    const es = new EventSource(`${API}/api/stream?id=${id}`);
    es.onmessage = (e) => {
      const data = JSON.parse(e.data);
      if (data.type === 'log') {
        appendRunLog(id, data.message);
      } else if (data.type === 'done') {
        updateRunStatus(id, data.status === 'done' ? 'done' : 'failed');
        es.close();
        // 完了した run を履歴一覧へ反映する。
        fetch(`${API}/api/runs`).then((r) => r.json()).then((d) => setHistory(d.runs ?? [])).catch(() => {});
      }
    };
    es.onerror = () => {
      es.close();
      markRunFailedIfRunning(id);
    };
  };

  // ドラフトを正規化して .spec.ts を保証する（ユーザーが拡張子を省いても許容する）。
  const normalizeSpecName = (raw: string) => {
    const base = raw.trim().replace(/\.spec\.ts$/i, '');
    return base ? `${base}.spec.ts` : '';
  };

  const commitRename = async (oldName: string) => {
    const newName = normalizeSpecName(draftName);
    setEditingName(null);
    // 空、または変更なしなら API を呼ばずに編集を閉じる。
    if (!newName || newName === oldName) return;
    const res = await fetch(
      `${API}/api/scenarios?name=${encodeURIComponent(oldName)}&to=${encodeURIComponent(newName)}`,
      { method: 'PATCH' },
    );
    if (!res.ok) {
      const msg = (await res.text()).trim();
      alert(`名前変更に失敗しました: ${msg}`);
    }
    fetchData();
  };

  const deleteScenario = async (name: string) => {
    await fetch(`${API}/api/scenarios?name=${encodeURIComponent(name)}`, { method: 'DELETE' });
    fetchData();
  };

  // 同じ対象が実行中のあいだは、その実行ボタンだけ無効化して二重起動を防ぐ。
  const runningLabels = new Set(runs.filter((r) => r.status === 'running').map((r) => r.label));
  // ラベルごとの最新 run（runs は先頭が新しい）。
  const latestRun = (label: string) => runs.find((r) => r.label === label);

  // 履歴 run の表示ラベル（タグ実行は @tag、単体は file、タグ複数は連結）。
  const historyLabel = (r: RunSummary): string =>
    r.tag ? `@${r.tag}` : r.file || (r.files && r.files.join(', ')) || r.id;

  const fmtTime = (s?: string) => {
    if (!s) return '';
    const d = new Date(s);
    return isNaN(d.getTime()) ? '' : d.toLocaleString();
  };

  // 履歴一覧の「ログを見る」から、選択した run のログを画面に展開する。
  // すでに展開済みなら何もせず、未展開なら表示枠を追加して SSE でログを読み込む。
  const viewHistoryLogs = (r: RunSummary) => {
    if (runs.some((x) => x.id === r.id)) return; // 既に表示中なら二重購読しない。
    const label = historyLabel(r);
    setRuns((currentRuns) => [{ id: r.id, label, status: r.status, logs: [] }, ...currentRuns]);
    const es = new EventSource(`${API}/api/stream?id=${r.id}`);
    es.onmessage = (e) => {
      const data = JSON.parse(e.data);
      if (data.type === 'log') {
        appendRunLog(r.id, data.message);
      } else if (data.type === 'done') {
        updateRunStatus(r.id, data.status === 'done' ? 'done' : 'failed');
        es.close();
      }
    };
    es.onerror = () => es.close();
  };

  const renderRunCard = (run: RunState) => (
    <div className="run-card">
      <div className="output-header">
        <span className="tag-label">{run.label}</span>
        <span className={`badge ${run.status}`}>{badgeLabel[run.status]}</span>
      </div>
      <pre className="terminal">
        {run.logs.map((line, i) => (
          <span key={i} className={`log-line ${line.kind}`}>{line.text + '\n'}</span>
        ))}
      </pre>
      {run.status !== 'running' && !run.id.startsWith('err-') && (
        <a className="report-link" href={`${API}/report/${run.id}/`} target="_blank" rel="noopener noreferrer">
          HTMLレポートを開く →
        </a>
      )}
    </div>
  );

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="page-title">Test cases</h1>
          <p className="page-sub">記録済みシナリオを実行・管理する。</p>
        </div>
      </header>

      <div className="baseurl-bar">
        <label htmlFor="env-select">実行先環境</label>
        <select
          id="env-select"
          className="baseurl-input"
          value={environmentId}
          onChange={(e) => setEnvironmentId(e.target.value)}
        >
          <option value="">（録画時の環境を使う）</option>
          {environments.map((e) => (
            <option key={e.id} value={e.id}>
              {e.name} — {e.baseURL}{e.hasAuthPass ? ' 🔒' : ''}
            </option>
          ))}
        </select>
        <a className="env-manage-link" href="/environments">環境を管理 →</a>
      </div>

      {tags.length > 0 && (
        <section>
          <div className="output-header">
            <h2 style={{ margin: 0 }}>タグで実行</h2>
            <label className="trace-toggle">
              <input
                type="checkbox"
                className="switch"
                checked={tagTrace}
                onChange={(e) => setTagTrace(e.target.checked)}
                aria-label="トレースを保存"
              />
              トレースを保存
            </label>
          </div>
          <div className="tags">
            {tags.map((tag) => (
              <button
                key={tag.name}
                className={`tag-run-button${runningLabels.has(`@${tag.name}`) ? ' active' : ''}`}
                style={{ background: tag.color, color: contrastText(tag.color) }}
                onClick={() => startRun(`@${tag.name}`, { tag: tag.name }, tagTrace)}
                disabled={runningLabels.has(`@${tag.name}`)}
              >
                @{tag.name}
              </button>
            ))}
          </div>
          {tags.some((t) => latestRun(`@${t.name}`)) && (
            <div className="run-list">
              {tags
                .map((t) => latestRun(`@${t.name}`))
                .filter((r): r is RunState => !!r)
                .map((run) => (
                  <div key={run.id}>{renderRunCard(run)}</div>
                ))}
            </div>
          )}
        </section>
      )}

      <section>
        <h2>シナリオで実行</h2>
        {scenarios.length === 0 ? (
          <p className="empty">シナリオがありません。「シナリオ作成」から作成してください。</p>
        ) : (
          <div className="scenario-list">
            {scenarios.map((s) => {
              const run = latestRun(s.name);
              return (
                <div key={s.name} className="scenario-item">
                  <div className="scenario-row">
                    <div className="scenario-meta">
                      {editingName === s.name ? (
                        <input
                          className="scenario-name-input"
                          autoFocus
                          value={draftName}
                          onChange={(e) => setDraftName(e.target.value)}
                          onBlur={() => {
                            // キー操作で処理済みなら、アンマウント由来の blur は無視。
                            if (renameHandledRef.current) {
                              renameHandledRef.current = false;
                              return;
                            }
                            commitRename(s.name);
                          }}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                              renameHandledRef.current = true;
                              commitRename(s.name);
                            } else if (e.key === 'Escape') {
                              renameHandledRef.current = true;
                              setEditingName(null);
                            }
                          }}
                        />
                      ) : (
                        <span
                          className="scenario-name"
                          title="ダブルクリックで名前を変更"
                          onDoubleClick={() => {
                            renameHandledRef.current = false;
                            setEditingName(s.name);
                            setDraftName(s.name.replace(/\.spec\.ts$/i, ''));
                          }}
                        >
                          {s.name}
                        </span>
                      )}
                      {(s.tags ?? []).length > 0 && (
                        <div className="scenario-tags">
                          {(s.tags ?? []).map((name) => (
                            <TagChip key={name} tag={tagByName(name)} />
                          ))}
                        </div>
                      )}
                    </div>
                    <div className="scenario-actions">
                      <button className="tag-edit-btn" onClick={() => setModalScenario(s.name)}>
                        🏷 タグ
                      </button>
                      <button className="tag-edit-btn" onClick={() => setEditCodeScenario(s.name)}>
                        ✎ 編集
                      </button>
                      <label className="trace-toggle row-trace">
                        <input
                          type="checkbox"
                          className="switch"
                          checked={!!scenarioTrace[s.name]}
                          onChange={(e) => setScenarioTrace((prev) => ({ ...prev, [s.name]: e.target.checked }))}
                          aria-label={`トレース ${s.name}`}
                        />
                        トレース
                      </label>
                      <button
                        className="run-btn"
                        onClick={() => startRun(s.name, { file: s.name }, !!scenarioTrace[s.name])}
                        disabled={runningLabels.has(s.name)}
                      >
                        実行
                      </button>
                      <button
                        className="delete-btn"
                        onClick={() => deleteScenario(s.name)}
                        disabled={runningLabels.has(s.name)}
                      >
                        削除
                      </button>
                    </div>
                  </div>
                  {run && renderRunCard(run)}
                </div>
              );
            })}
          </div>
        )}
      </section>

      <section>
        <h2>実行履歴</h2>
        {history.length === 0 ? (
          <p className="empty">まだ実行履歴がありません。テストを実行すると履歴に残ります。</p>
        ) : (
          <div className="run-list">
            {history.map((r) => {
              const live = runs.find((x) => x.id === r.id);
              return (
                <div key={r.id} className="history-item">
                  <div className="scenario-row">
                    <div className="scenario-meta">
                      <span className="tag-label">{historyLabel(r)}</span>
                      <span className={`badge ${r.status}`}>{badgeLabel[r.status]}</span>
                      <span className="history-time">{fmtTime(r.started_at)}</span>
                    </div>
                    <div className="scenario-actions">
                      {!live && (
                        <button className="tag-edit-btn" onClick={() => viewHistoryLogs(r)}>
                          ログを見る
                        </button>
                      )}
                      {r.status !== 'running' && (
                        <a className="report-link" href={`${API}/report/${r.id}/`} target="_blank" rel="noopener noreferrer">
                          レポート →
                        </a>
                      )}
                    </div>
                  </div>
                  {live && renderRunCard(live)}
                </div>
              );
            })}
          </div>
        )}
      </section>

      {modalScenario && (
        <TagModal
          scenario={modalScenario}
          allTags={tags}
          assigned={scenarios.find((s) => s.name === modalScenario)?.tags ?? []}
          onClose={() => { setModalScenario(null); fetchData(); }}
        />
      )}

      {editCodeScenario && (
        <CodeEditorModal
          scenario={editCodeScenario}
          onClose={() => { setEditCodeScenario(null); fetchData(); }}
        />
      )}
    </>
  );
}
