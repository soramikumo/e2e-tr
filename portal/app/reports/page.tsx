// 過去 run の永続化 (Issue #82) が入ってから本実装する。
// それまでは導線だけ確保し、何を作る場所かを宣言しておく。
export default function ReportsPage() {
  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="page-title">Reports</h1>
          <p className="page-sub">過去 run の一覧・詳細（trace / screenshot / 失敗理由）。</p>
        </div>
      </header>

      <div className="soon-panel">
        <span className="soon-pill">Coming soon</span>
        <h2 className="soon-title">Run 結果の永続化を待っています</h2>
        <p className="soon-body">
          現状の run は<code>MemoryRunStore</code>で扱っており、プロセス再起動で消えます。
          <code>playwright-report/</code> も最新 1 件で上書き。Reports 画面は
          <strong>run ごとの履歴保存</strong>が前提なので、先に backend の永続化を済ませてから繋ぎます。
        </p>
        <p className="soon-body">
          設計と進捗は{' '}
          <a href="https://github.com/soramikumo/e2e-tr/issues/82" target="_blank" rel="noopener noreferrer">
            Issue #82 「Run レポートの永続化と履歴ダッシュボード」
          </a>{' '}
          を参照。
        </p>
      </div>
    </>
  );
}
