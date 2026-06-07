'use client';

import { useState, useEffect, useCallback } from 'react';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

interface EnvView {
  id: string;
  name: string;
  baseURL: string;
  basicAuthUser?: string;
  hasAuthPass: boolean;
  created_at: string;
  updated_at: string;
}

type EditDraft = {
  id?: string;          // 既存編集なら id あり、新規なら undefined
  name: string;
  baseURL: string;
  basicAuthUser: string;
  basicAuthPass: string;
  // 既存編集時、パスワード欄を「未変更」にするためのフラグ。
  // true なら save 時に password フィールドを送らない(サーバは現状維持)。
  passUnchanged: boolean;
};

const emptyDraft = (): EditDraft => ({
  name: '', baseURL: '', basicAuthUser: '', basicAuthPass: '', passUnchanged: true,
});

function isValidUrl(v: string) {
  try {
    const u = new URL(v);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
}

export default function EnvironmentsPage() {
  const [envs, setEnvs] = useState<EnvView[]>([]);
  const [draft, setDraft] = useState<EditDraft | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      const res = await fetch(`${API}/api/environments`);
      const data = await res.json();
      setEnvs(data.environments ?? []);
    } catch {
      /* 取得失敗は静かに無視(空のまま) */
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const openCreate = () => { setError(''); setDraft({ ...emptyDraft(), passUnchanged: false }); };
  const openEdit = (e: EnvView) => {
    setError('');
    setDraft({
      id: e.id,
      name: e.name,
      baseURL: e.baseURL,
      basicAuthUser: e.basicAuthUser ?? '',
      basicAuthPass: '',
      passUnchanged: true,
    });
  };

  const save = async () => {
    if (!draft) return;
    if (!isValidUrl(draft.baseURL)) {
      setError('baseURL は http/https の URL を指定してください');
      return;
    }
    setBusy(true);
    setError('');
    try {
      const body: Record<string, unknown> = {
        name: draft.name,
        baseURL: draft.baseURL,
        basicAuthUser: draft.basicAuthUser,
      };
      // 編集で「未変更」フラグが立ったままなら pass を送らない(サーバ側で現状維持)。
      // 新規 or 変更ありなら値(空文字含む)を送る ── 空文字でクリアもできる。
      if (!draft.passUnchanged) {
        body.basicAuthPass = draft.basicAuthPass;
      }
      const path = draft.id ? `/api/environments?id=${draft.id}` : '/api/environments';
      const method = draft.id ? 'PATCH' : 'POST';
      const res = await fetch(`${API}${path}`, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
      setDraft(null);
      load();
    } catch (e) {
      setError(`保存失敗: ${e}`);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (e: EnvView) => {
    if (!confirm(`環境 "${e.name}" を削除しますか？`)) return;
    await fetch(`${API}/api/environments?id=${e.id}`, { method: 'DELETE' });
    load();
  };

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="page-title">Environments</h1>
          <p className="page-sub">実行先(dev/staging/prod)を名前付きで保存。テスト実行画面から選択して切替。</p>
        </div>
        <button className="record-btn" onClick={openCreate}>＋ 新規追加</button>
      </header>

      {envs.length === 0 ? (
        <p className="empty">環境が登録されていません。「新規追加」から作成してください。</p>
      ) : (
        <div className="env-list">
          {envs.map((e) => (
            <div key={e.id} className="env-card">
              <div className="env-card-main">
                <div className="env-card-title">{e.name}</div>
                <div className="env-card-url">{e.baseURL}</div>
                <div className="env-card-meta">
                  {e.basicAuthUser ? (
                    <span className="env-tag">
                      🔒 Basic Auth: {e.basicAuthUser}{e.hasAuthPass ? ' / ••••' : ' / (パスワード未設定)'}
                    </span>
                  ) : (
                    <span className="env-tag muted">認証なし</span>
                  )}
                </div>
              </div>
              <div className="env-card-actions">
                <button className="tag-edit-btn" onClick={() => openEdit(e)}>編集</button>
                <button className="delete-btn" onClick={() => remove(e)}>削除</button>
              </div>
            </div>
          ))}
        </div>
      )}

      {draft && (
        <div className="tag-modal-overlay" onClick={() => !busy && setDraft(null)}>
          <div className="code-modal" onClick={(ev) => ev.stopPropagation()}>
            <div className="tag-modal-header">
              <span className="tag-modal-title">{draft.id ? '環境を編集' : '環境を追加'}</span>
            </div>

            <label className="form-label">
              名前 <span className="env-required">*</span>
              <input
                className="form-input"
                value={draft.name}
                onChange={(ev) => setDraft({ ...draft, name: ev.target.value })}
                placeholder="dev / staging / prod"
              />
            </label>

            <label className="form-label">
              baseURL <span className="env-required">*</span>
              <input
                className="form-input"
                type="url"
                value={draft.baseURL}
                onChange={(ev) => setDraft({ ...draft, baseURL: ev.target.value })}
                placeholder="https://staging.example.com"
              />
            </label>

            <label className="form-label">
              Basic Auth ユーザー（任意）
              <input
                className="form-input"
                value={draft.basicAuthUser}
                onChange={(ev) => setDraft({ ...draft, basicAuthUser: ev.target.value })}
              />
            </label>

            <label className="form-label">
              Basic Auth パスワード（任意）
              <input
                className="form-input"
                type="password"
                value={draft.passUnchanged ? '••••••••' : draft.basicAuthPass}
                onFocus={() => draft.passUnchanged && setDraft({ ...draft, passUnchanged: false, basicAuthPass: '' })}
                onChange={(ev) => setDraft({ ...draft, basicAuthPass: ev.target.value, passUnchanged: false })}
                placeholder={draft.id ? '変更しないなら触らない' : ''}
              />
            </label>

            {error && <p className="log-line error">{error}</p>}

            <div className="code-modal-actions">
              <button className="tag-modal-close" onClick={() => setDraft(null)} disabled={busy}>キャンセル</button>
              <button className="record-btn" onClick={save} disabled={busy || !draft.name || !draft.baseURL}>
                {busy ? '保存中...' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
