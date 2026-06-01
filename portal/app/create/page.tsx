'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
const NOVNC_HOST = process.env.NEXT_PUBLIC_NOVNC_HOST ?? 'http://localhost';

function isValidUrl(val: string) {
  try {
    const u = new URL(val);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
}

type State = 'idle' | 'recording' | 'done' | 'error';

export default function CreatePage() {
  const [url, setUrl] = useState('');
  const [name, setName] = useState('');
  const [state, setState] = useState<State>('idle');
  const [message, setMessage] = useState('');
  const [savedFile, setSavedFile] = useState('');
  const [noVNCPort, setNoVNCPort] = useState<number | null>(null);
  const [codegenId, setCodegenId] = useState<string | null>(null);
  const [showCode, setShowCode] = useState(false);
  const [code, setCode] = useState('');
  const router = useRouter();

  const startRecording = async () => {
    if (!isValidUrl(url)) return;

    setState('recording');
    setMessage('ブラウザを起動しています...');
    setSavedFile('');
    setNoVNCPort(null);
    setCodegenId(null);
    setCode('');

    let id: string;
    try {
      const res = await fetch(`${API}/api/codegen/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url, name: name || undefined }),
      });
      const data = await res.json();
      id = data.id;
      setNoVNCPort(data.noVNCPort);
      setCodegenId(id);
    } catch (e) {
      setState('error');
      setMessage(`起動失敗: ${e}`);
      return;
    }

    const es = new EventSource(`${API}/api/codegen/stream?id=${id}`);
    es.onmessage = (e) => {
      const data = JSON.parse(e.data);
      if (data.type === 'status') {
        setMessage(data.message);
      } else if (data.type === 'done') {
        setState('done');
        setSavedFile(data.file);
        setMessage(`保存完了: ${data.file}`);
        es.close();
      } else if (data.type === 'error') {
        setState('error');
        setMessage(data.message);
        es.close();
      }
    };
    es.onerror = () => {
      es.close();
      setState((s) => (s === 'recording' ? 'error' : s));
    };
  };

  // コードパネルが開いている間、記録中はライブで spec をポーリング取得する。
  // codegen が --output を逐次書くため、操作するたびにコードが更新される。
  useEffect(() => {
    if (!codegenId || !showCode) return;
    if (state !== 'recording' && state !== 'done') return;

    let active = true;
    const fetchCode = async () => {
      try {
        const res = await fetch(`${API}/api/codegen/code?id=${codegenId}`);
        const data = await res.json();
        if (active) setCode(data.code ?? '');
      } catch {
        /* ポーリングは次回リトライするので握りつぶす */
      }
    };
    fetchCode();
    if (state !== 'recording') return;
    const timer = setInterval(fetchCode, 1500);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [codegenId, showCode, state]);

  const isRecording = state === 'recording';

  const stateToStatus = { idle: '', recording: 'running', done: 'done', error: 'failed' } as const;
  const stateLabel = { idle: '', recording: '記録中', done: '保存完了', error: 'エラー' };

  return (
    <>
      <h1 className="page-title">シナリオ作成</h1>

      <section>
        <h2>記録設定</h2>
        <div className="form">
          <label className="form-label">
            URL
            <input
              className="form-input"
              type="url"
              placeholder="https://example.com"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              disabled={isRecording}
            />
          </label>
          <label className="form-label">
            シナリオ名（省略可）
            <input
              className="form-input"
              type="text"
              placeholder="my-scenario"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={isRecording}
            />
          </label>
          <button
            className="record-btn"
            onClick={startRecording}
            disabled={!isValidUrl(url) || isRecording}
          >
            {isRecording ? '記録中...' : '記録開始'}
          </button>
        </div>
      </section>

      {state !== 'idle' && (
        <section>
          <h2>状態</h2>
          <div className="status-box">
            <span className={`badge ${stateToStatus[state]}`}>{stateLabel[state]}</span>
            <p className="status-message">{message}</p>
            {isRecording && (
              <p className="status-hint">
                ブラウザで操作を行ってください。完了したらブラウザを閉じると自動的に保存されます。
              </p>
            )}
            {isRecording && noVNCPort && (
              <iframe
                className="codegen-viewer"
                src={`${NOVNC_HOST}:${noVNCPort}/vnc.html?autoconnect=true&resize=scale`}
              />
            )}

            {(isRecording || state === 'done') && (
              <div className="codegen-code-bar">
                <button
                  className="tag-button"
                  onClick={() => setShowCode((v) => !v)}
                >
                  {showCode ? 'コードを隠す ▴' : 'コード表示 ▾'}
                </button>
              </div>
            )}
            {showCode && (isRecording || state === 'done') && (
              <pre className="codegen-code">
                {code || '// まだコードがありません。ブラウザで操作すると生成されます。'}
              </pre>
            )}

            {state === 'done' && savedFile && (
              <p className="status-hint">保存先: tests/tests/{savedFile}</p>
            )}
          </div>
          {state === 'done' && (
            <div className="done-actions">
              <button className="tag-button" onClick={() => router.push('/')}>
                テスト実行ページへ →
              </button>
            </div>
          )}
        </section>
      )}

      <section>
        <h2>使い方</h2>
        <ol className="how-to">
          <li>記録したいページの URL を入力する</li>
          <li>シナリオ名を入力する（省略するとランダムな名前になる）</li>
          <li>「記録開始」をクリックすると Chromium が起動する</li>
          <li>ブラウザで操作を行う（クリック・入力・遷移など）</li>
          <li>操作が終わったらブラウザを閉じる</li>
          <li>テストシナリオが自動保存され、実行ページから実行できる</li>
        </ol>
      </section>
    </>
  );
}
