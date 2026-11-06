# UI and visual design: working knowledge

Researched 2026-10-08 from 18 sites the owner picked. First pass: four research
agents reading pages as text (sections 1-5). Second pass, after the owner said
the result missed the point: about 110 screenshots taken in a headless browser
and looked at (section 0). Written for an agent that has to design or restyle
an interface, with Vega's dark HUD as the running case.

Marked **[S]** = taken from a source page; **[P]** = general practice, not from
these sites.

Follow-up: [motion and interaction study](ui-motion-study-2026-10-08.md) records
a second source audit, the current corpus direction, and which animation claims
were verified from code rather than observed live. The owner permits native UI
abilities and has discarded the spinning brain. Apply the newer direction above
the historical globe recipes below.

Latest feedback (2026-10-08): the owner rejected the radial atlas even after
moving dashes were added. The reset uses DdHC7W1OTqX's rising plume and
DbjD39NoSqP's sweeping trajectories: the entire filament field deforms, claim
particles travel, pointer influence bends nearby geometry, and topic selection
recomposes a current. A compact right-hand index and an overlaid left-hand
headline frame the dominant drawing. This version is awaiting owner feedback;
do not describe the atlas or the new field as an accepted design.

## 0. What the sites actually look like (read this first)

The text pass produced a tidy, quiet developer dashboard: small type, muted
colour, rounded boxes, a blurry glowing network. The owner rejected it. Looking
at the sites shows why: almost none of them look like that. What they share:

1. **Type is the main graphic.** Headlines are huge and tight (48-120 px, heavy
   or condensed, negative tracking): Aceternity, Motion's "Examples", Lenis'
   condensed display face, Mint's "Five casters. One arsenal.". The Instagram
   account sets a condensed heavy headline with one word in the accent colour. A
   page where everything is 11-14 px has no hierarchy and reads as a form.
2. **One loud colour, used flat and big.** Motion fills half the screen with
   yellow or blue; Mint a lime block; Lenis pink; Anime.js red; Unicorn violet;
   Relume magenta. The colour is a block, a button, a chip, a highlighted word.
   It is not a faint glow on a grey page.
3. **Crisp beats blurry.** Anime.js's hero is an instrument dial: a thick ring
   in coloured segments, hundreds of fine tick marks, concentric layers, dotted
   curves, hatched fills, flat shading, sharp edges. It is intricate because of
   precise small parts, not because of bloom. Glow appears as one accent (the
   Aceternity lamp line), never as the whole look.
4. **Editorial HUD composition.** Mint's product scenes (Ionbreaker, Fieldline,
   Platform Roster) put one 3D object in the middle, bleeding across the frame,
   with type-driven columns over it: a big headline, a numbered list ("01
   IONBREAKER"), mono micro-labels, stat bars with numbers, flat accent chips,
   corner brackets. Panels are separated by hairlines and space, not drawn as
   rounded cards side by side.
5. **Sharp geometry.** Motion uses zero radius everywhere, bordered cells, mono
   uppercase buttons with an arrow, labels like "FILTER://". Rounded 8 px cards
   are the default look of a template.
6. **Texture.** Heavy grain (ShaderGradient), dither, halftone and pixel
   matrices (Unicorn's gallery), dotted globes (Aceternity), starfields (Lenis,
   Threlte). Flat dark with no texture looks unfinished.
7. **Data as a poster** (the Instagram account, seen only dimmed behind the
   login wall): bar charts with the value set large on each bar, a mono caption
   strip on top, two colours.

Not seen: Uiverse (blocked by Cloudflare), and the Instagram profile beyond the
first row (login wall; never log in on the owner's behalf). Single posts open
without login at `instagram.com/p/<id>/embed/`.

### The look the owner actually wants

After two misses the owner linked seven Instagram posts (2026-10-08). Six
opened. They are **dense telemetry screens**, and this is the reference for
Vega, above everything else in this file:

- Black background, one mono face for nearly everything, tiny uppercase
  micro-labels with wide tracking, hairline separators, no rounded cards.
- A header strip ("▌DATA ANALYSIS BOT / NAME · T 00:02.57 · F 0154/1800").
- A row of big bold mono numerals with a small coloured unit ("148K", "58%"),
  each with a segmented bar and a dim caption underneath.
- One huge central picture made of thousands of fine lines or particles: a
  radial chord diagram inside a ring of histogram bars; a fountain of hairlines
  rising from a stem into ring-outlined bubbles; a mesh of thin cyan and red
  curves; dense coloured particle clusters with rays to boxed callouts and a
  large "Second Brain" headline bottom-left. Corner brackets frame it; a legend
  list sits beside it.
- Strips of small instruments below: horizontal bars, sparklines, segmented
  blocks, a heat matrix, an arc gauge, a state log, a tick-mark timeline, and a
  footer line of sources.
- Many colours, but as data encoding (a rainbow strip of domains, a colour per
  cluster) on near-black. Fine linework and density carry it, not glow.

Lesson for next time: when an owner says "you did not see what I meant", ask
for two or three exact examples before designing anything.

The rules in section 2 (surface ladder, one accent, hairlines, 4/8 px spacing)
still hold as hygiene. They are not a look by themselves.

## 1. What each site is good for

| Site | What it is | Use it for | Limits |
|---|---|---|---|
| styles.refero.design | 2,000+ extracted design systems (colours, type, spacing, do/don't) | Hard tokens. Open 2-3 systems close to the target and adapt | Values are Refero's extraction, not official specs |
| mobbin.com | 621k screens of shipped apps | Patterns and states (empty, loading, error) for a component | Login; Pro about $10/month; no sci-fi material |
| godly.design | Curated gallery of sites, heroes, apps | Art direction; open the live sites and inspect CSS | No tokens; images only |
| play.mint.gg | 50+ Three.js scenes (MIT repo) | How a central 3D object is framed by thin control panels | Framework WebGL; borrow composition, not code |
| instagram.com/donimas | Login-walled, not viewed. Title says "AI & Big Tech" | Unknown | Needs screenshots from the owner |
| ui.aceternity.com | ~110 React/Tailwind/Motion components | Effect ideas: spotlight, glowing border, beams, aurora | Custom licence (no redistribution); needs porting |
| magicui.design | ~78 components, MIT, many pure CSS | Best source to port from: border beam, animated beam, tickers | React wrappers around portable CSS |
| uiverse.io | 3,000+ plain HTML/CSS elements, MIT | Paste-ready buttons, loaders, patterns, CRT effects | Quality varies; site blocks fetchers (use the `galaxy` repo) |
| animejs.com | Anime.js 4.5, MIT, ~25 KB, single UMD file | Timelines, staggers, SVG drawing, text split/scramble | Default 1000 ms is too slow for UI |
| motion.dev | Animation APIs for vanilla JS and frameworks | Continuous retargeting, springs, transform/opacity | Mini and hybrid APIs have different scope; no dependency needed for simple native motion |
| lenis.dev | Smooth scroll, ~5 KB | Long scroll-storytelling pages | Wrong tool for a fixed dashboard |
| jitter.video | Browser motion-design tool; exports MP4/GIF/Lottie | Prototyping how a sequence should feel before coding it | Not a runtime |
| threlte.xyz | Three.js for Svelte | Svelte projects | Needs Svelte and a build |
| shadergradient.co | Animated shader gradients (React/Vue/Figma) | Reference; the effect is ~60 lines of GLSL (noise-displaced lit plane) | No vanilla build |
| unicorn.studio | Layered WebGL effects and runtime SDK | Named motion states, local interaction response | Runtime variables support data wiring; adopting the SDK is a separate dependency choice |
| vectary.com | Browser 3D design, iframe embeds | Product-style 3D | Remote embed only |
| higgsfield.ai | AI image/video generation | Concept art | A video backdrop reads as fake beside live data |
| relume.ai | AI sitemap/wireframe builder | Marketing sites in Figma/Webflow | Wrong domain for app UI |

For a local page under a strict CSP (`script-src 'self'`), none of these tools
produces drop-in output. They are references; the techniques are what transfer.

## 2. Visual rules for a dark interface

The pattern across Raycast, Factory, Vercel, Cursor and teenage engineering on
Refero **[S]**: weight-400 headings, mono uppercase micro-labels, one accent,
hairlines instead of shadows.

1. **Surface ladder, not one black.** 3-4 steps, 3-6% lightness apart. Raycast:
   `#040506` canvas, `#07080a` card, `#111214`, `#1b1c1e`, border `#2f3031`.
   Factory (warm): `#101010`, `#1d1a18`, hairline `#3d3a39`. Never pure `#000`.
2. **Three text tiers.** About `#eeeeee` primary, `#9c9c9d` secondary, `#6a6b6c`
   muted. No pure white for body.
3. **One accent, under ~5% of pixels**, reserved for what is live or active.
   Buttons stay neutral. Never more than one accent per surface.
4. **Semantic colours separate from the accent and desaturated**: green
   `#59d499` / `#a0ca92`, blue `#56c2ff`, orange `#ee6018` for live or warning.
5. **Two fonts**: one sans for content, one mono for labels, data and shells.
   Bundle locally.
6. **Micro-labels**: mono, 11-12 px, uppercase, +0.06 to +0.08em, muted colour.
7. **Headings at 400-500, never 700.** Tracking tightens with size: 0 at
   14-16 px, -0.02em at 36 px, -0.04em at 72 px.
8. **Big numbers**: 32-36 px, weight 400, negative tracking, tabular figures,
   12 px mono label above. Sparkline 40 px tall, 1 px stroke.
9. **Spacing from a 4/8 px scale only** (4, 8, 12, 16, 24, 32, 48, 64). Panel
   padding 24 px, gaps 8-16 px.
10. **Depth through hairlines**: 1 px `rgba(255,255,255,.06-.10)` plus a top
    inset highlight `inset 0 1px 0 rgba(255,255,255,.05)`. No drop shadows.
11. **One radius family** **[P]**: sharp HUD = 2-4 px controls, 8-10 px panels.
12. **Blur and glow are budgeted** **[P]**: blur at most one floating layer
    (24-48 px); glow only the accent object. Glow on every border is the main
    amateur tell.
13. **Contrast** **[P]**: body at least 4.5:1, muted labels at least 3:1. Chart
    strokes 1-1.5 px, gridlines 6-8% white, status dot 6 px.
14. **Design every state** **[P]**: empty, loading, error, focus (1 px accent ring).

Premium versus cheap, for backgrounds: premium is a very low-contrast drifting
noise gradient in 2-3 near-black hues, film grain at 2-4% (also hides banding),
a soft vignette, a sparse starfield with varied sizes. Cheap, when overdone:
heavy scanlines, strong chromatic aberration, glitch, rainbow gradients, uniform
star dots, bloom on everything.

## 3. Motion

Use native CSS and the Web Animations API; a library earns its place only for
seekable timelines, SVG morphing or text splitting (then vendor
`anime.umd.min.js`). Canvas needs its own frame loop regardless.

- **Durations** **[P]**: hover/press 80-150 ms; small state change 150-220 ms;
  panel enter 240-320 ms; large moves 350-500 ms; nothing over ~700 ms except a
  one-off boot.
- **Easing**: enter `cubic-bezier(0.16, 1, 0.3, 1)`; exit
  `cubic-bezier(0.4, 0, 1, 1)`; on-screen moves `cubic-bezier(0.2, 0, 0, 1)`;
  mechanical UI state `cubic-bezier(0.4, 0, 0.2, 1)` at 150-200 ms **[S]**.
  Linear only for loops and progress.
- **Springs**: near critical damping for tools (bounce 0-0.15, ~300-400 ms).
- **Stagger**: 30-70 ms per item, cascade capped at 400-500 ms.
- **Asymmetry**: exits at 60-75% of the enter time, less travel, no stagger.
- **Animate** `transform` and `opacity`; avoid layout properties and
  `box-shadow`.
- **Reduced motion**: keep short fades, drop translation and loops.
- **Canvas smoothing**: `v += (target - v) * (1 - Math.exp(-dt / 0.12))`.
- **Boot sequence**: 1.2-1.8 s, overlapping stages (grid lines, panel frames,
  labels, data), skippable, shorter on repeat visits.

Recipes: panel enter `opacity 0→1, translateY(12px)→0, scale .985→1`, 280 ms;
chat message in `translateY(8px)→0`, 200 ms; value-changed flash 80 ms up,
600 ms decay; number tick 400 ms ease-out with `tabular-nums`; status LED
`opacity .4↔1`, 1600 ms.

## 4. Effects worth porting, and how they work

All work with a static stylesheet plus `el.style.setProperty()`.

1. **Pointer spotlight border** (Magic Card). Two background layers:
   `linear-gradient(bg 0 0) padding-box, radial-gradient(200px circle at
   var(--mx) var(--my), accent, transparent) border-box`; a `pointermove`
   listener sets `--mx/--my`.
2. **Border beam** (Magic UI). Overlay with transparent border, masked to the
   ring (`mask-clip: padding-box, border-box; mask-composite: intersect`); a
   small gradient square travels on `offset-path: rect(0 auto auto 0 round N)`
   by animating `offset-distance` to 100% over 6 s linear.
3. **Rotating conic ring.** `@property --a` (angle) animated to 360deg;
   `background: linear-gradient(bg,bg) padding-box, conic-gradient(from var(--a),
   transparent 70%, accent 85%, transparent) border-box`.
4. **Cursor-directed glow arc** (Aceternity). Conic mask on `::after` whose
   start angle is `atan2` from the element centre to the pointer, eased along
   the shortest path. One hero element only.
5. **Animated beam along an edge.** A gradient with stops at `t-0.1, t, t+0.22`
   (transparent, colour A, colour B, transparent) moving along the path, ease
   `(.16, 1, .3, 1)`, 5 s.
6. **Text scramble.** Reveal one more real character every 50 ms; re-randomise
   the rest every 50 ms; mono font so width holds.
7. **Number ticker.** Frame loop `v += (target - v) * 0.12` into `textContent`.
8. **Terminal lines.** Each line fades in from `translateY(-5px)` over 0.3 s;
   commands type at 60 ms per character; caret blinks with `steps(1)`.
9. **Dot or grid backdrop with radial mask.**
   `radial-gradient(#ffffff14 1px, transparent 1px) 0 0/22px 22px` plus
   `mask-image: radial-gradient(ellipse at center, #000 30%, transparent 75%)`.
10. **Scanlines.** `repeating-linear-gradient(0deg, rgba(255,255,255,.04) 0 1px,
    transparent 1px 4px)` at very low strength; no flicker.
11. **Meteors, particles, flickering grid** for ambience; cap counts, pause when
    hidden.
12. **3D tilt card.** Parent `perspective: 1000px`; child
    `rotateY((x-cx)/25) rotateX(-(y-cy)/25)`.

Skip aurora (large blurred blend layers are the costliest effect and compete
with a centrepiece).

## 5. A node globe that looks spectacular

Historical direction, rejected for Vega. The current atlas keeps data anchors
stable and moves through its fine linework; do not restore this globe, orbit,
or bloom as a response to the owner's request for motion.

Three routes: better canvas 2D, raw WebGL2 (~400-600 lines, no dependencies),
or vendored Three.js with bloom (~700 KB; its import map is an inline script, so
under `script-src 'self'` it needs a hash or rewritten imports). For a small
fixed scene under a no-library rule, raw WebGL2 with a transparent 2D canvas on
top for labels is the pick. Shaders need no `unsafe-eval`.

In order of impact:

1. Additive soft point sprites: hot white core, coloured halo
   (`a = exp(-d*d*24.0)` plus a small hard core), depth write off.
2. Depth fade on brightness, size and hue for the far side.
3. Pulses in the shader on outward-curved arcs: per-vertex `t`, per-link
   `phase`, `pulse = smoothstep(0.08, 0.0, abs(t - fract(uTime*speed + phase)))`,
   sharp front and long tail. Arcs that bow outward beat chords through the core.
4. Real bloom: scene to a half-float target, separable gaussian at 1/2, 1/4,
   1/8 resolution, added back, then tone-mapped (ACES). Start around strength
   0.8-1.2, radius 0.4-0.7, threshold 0-0.2.
5. Fresnel rim on a translucent sphere (`pow(1.0 - dot(N, V), 3.0)`) and an
   atmosphere shell.
6. Fat, dim, edge-faded links so the pulses carry the brightness. Native GL
   lines are 1 px on Windows; expand to quads.
7. Background: drifting noise, vignette, fine grain.
8. Camera: slow orbit (0.03-0.08 rad/s), exponential damping, eased focus on a
   node over 0.6-1.0 s, idle drift.
9. A node flares and rings when a signal arrives.
10. Sparse parallax starfield.

Canvas 2D quick wins that carry over: `globalCompositeOperation = 'lighter'`,
pre-rendered glow sprites drawn with `drawImage` instead of `shadowBlur`, trails
by partial clears. Cap device pixel ratio at 1.5-2 and pause when hidden.
