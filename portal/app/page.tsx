'use client';

import { useState, useRef, useEffect, useCallback } from 'react';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

type Status = 'idle' | 'running' | 'done' | 'failed';

interface Scenario {
  name: string;
  modified: string;
  size: number;
}

interface LogLine {
  text: string;
  kind: 'default' | 'info' | 'error';
}

function classifyLine(text: string): LogLine['kind'] {
  if (text.startsWith('[info]')) return 'info';
  if (text.startsWith('[error]') || text.startsWith('[stderr]')) return 'error';
  return 'default';
}

export default function Home() {
  const [tags, setTags] = useState<string[]>([]);
  const [scenarios, setScenarios] = useState<Scenario[]>([]);
  const [status, setStatus] = useState<Status>('idle');
  const [activeLabel, setActiveLabel] = useState<string | null>(null);
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [runId, setRunId] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

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

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  const startRun = async (label: string, body: { tag?: string; file?: string }) => {
    setActiveLabel(label);
    setLogs([]);
    setStatus('running');
    setRunId(null);

    let id: string;
    try {
      const res = await fetch(`${API}/api/run`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      ({ id } = await res.json());
    } catch (e) {
      setLogs([{ text: `[error] 起動失敗: ${e}`, kind: 'error' }]);
      setStatus('failed');
      return;
    }

    setRunId(id);

    const es = new EventSource(`${API}/api/stream?id=${id}`);
    es.onmessage = (e) => {
      const data = JSON.parse(e.data);
      if (data.type === 'log') {
        setLogs((prev) => [...prev, { text: data.message, kind: classifyLine(data.message) }]);
      } else if (data.type === 'done') {
        setStatus(data.status === 'done' ? 'done' : 'failed');
        es.close();
      }
    };
    es.onerror = () => {
      es.close();
      setStatus((s) => (s === 'running' ? 'failed' : s));
    };
  };

  const deleteScenario = async (name: string) => {
    await fetch(`${API}/api/scenarios?name=${encodeURIComponent(name)}`, { method: 'DELETE' });
    fetchData();
  };

  const badgeLabel: Record<Status, string> = {
    idle: '', running: '実行中...', done: '成功', failed: '失敗',
  };

  return (
    <>
      <h1 className="page-title">テスト実行</h1>

      {tags.length > 0 && (
        <section>
          <h2>タグで実行</h2>
          <div className="tags">
            {tags.map((tag) => (
              <button
                key={tag}
                className={`tag-button${activeLabel === `@${tag}` && status === 'running' ? ' active' : ''}`}
                onClick={() => startRun(`@${tag}`, { tag })}
                disabled={status === 'running'}
              >
                @{tag}
              </button>
            ))}
          </div>
        </section>
      )}

      <section>
        <h2>シナリオで実行</h2>
        {scenarios.length === 0 ? (
          <p className="empty">シナリオがありません。「シナリオ作成」から作成してください。</p>
        ) : (
          <div className="scenario-list">
            {scenarios.map((s) => (
              <div key={s.name} className="scenario-row">
                <span className="scenario-name">{s.name}</span>
                <div className="scenario-actions">
                  <button
                    className="run-btn"
                    onClick={() => startRun(s.name, { file: s.name })}
                    disabled={status === 'running'}
                  >
                    実行
                  </button>
                  <button
                    className="delete-btn"
                    onClick={() => deleteScenario(s.name)}
                    disabled={status === 'running'}
                  >
                    削除
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {(logs.length > 0 || status !== 'idle') && (
        <section>
          <div className="output-header">
            <div className="output-title">
              <h2 style={{ margin: 0 }}>実行ログ</h2>
              {activeLabel && <span className="tag-label">{activeLabel}</span>}
            </div>
            {status !== 'idle' && (
              <span className={`badge ${status}`}>{badgeLabel[status]}</span>
            )}
          </div>
          <pre className="terminal">
            {logs.map((line, i) => (
              <span key={i} className={`log-line ${line.kind}`}>{line.text + '\n'}</span>
            ))}
            <div ref={bottomRef} />
          </pre>
          {status !== 'running' && runId && (
            <a
              className="report-link"
              href={`${API}/report/`}
              target="_blank"
              rel="noopener noreferrer"
            >
              HTMLレポートを開く →
            </a>
          )}
        </section>
      )}
    </>
  );
}
