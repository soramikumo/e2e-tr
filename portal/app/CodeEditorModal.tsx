'use client';

import { useState, useEffect } from 'react';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

interface CodeEditorModalProps {
  scenario: string;    // 編集対象シナリオのファイル名(.spec.ts)
  onClose: () => void; // 閉じる(保存有無に関わらず親側で再取得する)
}

type Load = 'loading' | 'ready' | 'error';

// CodeEditorModal は保存済み spec のソースをその場で編集する。
// dev/prod の差異吸収やセレクタ修正など、録画し直さずに直したいケース向け。
export function CodeEditorModal({ scenario, onClose }: CodeEditorModalProps) {
  const [code, setCode] = useState('');
  const [load, setLoad] = useState<Load>('loading');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const res = await fetch(`${API}/api/scenarios/code?name=${encodeURIComponent(scenario)}`);
        if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
        const data = await res.json();
        if (active) {
          setCode(data.code ?? '');
          setLoad('ready');
        }
      } catch (e) {
        if (active) {
          setError(`読み込み失敗: ${e}`);
          setLoad('error');
        }
      }
    })();
    return () => {
      active = false;
    };
  }, [scenario]);

  const save = async () => {
    setBusy(true);
    setError('');
    try {
      const res = await fetch(`${API}/api/scenarios/code?name=${encodeURIComponent(scenario)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code }),
      });
      if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
      onClose();
    } catch (e) {
      setError(`保存失敗: ${e}`);
      setBusy(false);
    }
  };

  return (
    <div className="tag-modal-overlay" onClick={onClose}>
      <div className="code-modal" onClick={(e) => e.stopPropagation()}>
        <div className="tag-modal-header">
          <span className="tag-modal-title">ソース編集</span>
          <span className="tag-modal-sub">{scenario}</span>
        </div>

        {load === 'loading' && <p className="status-message">読み込み中...</p>}
        {load === 'error' && <p className="log-line error">{error}</p>}
        {load === 'ready' && (
          <textarea
            className="code-editor"
            value={code}
            spellCheck={false}
            onChange={(e) => setCode(e.target.value)}
          />
        )}

        {error && load === 'ready' && <p className="log-line error">{error}</p>}

        <div className="code-modal-actions">
          <button className="tag-modal-close" onClick={onClose} disabled={busy}>
            キャンセル
          </button>
          <button className="record-btn" onClick={save} disabled={busy || load !== 'ready'}>
            {busy ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}
