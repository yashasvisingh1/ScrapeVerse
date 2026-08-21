# Bright Data Scraper Studio — Working Notes

Learnings accumulated from building and debugging a multi-retailer (Amazon/Myntra/Ajio) product scraper in Scraper Studio's IDE.

## Core structure: two separate, isolated code contexts

A Scraper Studio scraper is **not one script**. It's two independently executed blocks that don't share scope:

| Code type | Runs in | Has access to | Does not have access to |
|---|---|---|---|
| **Interaction code** | Browser/HTTP driver context | `navigate`, `wait*`, `click`, `collect`, `parse()`, `input`, `job`, `location.href` | `$` (Cheerio), the DOM |
| **Parser code** | Cheerio (jQuery-like) context | `$`, `input`, `location.href`, `parser` (tagged data) | `navigate`, `wait`, `click`, or any browser-control function |

**Implication:** a function defined in interaction code (e.g. `parseAmazonProduct`) cannot be called from parser code or vice versa. There is no `window[fnName]()` bridge across the two — the only handoff points are:
- `parse()` — called from interaction code, runs the parser script, returns its result
- `collect(data)` — called from interaction code, appends one record to the output dataset
- `tag_response()` / `tag_script()` / etc. — capture browser-side data and expose it to parser code via the `parser` global

If you're writing one script and trying to split retailer-specific logic across both files: **the parser script must self-detect context** (e.g. branch on `location.href`) since interaction code can't tell it which parser function to run.

## The `location` global, not `window`

Both contexts expose `location.href` for the current page URL. There is **no `window` object** — `window.location.href` will not work; use `location.href` directly. Likewise, don't reach for `window[...]` to dynamically dispatch functions across contexts — it doesn't exist there.

## Sync vs. async — the confusing part

Bright Data's own documentation and examples show **all interaction functions called synchronous-style, no `await`**, even though they block on real browser/network I/O internally:

```javascript
function main() {
  navigate(input.url);
  wait('#selector');
  return { ok: true };
}
return main();
```

However, in practice during this session:
- A plain `function main() { navigate(...); wait(...); }` sometimes threw:
  `Crawler error: async code is not allowed in sync functions`
- Wrapping the same navigate/wait calls in `async function main() { await navigate(...); await wait(...); }` sometimes **fixed** it.
- But when `navigate()`/`wait()` calls were moved into a **separate synchronous helper function** (even while `main()` itself was async), the error came back — because the async wrapper doesn't propagate into non-async helper functions. Every function in the call chain that reaches `navigate`/`wait`/`wait_any`/etc. needs to itself be `async` and use `await` on those calls, all the way up to `main()`.
- Conversely, when the **parser** script's `main()` was made `async`, `parse()` returned a Promise instead of the actual data, causing `products is not iterable` in the calling interaction code. Parser code should stay **plain synchronous** — nothing in Cheerio-based extraction needs to await anything.

**Working rule of thumb for this environment:**
- Interaction code: make `main()` **and every helper function that directly or indirectly calls `navigate`/`wait*`/`click`/etc.** `async`, and `await` those specific calls. Functions doing pure JS logic (validation, anomaly detection, string parsing) don't need to be async.
- Parser code: keep everything synchronous. Never use `async function main()` here — `parse()` on the interaction side expects the raw return value, not a Promise.

This is not confirmed to be officially documented behavior — it's inferred from trial and error in this session. Treat it as a working hypothesis, not gospel, if it recurs unexpectedly elsewhere.

## Errors seen and what they meant

| Error | Actual cause |
|---|---|
| `Invalid crawl code: unknown: Unexpected token, expected ","` | A literal JS syntax error — in our case, unescaped nested double quotes inside a double-quoted string (`"...[data-asin=""]..."`). Read the line/column pointer in the error; it's usually exactly right. |
| `async code is not allowed in sync functions` | Some function in the call chain used `await`/was `async` while an ancestor caller wasn't (or vice versa — see above). Also worth ruling out: stale/cached editor content not matching what's actually being sent (see "Debugging technique" below). |
| `products is not iterable` (or similar `TypeError`) | `parse()` returned something other than the expected array/object — most often because the parser's `main()` was `async`, making `parse()` return a Promise. |
| Preview shows `Total page loads: 0` and ends immediately, no crawler error surfaced | The interaction code threw an exception (e.g. `TypeError` from calling `.includes()` on an `undefined` URL) that was **caught by an outer `try/catch` in the script itself**, so it never became a visible crawler error — it just silently returned `{ success: false, ... }`. Root cause here was `input.url` being empty because no input was set for that particular preview run. Lesson: if a preview ends with zero activity and no error, suspect your own try/catch swallowing something, and temporarily add a `console.log` before the earliest risky line. |

## Debugging technique: verify what's actually running, not what the editor shows

When errors don't match what the pasted code should produce, the fastest way to confirm whether it's an *editor/cache* problem versus a genuine *runtime* problem is:
1. Open browser DevTools → Network tab in the Studio IDE itself.
2. Trigger "Start preview."
3. Find the request that fires the preview run (a POST carrying the interaction/parser code as a payload).
4. Inspect the actual code string being sent, not what's visible in the editor pane.

This confirms definitively whether stale/cached content is the issue before spending more cycles changing code. (Note: this payload can be encrypted/opaque in some setups, in which case this technique isn't available and you're limited to indirect evidence — like whether the same error reproduces identically across structurally different scripts, which points away from a code-level cause and toward a session/infrastructure-level one.)

## Function reference highlights (from Bright Data's docs)

**Interaction code globals:** `input`, `job`, `location` (`.href` only), `parser`

**Key interaction functions:**
- `navigate(url, opts)` — `opts`: `wait_until`, `timeout`, `referer`, `allow_status`, `fingerprint`, `sniff_mime`. **No `userAgent` option** — don't invent one; fingerprinting/anti-detection is handled at the proxy layer.
- `wait(selector, opts)`, `wait_any([selectors], opts)`, `wait_visible`, `wait_hidden`, `wait_for_text`, `wait_network_idle`, `wait_page_idle` — all **Browser worker only**.
- `parse()` — runs parser code, returns its result.
- `collect(data, validateFn?)` — appends one record to the output dataset.
- `el_exists(selector, timeout?)`, `el_is_visible(selector, timeout?)` — safe existence/visibility checks, don't throw if absent.
- `bad_input(message?)` — marks the input as invalid, prevents retries.
- `blocked(message?)` / `dead_page(message?)` — mark specific failure types.
- `next_stage(input)`, `run_stage(n, input)`, `rerun_stage(input)` — multi-stage orchestration.
- `country(code)` — route through a specific country's proxy.
- `hash(data, algorithm?)` — sha256/sha1/sha512/md5 digest, useful for dedup keys.

**Worker types:** Code worker (fast, no browser) vs Browser worker (full browser, required for `wait*`, `click`, `type`, `scroll_to`, `tag_*`, `solve_captcha`, and most interactive functions — these throw `not_supported_in_code_worker` if called on a Code worker). Default to Code worker unless you need browser-rendered content or interaction.

**Parser code globals:** `$` (Cheerio, jQuery-like), `input`, `location.href`, `parser` (tagged data from interaction code)

**Parser-specific Cheerio helpers:**
- `$(sel).text_sane()` — collapses/trims whitespace (better than `.text()` for messy markup).
- `$(sel).filter_includes(text)` — filter elements by substring match, chainable.

**Value constructors** (both contexts): `new Image(src)`, `new Video(src)`, `new Pdf(src)`, `new Doc(src)`, `new Money(value, currency)` — for structured output fields; support `validate_type: true` to verify downloaded content matches expected type.

## Practical process notes

- **Preview runs need an explicit input.** If the Studio preview panel has an input box (e.g. `{"url": "..."}`) and it's empty, `input.url` is `undefined` — code should defensively fall back (`input && input.url ? input.url : "default"`) or call `bad_input()` early with a clear message, rather than let it fail obscurely deeper in the call stack.
- **For iterative debugging, hardcoding test URLs directly in `main()`** (in a loop, with a fallback array) sidesteps the input-panel dependency entirely and is faster for multi-retailer testing than repeatedly setting the input JSON.
- **Don't assume all target retailers actually carry the product you're testing.** (E.g. Ajio's Apple catalog turned out to have no iPad listings — only iPhones/accessories/wearables. Verify with a real search before writing retailer-specific selectors around an assumed product.)
- **Selectors copied into a multi-retailer template without verification against live DOM are a guess, not a fact.** Only trust selectors that came from a confirmed-working single-site template (in this case, the Amazon extraction logic, which used multiple documented fallback selectors per field). Flag unverified selectors explicitly so a `null` result is diagnosed as "selector probably wrong" rather than "site blocked us."
- **In-memory JS state (module-level variables like a `scraperState` object) does not persist across separate scraper runs.** Health tracking / rolling price history / anomaly baselines built this way reset to empty on every run. If cross-run state matters, it needs to live outside the script (external dataset, webhook, database) — not as a JS variable.
