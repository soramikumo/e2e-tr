'use client';

import { useState, useEffect, useCallback } from 'react';
import { TagModal, TagChip, TagDef, contrastText } from './TagModal';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

type RunStatus = 'running' | 'done' | 'failed';

interface Scenario {
  name: string;
  modified: string;
  size: number;
  tags?: string[];
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

  const tagByName = (name: string): TagDef =>
    tags.find((t) => t.name === name) ?? { name, color: '#6e7781' };

  const fetchData = useCallback(() => {
    fetch(`${API}/api/tags`)
      .then((r) => r.json())
      .then((d) => setTags(d.tags ?? []))
      .catch(() => {});
    fetch(`${API}/api/scenarios`)
      .then((r) => r.json())
      .then((d) => setScenarios(d.scenarios ?? []))
      .catch(() => {});
  }, []);

  useEffect(() => { fetchData(); }, [fetchData]);

  const patchRun = (id: string, fn: (r: RunState) => RunState) =>
    setRuns((prev) => prev.map((r) => (r.id === id ? fn(r) : r)));

  const startRun = async (label: string, body: { tag?: string; file?: string }, trace: boolean) => {
    let id: string;
    try {
      const res = await fetch(`${API}/api/run`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...body, trace }),
      });
      if (!res.ok) {
        const msg = (await res.text()).trim();
        setRuns((prev) => [
          { id: `err-${Date.now()}`, label, status: 'failed', logs: [{ text: `[error] 起動失敗: ${msg}`, kind: 'error' }] },
          ...prev,
        ]);
        return;
      }
      ({ id } = await res.json());
    } catch (e) {
      setRuns((prev) => [
        { id: `err-${Date.now()}`, label, status: 'failed', logs: [{ text: `[error] 起動失敗: ${e}`, kind: 'error' }] },
        ...prev,
      ]);
      return;
    }

    // 新しい run を先頭に追加（同一対象の古い run は置き換える）。
    setRuns((prev) => [{ id, label, status: 'running', logs: [] }, ...prev.filter((r) => r.label !== label)]);

    const es = new EventSource(`${API}/api/stream?id=${id}`);
    es.onmessage = (e) => {
      const data = JSON.parse(e.data);
      if (data.type === 'log') {
        patchRun(id, (r) => ({ ...r, logs: [...r.logs, { text: data.message, kind: classifyLine(data.message) }] }));
      } else if (data.type === 'done') {
        patchRun(id, (r) => ({ ...r, status: data.status === 'done' ? 'done' : 'failed' }));
        es.close();
      }
    };
    es.onerror = () => {
      es.close();
      patchRun(id, (r) => (r.status === 'running' ? { ...r, status: 'failed' } : r));
    };
  };

  const deleteScenario = async (name: string) => {
    await fetch(`${API}/api/scenarios?name=${encodeURIComponent(name)}`, { method: 'DELETE' });
    fetchData();
  };

  // 同じ対象が実行中のあいだは、その実行ボタンだけ無効化して二重起動を防ぐ。
  const runningLabels = new Set(runs.filter((r) => r.status === 'running').map((r) => r.label));
  // ラベルごとの最新 run（runs は先頭が新しい）。
  const latestRun = (label: string) => runs.find((r) => r.label === label);

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
        <a className="report-link" href={`${API}/report/`} target="_blank" rel="noopener noreferrer">
          HTMLレポートを開く →
        </a>
      )}
    </div>
  );

  return (
    <>
      <h1 className="page-title">テスト実行</h1>

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
                      <span className="scenario-name">{s.name}</span>
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

      {modalScenario && (
        <TagModal
          scenario={modalScenario}
          allTags={tags}
          assigned={scenarios.find((s) => s.name === modalScenario)?.tags ?? []}
          onClose={() => { setModalScenario(null); fetchData(); }}
        />
      )}
    </>
  );
}
