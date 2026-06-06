package domain

import (
	"net/url"
	"regexp"
	"strings"
)

// gotoRe は page.goto('http(s)://...') の絶対 URL リテラルを捉える。
// 引用符は ' でも " でもよい(codegen は ' を吐くが、手書き編集後の再保存にも備える)。
var gotoRe = regexp.MustCompile(`page\.goto\((['"])(https?://[^'"]+)['"]\)`)

const playwrightImport = "@playwright/test';"

// RelativizeFirstGoto は spec の最初の絶対 goto を相対パスへ書き換え、録画元の
// origin を baseURL の既定として埋め込む。これにより同一 spec を baseURL 上書き
// (PLAYWRIGHT_BASE_URL)で dev/prod/blue-green に振り分けられる。
//
//	await page.goto('https://app.example.com/cart?x=1');
//	  ↓
//	test.use({ baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'https://app.example.com' });
//	await page.goto('/cart?x=1');
//
// env を file 内の test.use で読むのが要点 ── test.use は config の use より優先
// されるため、env 未指定なら録画元 origin、指定時はその値が確実に勝つ。最初の
// goto(エントリポイント)のみ対象。絶対 goto が無ければ src をそのまま返す。
func RelativizeFirstGoto(src string) string {
	loc := gotoRe.FindStringSubmatchIndex(src)
	if loc == nil {
		return src
	}
	// loc[4]:loc[5] が URL 本体(submatch 2)。引用符は含まない。
	rawURL := src[loc[4]:loc[5]]
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return src
	}

	rel := u.RequestURI() // path + "?" + query。空パスでも "/" を返す。
	if u.Fragment != "" {
		rel += "#" + u.Fragment
	}
	origin := u.Scheme + "://" + u.Host

	// この goto の URL 部分だけ置換する(他の goto は触らない)。
	out := src[:loc[4]] + rel + src[loc[5]:]

	inject := "\ntest.use({ baseURL: process.env.PLAYWRIGHT_BASE_URL ?? '" + origin + "' });\n"
	if i := strings.Index(out, playwrightImport); i >= 0 {
		end := i + len(playwrightImport)
		out = out[:end] + "\n" + inject + out[end:]
	} else {
		out = inject + out
	}
	return out
}
