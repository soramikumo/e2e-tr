'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';

const API = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

// Dashboard はプロジェクト全体の健康状態を一望する場所(Octomind の Project Health 相当)。
// 永続化された run 履歴が必要な指標(成功率トレンド・最近の run・失敗の傾向)は
// Issue #82 (Run レポートの永続化) の到着待ち。それまでは Coming soon スケルトンで
// 場所だけ確保し、現在 backend から取れる軽い件数(シナリオ/タグ/環境)だけ生で出す。
export default function DashboardPage() {
  const [counts, setCounts] = useState<{ scenarios: number; tags: number; environments: number } | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const [s, t, e] = await Promise.all([
          fetch(`${API}/api/scenarios`).then((r) => r.json()),
          fetch(`${API}/api/tags`).then((r) => r.json()),
          fetch(`${API}/api/environments`).then((r) => r.json()),
        ]);
        if (!alive) return;
        setCounts({
          scenarios: (s.scenarios ?? []).length,
          tags: (t.tags ?? []).length,
          environments: (e.environments ?? []).length,
        });
      } catch {
        if (alive) setCounts({ scenarios: 0, tags: 0, environments: 0 });
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => { alive = false; };
  }, []);

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
        <KpiCard label="シナリオ"    value={counts?.scenarios ?? '…'} hint="記録済みの spec ファイル数" loading={loading} />
        <KpiCard label="タグ"       value={counts?.tags ?? '…'}       hint="シナリオ束ね用のラベル"     loading={loading} />
        <KpiCard label="環境"       value={counts?.environments ?? '…'} hint="dev / staging / prod"       loading={loading} />
        <KpiCard label="直近成功率"  value="—"                          hint="run 履歴の永続化が必要"     loading={false} comingSoon />
      </div>

      <section className="dash-grid">
        <div className="dash-card">
          <div className="dash-card-head">
            <h2 className="dash-card-title">直近の実行</h2>
            <span className="soon-pill">Coming soon</span>
          </div>
          <div className="dash-skeleton">
            <SkeletonRow />
            <SkeletonRow />
            <SkeletonRow />
          </div>
          <p className="dash-card-foot">
            run 結果を永続化する <a href="https://github.com/soramikumo/e2e-tr/issues/82" target="_blank" rel="noopener noreferrer">Issue #82</a> 完了後にここへ繋ぐ。
          </p>
        </div>

        <div className="dash-card">
          <div className="dash-card-head">
            <h2 className="dash-card-title">成功率トレンド (30日)</h2>
            <span className="soon-pill">Coming soon</span>
          </div>
          <div className="dash-chart-skeleton" />
          <p className="dash-card-foot">同じく <a href="https://github.com/soramikumo/e2e-tr/issues/82" target="_blank" rel="noopener noreferrer">Issue #82</a> の永続化後に描画。</p>
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

function KpiCard({
  label, value, hint, loading, comingSoon,
}: { label: string; value: string | number; hint: string; loading: boolean; comingSoon?: boolean }) {
  return (
    <div className={`kpi-card${comingSoon ? ' kpi-soon' : ''}`}>
      <div className="kpi-label">{label}</div>
      <div className="kpi-value">{loading ? '…' : value}</div>
      <div className="kpi-hint">{hint}</div>
      {comingSoon && <span className="kpi-soon-pill">Soon</span>}
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
