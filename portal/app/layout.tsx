import type { Metadata } from 'next';
import Link from 'next/link';
import './globals.css';

export const metadata: Metadata = {
  title: 'E2E Test Portal',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body>
        <nav className="nav">
          <span className="nav-brand">E2E Portal</span>
          <div className="nav-links">
            <Link href="/" className="nav-link">テスト実行</Link>
            <Link href="/create" className="nav-link">シナリオ作成</Link>
            <Link href="/environments" className="nav-link">環境</Link>
          </div>
        </nav>
        <main className="container">{children}</main>
      </body>
    </html>
  );
}
