# Vega: motion and interaction study

Status update, 2026-10-08: the owner rejected the atlas described below as still
too static and requested a design reset using the Instagram posts. The new
implementation is a deforming filament field with moving claim particles,
local pointer bending and physical topic focus. The historical atlas decisions
below are research history, not the current design. Cached covers supplied six
usable compositions; DdoDyHLKTld's cached embed reported an unavailable post.
No reference video playback or live Vega visual inspection was available.

Studied 2026-10-08 following the owner's feedback that the radial atlas was a
better direction but too static. Three research agents covered the 18 supplied
sites. This is a record of evidence and decisions, not a claim that reviewing a
gallery produces universal design expertise.

## Scope and evidence

The accepted direction is a dense telemetry instrument: black, mono type,
hairlines, real numbers, a large central drawing. The spinning brain has been
discarded. The website may have native abilities alongside chat. Design
iterations need neither a commit nor the full project checks each time.

No research browser was enabled in this session: the computer-use inventory had
no browsers or apps. We inspected public example code, primary documentation,
template metadata, and publicly supplied static gallery images. We did not
watch live animation. Timings proposed for Vega below are our design choices,
unless explicitly identified as parameters from a published example.

No accounts were created, paywalls bypassed, libraries installed, paid services
used, or external assets added to Vega. Public images and code were research
material; the implementation is original vanilla JavaScript and Canvas2D.

## The two useful ideas for this version

### 1. Move through paths while their anchors stay readable

[Aceternity Background Lines](https://ui.aceternity.com/components/background-lines)
publishes fixed paths with short strokes travelling along them through dash
offset. The source uses independent phases and a linear ten-second cycle. It
provides an effective way to make linework alive without rotating the entire
coordinate system.

[Magic UI Animated Beam](https://magicui.design/docs/components/animated-beam)
uses a related approach: a moving gradient along a fixed quadratic path. Its
source describes a five-second cycle and a strong ease-out. Vega borrows the
fixed-path relationship but uses crisp dashes rather than a soft light gradient.

For Vega, each underlying curve represents a saved claim gathered into a topic
register. Ambient strokes move along those same curves. The caption calls this
visual flow; it does not report an ongoing retrieval or create extra claims.
Stable topic phases survive polling. The ring, quantitative bars, labels, and
click targets remain anchored.

### 2. Give the drawing an explicit record journey

The publicly supplied image and HTML for
[Mint Memory Constellation](https://play.mint.gg/memory-constellation) show a large
memory view, search, filters, an inspector, previous/next controls, and a journey
through the current selection. Motion can be navigation through evidence,
instead of perpetual movement of an ornamental object.

Vega's answer selector already chooses actual logged turns. Selecting an answer
now replays a short traversal through its recorded vault topics. The replay ends
in a stable highlight; a Replay Trace control makes the historical nature clear.
The inspector and index remain available throughout.

The API currently supplies a claim count per topic, not each displayed claim's
identifier. Recorded IDs also include summary entries and potentially missing
numeric positions. Therefore the replay uses one collector route per real used
topic. It does not pretend to identify the exact displayed wire for a referenced
claim. Counts on the replay come from unique recorded claim IDs.

## Motion grammar and implementation decisions

- One shared requestAnimationFrame clock, with cached geometry and static
  raster layers. Idle frames composite the fixed drawing and moving strokes.
  Layout measurement and geometry construction happen on invalidation, not
  on every animation frame.
- A short initial reveal establishes the ring and traces over about 700 ms.
  Controls remain usable. An explicit selection skips the remaining reveal.
- Focus emphasis approaches its current target exponentially, using a 90 ms
  time constant. A new target continues from the current appearance. Topic
  sizes and bar lengths do not bounce or overshoot their data values.
- Topic replays travel over 1100 ms with a 400 ms endpoint decay. Start offsets
  are capped across multiple topics. These are Vega choices, not measured
  reference-site timings.
- DOM changes use transform and opacity over 240 ms. Numeric transitions show
  the genuine new value immediately; they do not invent intermediate counts.
- Pause retains the drawing and current flow phase. Hidden tabs stop the frame
  loop. Resume seeds a fresh timestamp, so background wall time causes no jump.
  Reduced motion disables perpetual flow and snaps focus to its final state.
- A repeated state poll must not restart a replay, an entrance, or a number
  transition. Animation follows a user action or genuinely changed data.

[Motion's performance guidance](https://motion.dev/docs/performance) supports
transform/opacity for DOM motion and avoiding repeated layout work. Its
[spring-value documentation](https://motion.dev/docs/spring-value) explains the
useful interaction principle of retargeting continuously rather than restarting
an effect. Vega uses its own interpolation, with no Motion dependency.

## What the rest of the references contributed

| Reference | Evidence examined | Useful lesson or boundary |
|---|---|---|
| [Refero Styles](https://styles.refero.design/) | Style pages and a public Factory image | Check consistency of hierarchy and tokens. Refero identifies recommendations as interpretation and its examples as reconstructions; do not treat them as a site's exact source system. |
| [Anime.js](https://animejs.com/) | Official homepage example source, SVG documentation, grid staggering example | Sequence a local response from a chosen origin. Keep a wave's endpoints meaningful. A timeline is useful choreography, not a reason to animate every component. |
| [Motion](https://motion.dev/) | Official animate, spring, performance and cursor example docs | Continuous input should retarget smoothly. Complete transform and opacity are appropriate for small interface entrances. |
| [Mobbin](https://mobbin.com/explore/web/flows/filtering-sorting) | Public filtering collection and a Mindtrip image | A map and source list can share one persistent selection. Full flows required login; none was attempted. |
| [Uiverse](https://uiverse.io/Nawsome/breezy-ape-20) | Public loader HTML/CSS and the official Galaxy repository | Phase-offset repetition creates rhythm. Its tilted disc, extrusion and animated shadows do not fit Vega's crisp data surface. |
| [Instagram / donimas](https://www.instagram.com/donimas/) | Fetch attempted | No new usable evidence from the profile. The owner's previously inspected individual telemetry posts remain the stronger visual brief. |
| [Godly / Paradigm](https://godly.design/website/paradigmai/) | Public product image and linked live product text | Scope processing states to the work that is actually pending, then resolve into results and cited evidence. |
| [Vectary PCB configurator](https://www.vectary.com/examples/pcb-configurator/) | Official case study and documented controls | A moving route explains a cause and destination. A hotspot opens evidence. Reflective materials and bloom are unnecessary for that relationship. |
| [Threlte trail texture](https://threlte.xyz/docs/reference/extras/use-trail-texture/) | Complete public wireframe demo source | A local pointer field can brighten nearby fine geometry and decay after movement. Do not deform a quantitative radius or change semantic colour with cursor speed. |
| [Jitter stacked donut](https://jitter.video/template/stacked-donut-chart/) | Template metadata and descriptions; related skyline/radar templates | Establish rings and chart elements in a clear order. Presentation movie durations of 4–10 seconds are not suitable delays for a working interface. |
| [Lenis showcase / OP–XY](https://opxy.mengto.here.now/) | Public synthesizer demo source | A centerpiece can be an actual instrument: keys, encoders, sequencer, real analyser traces. Transfer cause-and-effect interaction, not a synthetic fallback waveform or scroll narrative. |
| [ShaderGradient](https://github.com/ruucm/shadergradient) | Primary source | Separate speed, strength, and camera states. Its broad coloured surfaces would compete with Vega's topic encoding. |
| [Unicorn events](https://www.unicorn.studio/docs/events/) | Events, runtime variables and performance documentation | Distinguish pointer position, velocity, hover and entry. Let interaction intensity recover smoothly. Its SDK is not needed to reproduce those principles. |
| [Higgsfield camera controls](https://higgsfield.ai/camera-controls) | Official control gallery | Focus, dolly and orbit have different jobs. Generated footage cannot inspect or accurately follow Vega's records. |
| [Relume Design View](https://www.relume.ai/resources/docs/how-to-style-all-pages-using-the-design-view) | Public editor image and official documentation | Set structure and shared styling before final spacing and custom motion. Do not turn every animation example into another visual system. |
| [Mint Fieldline](https://play.mint.gg/fieldline-santa-clara) | Public scene image and rendered controls | Preserve overview while entering a focused view; make reset discoverable. Inputs and analysis frame one dominant picture. |

Additional primary examples:

- [Anime advanced grid staggering](https://github.com/juliangarnier/anime/blob/master/examples/advanced-grid-staggering/index.js)
  propagates delays from a chosen grid origin. Vega transfers local sequencing,
  not random new origins or changes to quantitative geometry.
- [Mint Dynasty Index](https://play.mint.gg/dynasty-index) shows a persistent
  selector rail around a dominant object. The actual answer rail serves that
  role in Vega.
- [Threlte render modes](https://threlte.xyz/docs/learn/basics/render-modes/)
  demonstrates making render activity respond to state changes. Vega caches
  the settled plate and stops the loop when motion is disabled.

## Corrections to the previous notes

Motion supports vanilla JavaScript. Its official
[animate documentation](https://motion.dev/docs/animate) distinguishes a small
mini API from the larger hybrid API; React and a full-library bundle are not
intrinsic requirements.

Unicorn can be wired to data. Its current
[embed documentation](https://www.unicorn.studio/docs/embed/) describes runtime
variables, presets and self-hosted scenes. The reason to keep Vega native is
scope, control and the current dependency rule, not an inability of that SDK.

Uiverse was readable during this study. Earlier access failures were session
limitations, not a permanent property of the site.

The old globe-orbit and bloom recipes are historical. They are not applicable
to the accepted corpus instrument.

## What remains unverified

Live reference-site animation and Vega's appearance/frame rate in the owner's
browser were not visually verified in this session. A local-data JavaScript
review confirmed 436 generated claim curves across 30 topics, stable geometry
across frames, pause/hidden/reduced-motion stopping, phase-preserving resume,
replay completion, and polling that does not reset the replay. Full project
checks were skipped at the owner's request. Focused follow-up checks confirmed
that reselecting an answer clears its topic selection and refreshes the inspector,
and filtered-out topics produce no replay strokes. The running server serves the
current scripts, stylesheet and motion controls. The owner will try this one
version in the browser; no screenshot was presented.
