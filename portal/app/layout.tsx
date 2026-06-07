import type { Metadata } from 'next';
import './globals.css';
import { Sidebar } from './Sidebar';

export const metadata: Metadata = {
  title: 'e2e-tr',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body>
        <div className="app-shell">
          <Sidebar />
          <main className="app-main">
            <div className="app-content">{children}</div>
          </main>
        </div>
      </body>
    </html>
  );
}
