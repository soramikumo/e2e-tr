How to run locally

  ---
  ローカル起動手順

  1. tests セットアップ
  cd tests
  npm install
  npx playwright install chromium

  2. Go runner 起動
  cd runner
  go run .
  # → http://localhost:8080

  3. Next.js portal 起動
  cd portal
  npm install
  NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev
  # → http://localhost:3000

  ブラウザで localhost:3000 を開くと @search @cart ボタンが並んでいて、押すとリアルタイムでログが流れ、終わったら HTMLレポートへのリンクが出ます。