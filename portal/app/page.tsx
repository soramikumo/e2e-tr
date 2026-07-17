'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { RunSummary, badgeLabel, historyLabel, fmtTime } from './runs';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

const TREND_DAYS = 30;   // 成功率トレンドの集計窓(直近成功率 KPI もこの窓で出す)。
const RECENT_LIMIT = 6;  // 「直近の実行」に並べる件数。

// Dashboard はプロジェクト全体の健康状態を一望する場所(Octomind の Project Health 相当)。
// run 履歴の永続化(#90/#97 で GET /api/runs が稼働)が揃ったので、直近成功率・直近の実行・
// 成功率トレンドを実データから描く。self-healing だけは引き続き Future。
export default function DashboardPage() {
  const [counts, setCounts] = useState<{ scenarios: number; tags: number; environments: number } | null>(null);
  const [runs, setRuns] = useState<RunSummary[] | null>(null);

  useEffect(() => {
    let alive = true;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const getJSON = async (path: string) => {
      const res = await fetch(`${API}${path}`);
      if (!res.ok) throw new Error(`HTTP ${res.status} ${path}`);
      return res.json();
    };

    // counts と runs は独立して読む。片方(特に /api/runs)が落ちても、成功した
    // 他方の表示を巻き込んで 0 / 空にしないため Promise.all を分ける。
    (async () => {
      try {
        const [s, t, e] = await Promise.all([
          getJSON('/api/scenarios'),
          getJSON('/api/tags'),
          getJSON('/api/environments'),
        ]);
        if (!alive) return;
        setCounts({
          scenarios: (s.scenarios ?? []).length,
          tags: (t.tags ?? []).length,
          environments: (e.environments ?? []).length,
        });
      } catch {
        if (alive) setCounts({ scenarios: 0, tags: 0, environments: 0 });
      }
    })();

    // 実行中 run があるあいだは追従ポーリングし、完了で自然に止まる。取得失敗時は
    // 既存表示を保持(初回のみ空)して、一時的なエラーで履歴を消さない。
    const loadRuns = async () => {
      try {
        const d = await getJSON('/api/runs');
        if (!alive) return;
        const list: RunSummary[] = d.runs ?? [];
        setRuns(list);
        if (list.some((r) => r.status === 'running')) {
          timer = setTimeout(loadRuns, 4000);
        }
      } catch {
        if (alive) setRuns((prev) => prev ?? []);
      }
    };
    loadRuns();

    return () => { alive = false; if (timer) clearTimeout(timer); };
  }, []);

  // 完了 run(成功/失敗)だけを成功率の母数にする。実行中は確定していないので除外。
  const trend = useMemo(() => computeTrend(runs ?? [], TREND_DAYS), [runs]);
  const recent = (runs ?? []).slice(0, RECENT_LIMIT);
  const successRate =
    trend.totalCompleted > 0 ? Math.round((trend.totalDone / trend.totalCompleted) * 100) : null;

  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="page-title">Dashboard</h1>
          <p className="page-sub">プロジェクトの全体像。実行 / 編集はサイドバーから。</p>
        </div>
        <Link href="/tests" className="record-btn">テストを実行 →</Link>
      </header>

      <div className="kpi-row">
        <KpiCard label="シナリオ"   value={counts?.scenarios ?? '…'}     hint="記録済みの spec ファイル数" loading={counts === null} />
        <KpiCard label="タグ"       value={counts?.tags ?? '…'}          hint="シナリオ束ね用のラベル"     loading={counts === null} />
        <KpiCard label="環境"       value={counts?.environments ?? '…'}  hint="dev / staging / prod"       loading={counts === null} />
        <KpiCard
          label="直近成功率"
          value={successRate === null ? '—' : `${successRate}%`}
          hint={trend.totalCompleted > 0 ? `直近${TREND_DAYS}日 · ${trend.totalCompleted}件` : `直近${TREND_DAYS}日に完了した実行なし`}
          loading={runs === null}
        />
      </div>

      <section className="dash-grid">
        <div className="dash-card">
          <div className="dash-card-head">
            <h2 className="dash-card-title">直近の実行</h2>
            <Link href="/tests" className="dash-card-link">すべて見る →</Link>
          </div>
          {runs === null ? (
            <div className="dash-skeleton">
              <SkeletonRow /><SkeletonRow /><SkeletonRow />
            </div>
          ) : recent.length === 0 ? (
            <p className="dash-empty">まだ実行履歴がありません。テストを実行すると、ここに並びます。</p>
          ) : (
            <div className="recent-list">
              {recent.map((r) => (
                <div key={r.id} className="recent-row">
                  <span className="recent-label" title={historyLabel(r)}>{historyLabel(r)}</span>
                  <span className={`badge ${r.status}`}>{badgeLabel[r.status]}</span>
                  <span className="recent-time">{fmtTime(r.started_at)}</span>
                  {r.status !== 'running' && (
                    <a className="report-link" href={`${API}/report/${r.id}/`} target="_blank" rel="noopener noreferrer">
                      レポート →
                    </a>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="dash-card">
          <div className="dash-card-head">
            <h2 className="dash-card-title">成功率トレンド ({TREND_DAYS}日)</h2>
          </div>
          {runs === null ? (
            <div className="dash-chart-skeleton" />
          ) : trend.totalCompleted === 0 ? (
            <p className="dash-empty">直近{TREND_DAYS}日に完了した実行はありません。実行を重ねると日次の成功率が描かれます。</p>
          ) : (
            <>
              <TrendChart days={trend.days} />
              <div className="trend-legend">
                <span><i className="dot ok" />80%+</span>
                <span><i className="dot warn" />50–79%</span>
                <span><i className="dot bad" />&lt;50%</span>
              </div>
            </>
          )}
        </div>

        <div className="dash-card">
          <div className="dash-card-head">
            <h2 className="dash-card-title">セルフ修復 (Self-healing)</h2>
            <span className="soon-pill">Future</span>
          </div>
          <p className="dash-card-body">
            失敗 run から spec の修正案を LLM に提案させ、採用/拒否を選べる仕組み。
            Octomind の本丸機能を OSS で。
          </p>
          <p className="dash-card-foot">
            <a href="https://github.com/soramikumo/e2e-tr/issues/83" target="_blank" rel="noopener noreferrer">Issue #83</a> で設計中。
          </p>
        </div>
      </section>
    </>
  );
}

interface DayBucket { key: string; label: string; done: number; failed: number; }
interface Trend { days: DayBucket[]; totalDone: number; totalCompleted: number; }

// ローカル日付の YYYY-M-D キー(UTC 変換を挟まず、ユーザーの暦日でバケットする)。
function dayKey(d: Date): string {
  return `${d.getFullYear()}-${d.getMonth() + 1}-${d.getDate()}`;
}

// 完了 run を直近 days 日ぶんの日次バケットへ集計する。窓より前の run は無視する。
function computeTrend(runs: RunSummary[], days: number): Trend {
  const buckets = new Map<string, DayBucket>();
  const today = new Date();
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(today);
    d.setDate(today.getDate() - i);
    buckets.set(dayKey(d), { key: dayKey(d), label: `${d.getMonth() + 1}/${d.getDate()}`, done: 0, failed: 0 });
  }
  for (const r of runs) {
    if (r.status !== 'done' && r.status !== 'failed') continue;
    const d = new Date(r.started_at);
    if (isNaN(d.getTime())) continue;
    const b = buckets.get(dayKey(d));
    if (!b) continue;
    if (r.status === 'done') b.done++; else b.failed++;
  }
  const list = [...buckets.values()];
  const totalDone = list.reduce((n, b) => n + b.done, 0);
  const totalCompleted = list.reduce((n, b) => n + b.done + b.failed, 0);
  return { days: list, totalDone, totalCompleted };
}

// 日次成功率の縦棒チャート(外部依存なし)。各日は full-height の薄いトラックで、
// 成功率ぶんだけ下から色付きバーが伸びる。flex + % 高さなので幅に応じて等分に
// 伸縮し、角丸や棒幅が歪まない(SVG の preserveAspectRatio 歪み問題を回避)。
function TrendChart({ days }: { days: DayBucket[] }) {
  return (
    <div className="trend-chart" role="img" aria-label={`直近${days.length}日の日次成功率`}>
      {days.map((d) => {
        const total = d.done + d.failed;
        const rate = total > 0 ? d.done / total : 0;
        const cls = total === 0 ? '' : rate >= 0.8 ? 'ok' : rate >= 0.5 ? 'warn' : 'bad';
        const pct = total > 0 ? Math.max(4, Math.round(rate * 100)) : 0;
        const title = total > 0 ? `${d.label} · 成功 ${d.done}/${total} (${Math.round(rate * 100)}%)` : `${d.label} · 実行なし`;
        return (
          <div key={d.key} className="trend-col" title={title}>
            {total > 0 && <div className={`trend-fill ${cls}`} style={{ height: `${pct}%` }} />}
          </div>
        );
      })}
    </div>
  );
}

function KpiCard({
  label, value, hint, loading,
}: { label: string; value: string | number; hint: string; loading: boolean }) {
  return (
    <div className="kpi-card">
      <div className="kpi-label">{label}</div>
      <div className="kpi-value">{loading ? '…' : value}</div>
      <div className="kpi-hint">{hint}</div>
    </div>
  );
}

function SkeletonRow() {
  return (
    <div className="skeleton-row">
      <div className="skeleton-bar w-30" />
      <div className="skeleton-bar w-60" />
      <div className="skeleton-bar w-20" />
    </div>
  );
}
