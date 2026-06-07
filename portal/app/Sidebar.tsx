'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

// Octomind 風のアプリ的レイアウト: 固定左サイドバー。
// 上から「観察 → 編集 → 設定」の順で並べる(Octomind の navigation 順を踏襲)。
interface Item {
  href: string;
  label: string;
  icon: string;
  badge?: string; // "Soon" 等の補助バッジ
}

const items: Item[] = [
  { href: '/',             label: 'Dashboard',    icon: '📊' },
  { href: '/tests',        label: 'Test cases',   icon: '🧪' },
  { href: '/create',       label: 'Record',       icon: '⏺' },
  { href: '/environments', label: 'Environments', icon: '🌐' },
  { href: '/reports',      label: 'Reports',      icon: '📑', badge: 'Soon' },
  { href: '/settings',     label: 'Settings',     icon: '⚙',  badge: 'Soon' },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="sidebar">
      <Link href="/" className="sidebar-brand">
        <span className="sidebar-logo">⚡</span>
        <span className="sidebar-brand-name">e2e-tr</span>
      </Link>

      <nav className="sidebar-nav">
        {items.map((it) => {
          // ルート / は完全一致、その他は前方一致でアクティブ判定。
          // (例: /environments の中で /environments/foo を作っても親リンクが光る)
          const active = it.href === '/' ? pathname === '/' : pathname.startsWith(it.href);
          return (
            <Link
              key={it.href}
              href={it.href}
              className={`sidebar-link${active ? ' active' : ''}`}
            >
              <span className="sidebar-icon">{it.icon}</span>
              <span className="sidebar-label">{it.label}</span>
              {it.badge && <span className="sidebar-badge">{it.badge}</span>}
            </Link>
          );
        })}
      </nav>

      <div className="sidebar-foot">
        <a
          className="sidebar-foot-link"
          href="https://github.com/soramikumo/e2e-tr"
          target="_blank"
          rel="noopener noreferrer"
        >
          GitHub ↗
        </a>
      </div>
    </aside>
  );
}
