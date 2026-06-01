'use client';

import { useState } from 'react';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

export interface TagDef {
  name: string;
  color: string;
}

// GitHub のラベル既定色に近いパレット。
const PALETTE = [
  '#0e8a16', '#1d76db', '#5319e7', '#b60205',
  '#d93f0b', '#fbca04', '#0052cc', '#006b75',
  '#e99695', '#c5def5', '#bfdadc', '#d4c5f9',
];

// 背景色の明度から、読みやすい文字色(黒 or 白)を返す。
export function contrastText(hex: string): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex);
  if (!m) return '#fff';
  const n = parseInt(m[1], 16);
  const r = (n >> 16) & 255, g = (n >> 8) & 255, b = n & 255;
  // 相対輝度(簡易): 明るい背景なら黒文字。
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return luminance > 0.6 ? '#0f172a' : '#fff';
}

export function TagChip({ tag, onRemove }: { tag: TagDef; onRemove?: () => void }) {
  return (
    <span className="tag-chip" style={{ background: tag.color, color: contrastText(tag.color) }}>
      {tag.name}
      {onRemove && (
        <button className="tag-chip-x" onClick={onRemove} aria-label={`${tag.name} を外す`}>
          ×
        </button>
      )}
    </span>
  );
}

interface TagModalProps {
  scenario: string;          // タグを付ける対象シナリオのファイル名
  allTags: TagDef[];         // 定義済みタグ一覧
  assigned: string[];        // このシナリオに割当済みのタグ名
  onClose: () => void;       // 閉じる(親側で再取得する)
}

export function TagModal({ scenario, allTags, assigned, onClose }: TagModalProps) {
  const [tags, setTags] = useState<TagDef[]>(allTags);
  const [assignedSet, setAssignedSet] = useState<Set<string>>(new Set(assigned));
  const [newName, setNewName] = useState('');
  const [newColor, setNewColor] = useState(PALETTE[0]);
  const [busy, setBusy] = useState(false);

  // 割当を丸ごと PUT で置き換える。
  const persistAssignment = async (next: Set<string>) => {
    setAssignedSet(next);
    await fetch(`${API}/api/scenarios/tags`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ scenario, tags: [...next] }),
    });
  };

  const toggleAssign = (name: string) => {
    const next = new Set(assignedSet);
    next.has(name) ? next.delete(name) : next.add(name);
    persistAssignment(next);
  };

  const createTag = async () => {
    const name = newName.trim();
    if (!name || busy) return;
    setBusy(true);
    const res = await fetch(`${API}/api/tags`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, color: newColor }),
    });
    if (res.ok) {
      const data = await res.json();
      setTags(data.tags ?? []);
      setNewName('');
      // 作ったタグはそのままこのシナリオに割り当てる。
      persistAssignment(new Set(assignedSet).add(name));
    }
    setBusy(false);
  };

  const deleteTag = async (name: string) => {
    const res = await fetch(`${API}/api/tags?name=${encodeURIComponent(name)}`, { method: 'DELETE' });
    if (res.ok) {
      const data = await res.json();
      setTags(data.tags ?? []);
      const next = new Set(assignedSet);
      next.delete(name);
      setAssignedSet(next);
    }
  };

  return (
    <div className="tag-modal-overlay" onClick={onClose}>
      <div className="tag-modal" onClick={(e) => e.stopPropagation()}>
        <div className="tag-modal-header">
          <span className="tag-modal-title">タグ</span>
          <span className="tag-modal-sub">{scenario}</span>
        </div>

        <div className="tag-modal-list">
          {tags.length === 0 && <p className="empty">タグがありません。下で作成してください。</p>}
          {tags.map((tag) => (
            <label key={tag.name} className="tag-modal-row">
              <input
                type="checkbox"
                checked={assignedSet.has(tag.name)}
                onChange={() => toggleAssign(tag.name)}
              />
              <span className="tag-dot" style={{ background: tag.color }} />
              <span className="tag-modal-name">{tag.name}</span>
              <button
                className="tag-modal-del"
                onClick={(e) => { e.preventDefault(); deleteTag(tag.name); }}
                aria-label={`${tag.name} を削除`}
              >
                削除
              </button>
            </label>
          ))}
        </div>

        <div className="tag-create">
          <span className="tag-create-title">新規作成</span>
          <input
            className="form-input"
            type="text"
            placeholder="タグ名"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && createTag()}
          />
          <div className="color-swatches">
            {PALETTE.map((c) => (
              <button
                key={c}
                className={`swatch${newColor === c ? ' selected' : ''}`}
                style={{ background: c }}
                onClick={() => setNewColor(c)}
                aria-label={`色 ${c}`}
              />
            ))}
          </div>
          <div className="tag-create-preview">
            プレビュー: <TagChip tag={{ name: newName.trim() || 'タグ名', color: newColor }} />
          </div>
          <button className="record-btn" onClick={createTag} disabled={!newName.trim() || busy}>
            作成
          </button>
        </div>

        <button className="tag-modal-close" onClick={onClose}>閉じる</button>
      </div>
    </div>
  );
}
