// 認証(OIDC は cloud 側で実装済) / プロジェクト切替 / API トークン管理 などを置く予定。
// OSS 単一ユーザー版では設定対象が無いので、現状は当面のスケルトン。
export default function SettingsPage() {
  return (
    <>
      <header className="page-head">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="page-sub">ワークスペース設定。</p>
        </div>
      </header>

      <div className="soon-panel">
        <span className="soon-pill">Coming soon</span>
        <h2 className="soon-title">OSS 版では設定項目はまだありません</h2>
        <p className="soon-body">
          このページは将来的に Web 公開版 (cloud) で
          認証 / プロジェクト切替 / API トークン管理 などを置く場所です。
        </p>
        <p className="soon-body">
          OSS 範囲の調整は <code>runner/config/config.go</code> の環境変数や
          <code>docker-compose.yml</code> で行ってください。
        </p>
      </div>
    </>
  );
}
